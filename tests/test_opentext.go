//go:build opentext

package main

// test_opentext.go — 开放题动态提示词迭代验证测试（CLI 交互）
//
// 处理流程：
//   Round 1      : DeepSeek + open_text.initial（无补充）→ 生成初始答案
//   Verify       : Claude + open_text.verify → { human_score, ai_score, note }
//   Dynamic Loop : ai_score > 30 → 若 note 非空则追加到补充列表 → 重生成 → 再验证
//   Pass         : ai_score <= aiScorePass → 执行 Humanize → 输出最终答案
//
// 运行方式（在项目根目录执行）：
//   go run -tags opentext tests/test_opentext.go
//
// 动态补充策略：
//   Round N 提示词 = open_text.initial + 第1轮note + ... + 第N-1轮note
//   ai_score > 40 时 Claude 才生成改进建议（note），30 < ai_score <= 40 则直接重试

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

// ── 配置结构 ──────────────────────────────────────────────────────────────────

type testEnvConfig struct {
	Proxy     proxyCfg     `yaml:"proxy"`
	DeepSeek  dsAPICfg     `yaml:"deepseek"`
	Anthropic anthropicCfg `yaml:"anthropic"`
	// Gemini 保留备用，当前验证器已切换至 Anthropic
	Gemini oaiAPICfg `yaml:"gemini"`
}

type proxyCfg struct {
	HTTPSProxy string `yaml:"https_proxy"`
}

type dsAPICfg struct {
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	Timeout     int     `yaml:"timeout"`
}

type oaiAPICfg struct {
	APIKey  string `yaml:"api_key"`
	BaseURL string `yaml:"base_url"`
	Model   string `yaml:"model"`
	Timeout int    `yaml:"timeout"`
}

type anthropicCfg struct {
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	MaxTokens   int     `yaml:"max_tokens"`
	Temperature float64 `yaml:"temperature"`
	Timeout     int     `yaml:"timeout"`
}

// ── 常量 ──────────────────────────────────────────────────────────────────────

const (
	maxRounds        = 10 // 最大迭代轮次
	aiScorePass      = 30 // 通过线：ai_score <= 30
	dynamicThreshold = 40 // 触发补充：ai_score > 40 时 Claude 生成改进建议
)

// ── 内置 fallback 提示词（数据库未配置时使用）────────────────────────────────

const fallbackInitial = `Rules:
- Use fragments. Not every sentence needs a subject.
#用短句，不用每句话都有主语。
- Skip the reason. Don't explain why things happened.
# 不要解释为什么事情发生了。
- No clean endings. Don't wrap up each paragraph neatly.
# 不要写总结句。
- Occasionally throw in "or whatever", "I don't know", "kind of" — but don't overdo it.
# 偶尔使用"或者"、"我不知道"、"有点"等词语，但不要过度使用。
- No precise details like exact times or numbers unless they're vague on purpose.
# 不要写精确的时间或数字，除非它们是模糊的。
- Jump between ideas without smooth transitions.
# 不要写过渡句。
- Avoid summary sentences at the end of paragraphs.
# 不要写总结句。
- Don't connect every sentence logically — real people don't.
# 不要写逻辑句。
- Drop filler words. Cut anything that sounds like a setup.
# 不要写填充词。
- Never use: seamless, vibrant, dynamic, ensure, utilize, leverage, moreover, furthermore, however.
# 不要使用：流畅、 vibrant、动态、确保、利用、 leverage、 moreover、 furthermore、 however。
- Avoid complete subject-verb-object sentences back to back.
# 不要写完整的主题-动词-宾语句子。
- Mix sentence lengths unevenly — two short, one longer, one fragment, repeat roughly.
# 不要写不均匀的句子长度。
- Don't over-explain emotions. Say what happened, not how it made you feel.
# 不要写过度解释情感的句子。
- Avoid words that sound like AI mimicking casual speech: "totally", "literally", "honestly" used as openers.
# 不要写模仿人类 casual 说话的词语："完全"、" literally"、" honestly" 等词语。
- Leave some things slightly unresolved or unexplained.
# 留下一些稍微未解决或未解释的事情。
- The goal is to be more than 50 but less than 70 words.
# 目标大于 50小于70 个单词。
- No bullet points, no lists, no headers in output.
# 不要写 bullet points, lists, headers 等标题。

补充：
1，若要减少AI痕迹，提示词中应加入'强制破坏句式结构'的指令，例如要求：'禁止使用因果连词（因为、所以、但是），强制将长句拆解为语义跳跃的短语或名词堆砌'。AI通常倾向于保持叙事的完整逻辑链（即使伪装成口语），可以通过要求'叙述中必须包含无关的感官干扰（如窗外的噪音、手边的凉水）'，打破AI追求的'叙事简洁与中心突出'的预设倾向，从而模拟人类思维的随机游走。

要求:
为上面的文案用ai和human打分. AI痕迹满分50,human书写满分50.
{
  "human_score": 50,
  "ai_score": 50,
  "note": {if (ai_score > 40) 用中文写去掉AI痕迹提示词改进的补充意见 else ''}
}`

// fallbackHumanize 对通过验证的答案做最后一道反统计检测扰动
const fallbackHumanize = `You will receive a survey answer. Apply these specific edits to make it harder for AI detectors to flag it. Do NOT rewrite or restructure — make surgical word-level changes only.

WHAT TO CHANGE (pick 3–4 of these, whichever fit naturally):

Word substitutions — swap predictable words for less expected ones that still make sense:
- "memorable" → "one I keep thinking about" or "that one time"
- "experience" → "thing that happened" or "situation"
- "customer service" → "the person at the desk" or "the lady/guy there"
- "however" / "but" → "though" or just start a new sentence
- "decided" → "ended up" or "just went ahead and"
- "received" → "got" or "they handed me"
- "noticed" → "could tell" or "saw"
- any -ly adverb → rephrase without it ("quickly" → "right away", "surprisingly" → "which was weird")

Rhythm breaks — do ONE of these:
- Take the longest sentence and split it at a natural pause with a period, even mid-thought
- Take two short sentences and run them together with just a comma (comma splice is fine)

Filler insertion — add ONE of these somewhere it feels natural (not at the start):
- "or whatever", "I guess", "which was odd", "for some reason", "not gonna lie"

End abruptly — if the last sentence wraps things up too neatly, cut the final 4–6 words or replace with "idk" / "so yeah" / nothing

DO NOT:
- Reorder sentences or change sentence structure significantly
- Fix or introduce grammar errors that make the text confusing
- Remove existing casual words like "honestly", "tbh", "like" — keep them
- Add new facts or change what happened
- Exceed 85 words

Output the modified answer only. No explanation.`

const fallbackVerify = `You are a survey answer quality auditor. Score whether this answer was written by a real human or generated by AI.

## SCORING DIMENSIONS

### Dimension 1: AI Artifact Score (0–50, start at 0, add deductions)
- Formal opener (Certainly / Of course / Absolutely)     : +20
- Transition words (Furthermore / Moreover / In addition) : +15
- Closing remark (In conclusion / I hope this helps)     : +15
- Zero contractions (I do not / it is / I am)            : +15
- Uniform sentence lengths (±5 words across all)         : +10
- No personal specifics (brand/place/name/number)        : +10
- Hedging language (It is worth noting / One could)      : +10
- Bullet points or numbered lists                        : +20

### Dimension 2: Human Writing Score (0–50, start at 0, add additions)
- Contractions used naturally (don't, it's, I've)        : +10
- Slang or filler words (honestly, like, tbh, kinda)     : +10
- Specific personal detail (brand/place/name/number)     : +10
- Uneven sentence lengths (mix of short and long)        : +8
- Mild typo or casual punctuation                        : +5
- Incomplete thought or self-correction ("or maybe...")  : +5
- Clear personal stance with no hedging                  : +7

## PASS CONDITION
ai_score <= 30 → passes as human-written

## OUTPUT FORMAT
Return ONLY valid JSON. No markdown code fences. No other text:
{
  "human_score": <integer 0-50>,
  "ai_score": <integer 0-50>,
  "score_note": "Brief English explanation naming the dominant pattern that most affected the score",
  "note": "If ai_score > 40: write specific Chinese improvement suggestions in '禁止X/必须Y' format (2–4 items, numbered) to reduce AI artifacts. Otherwise: empty string."
}`

// ── 数据结构 ──────────────────────────────────────────────────────────────────

// DynamicSupplement 记录每轮追加到提示词的改进建议
type DynamicSupplement struct {
	Round int
	Note  string
}

// verifyResult Claude 审核返回的评分结构
// human_score 和 ai_score 各自独立评分（满分各 50 分）
// 通过条件：ai_score <= aiScorePass (30)
type verifyResult struct {
	HumanScore int    `json:"human_score"`
	AIScore    int    `json:"ai_score"`
	ScoreNote  string `json:"score_note"`
	Note       string `json:"note"` // ai_score > 40 时 Claude 返回中文改进建议
}

// openTextPrompts 保存运行时加载的提示词
type openTextPrompts struct {
	Initial  string
	Verify   string
	Humanize string
}

// ── 提示词加载（DB 优先，fallback 兜底）─────────────────────────────────────

// loadPrompts 尝试从 SQLite 加载，失败时使用内置 fallback
func loadPrompts(dbPath string) *openTextPrompts {
	p := &openTextPrompts{
		Initial:  fallbackInitial,
		Verify:   fallbackVerify,
		Humanize: fallbackHumanize,
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Printf("  [提示词] 无法打开数据库，使用内置 fallback: %v\n", err)
		return p
	}
	defer db.Close()

	keys := map[string]*string{
		"open_text.initial":  &p.Initial,
		"open_text.verify":   &p.Verify,
		"open_text.humanize": &p.Humanize,
	}

	loaded := 0
	for key, target := range keys {
		var content string
		err := db.QueryRow(
			`SELECT content FROM prompts WHERE prompt_key = ? AND is_active = 1 LIMIT 1`,
			key,
		).Scan(&content)
		if err == nil && content != "" {
			*target = content
			loaded++
		}
	}

	if loaded > 0 {
		fmt.Printf("  [提示词] 从数据库加载 %d/3 条（%s）\n", loaded, dbPath)
	} else {
		fmt.Printf("  [提示词] 数据库中未找到 open_text.* 提示词，使用内置 fallback\n")
	}
	return p
}

// ── 主程序 ────────────────────────────────────────────────────────────────────

func main() {
	printBanner()

	cfg, err := loadTestEnvConfig("configs/env.yaml")
	if err != nil {
		fatalf("加载配置失败: %v", err)
	}
	if err := validateTestConfig(cfg); err != nil {
		fatalf("配置校验失败: %v", err)
	}

	prompts := loadPrompts("data/survey202605.db")
	fmt.Println()

	question := askQuestion()
	fmt.Println()

	runOpenTextLoop(question, cfg, prompts)
}

// runOpenTextLoop 执行动态提示词迭代验证循环
//
// @param question 题目文本
// @param cfg      环境配置
// @param prompts  提示词（DB 或 fallback）
func runOpenTextLoop(question string, cfg *testEnvConfig, prompts *openTextPrompts) {
	var prevAnswer string
	var supplements []DynamicSupplement

	for round := 1; round <= maxRounds; round++ {
		sysPrompt := buildDynamicPrompt(prompts.Initial, supplements)
		userMsg := "题目：" + question

		printStepHeader(round, supplements)

		// Step A：DeepSeek 生成
		answer, err := callChatAPI(
			cfg.DeepSeek.APIKey, cfg.DeepSeek.BaseURL, cfg.DeepSeek.Model,
			0.7, cfg.DeepSeek.Timeout, sysPrompt, userMsg,
		)
		if err != nil {
			printLineError("DeepSeek 调用失败: " + err.Error())
			fmt.Println("└──────────────────────────────────────────────────────────")
			fmt.Println()
			time.Sleep(2 * time.Second)
			continue
		}
		answer = strings.TrimSpace(answer)
		prevAnswer = answer
		printGenerated(answer)

		// Step B：Claude 验证
		fmt.Println("│  [Claude 验证中...]  [open_text.verify]")
		result, err := verifyWithClaude(cfg, prompts.Verify, question, answer)
		if err != nil {
			printLineError("Claude 验证失败: " + err.Error())
			fmt.Println("└──────────────────────────────────────────────────────────")
			fmt.Println()
			time.Sleep(2 * time.Second)
			continue
		}

		printVerifyResult(result)

		if result.AIScore <= aiScorePass {
			// Step C：Humanize 反统计扰动
			fmt.Println("│  [DeepSeek 反perplexity扰动中...]  [open_text.humanize]")
			humanized, herr := callChatAPI(
				cfg.DeepSeek.APIKey, cfg.DeepSeek.BaseURL, cfg.DeepSeek.Model,
				1.1, cfg.DeepSeek.Timeout, prompts.Humanize, answer,
			)
			if herr != nil {
				printLineError("Humanize 调用失败，使用原答案: " + herr.Error())
				printFinal(answer, round)
				return
			}

			humanizedAnswer := strings.TrimSpace(humanized)
			fmt.Println("│  [Humanize 结果]")
			for _, line := range strings.Split(humanizedAnswer, "\n") {
				fmt.Printf("│    %s\n", line)
			}
			fmt.Println("│")

			// Step D：二次验证 Humanize 结果，防止扰动把答案改差
			fmt.Println("│  [Claude 二次验证 Humanize 结果...]")
			result2, err2 := verifyWithClaude(cfg, prompts.Verify, question, humanizedAnswer)
			if err2 != nil {
				printLineError("二次验证失败，回滚至 Humanize 前: " + err2.Error())
				printFinal(answer, round)
				return
			}
			printVerifyResult(result2)

			if result2.AIScore <= result.AIScore {
				fmt.Printf("│  → Humanize 有效（AI %d → %d），采用扰动后答案\n", result.AIScore, result2.AIScore)
				printFinal(humanizedAnswer, round)
			} else {
				fmt.Printf("│  → Humanize 有损（AI %d → %d），回滚至扰动前答案\n", result.AIScore, result2.AIScore)
				printFinal(answer, round)
			}
			return
		}

		if round == maxRounds {
			printMaxReached(prevAnswer)
			return
		}

		// Step E：追加动态补充规则（仅 ai_score > dynamicThreshold 且有 note 时）
		if result.AIScore > dynamicThreshold && result.Note != "" {
			supplements = append(supplements, DynamicSupplement{Round: round, Note: result.Note})
			fmt.Printf("  → ai_score=%d > %d，提示词已追加第 %d 条补充规则，进入第 %d 轮\n\n",
				result.AIScore, dynamicThreshold, len(supplements), round+1)
		} else {
			fmt.Printf("  → ai_score=%d 未达 %d，进入第 %d 轮重生成\n\n",
				result.AIScore, aiScorePass, round+1)
		}

		time.Sleep(300 * time.Millisecond)
	}
}

// buildDynamicPrompt 将 initial 提示词与历轮补充规则拼接为完整系统提示词
//
// @param initial     初始提示词
// @param supplements 历轮追加的改进建议列表（按轮次顺序）
// @returns           拼接后的完整提示词
func buildDynamicPrompt(initial string, supplements []DynamicSupplement) string {
	if len(supplements) == 0 {
		return initial
	}
	var sb strings.Builder
	sb.WriteString(initial)
	sb.WriteString("\n\n补充规则（严格遵守，优先级高于以上所有规则）：")
	for _, s := range supplements {
		sb.WriteString(fmt.Sprintf("\n\n[基于第%d轮验证的改进建议]\n%s", s.Round, s.Note))
	}
	return sb.String()
}

// verifyWithClaude 调用 Anthropic Claude API 审核答案，返回评分结构
//
// @param cfg          环境配置（Anthropic）
// @param verifyPrompt 审核系统提示词
// @param question     原题目
// @param answer       待审核答案
func verifyWithClaude(cfg *testEnvConfig, verifyPrompt, question, answer string) (*verifyResult, error) {
	userMsg := fmt.Sprintf("题目：%s\n\n答案：%s", question, answer)

	raw, err := callClaudeAPI(
		cfg.Anthropic.APIKey, cfg.Anthropic.BaseURL, cfg.Anthropic.Model,
		cfg.Anthropic.MaxTokens, cfg.Anthropic.Temperature, cfg.Anthropic.Timeout,
		verifyPrompt, userMsg,
	)
	if err != nil {
		return nil, err
	}

	raw = stripMarkdownFence(raw)
	fmt.Printf("│  [调试] Claude 原始返回: %s\n", raw)
	var result verifyResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("解析验证JSON失败: %v | 原始: %s", err, raw)
	}
	return &result, nil
}

// callClaudeAPI 调用 Anthropic Claude Messages API（非 OpenAI 兼容格式）
//
// @param apiKey      Anthropic API Key
// @param baseURL     API 基地址（https://api.anthropic.com）
// @param model       模型名称（claude-sonnet-4-6 等）
// @param maxTokens   最大输出 token 数
// @param temperature 温度参数
// @param timeoutSec  超时秒数（0 默认 60s）
// @param system      系统提示词
// @param user        用户消息
// @returns           AI 回复文本，或错误
func callClaudeAPI(apiKey, baseURL, model string, maxTokens int,
	temperature float64, timeoutSec int, system, user string) (string, error) {

	type claudeMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type claudeReq struct {
		Model       string      `json:"model"`
		MaxTokens   int         `json:"max_tokens"`
		Temperature float64     `json:"temperature"`
		System      string      `json:"system"`
		Messages    []claudeMsg `json:"messages"`
	}

	if maxTokens == 0 {
		maxTokens = 1024
	}

	body, _ := json.Marshal(claudeReq{
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		System:      system,
		Messages: []claudeMsg{
			{Role: "user", Content: user},
		},
	})

	timeout := time.Duration(timeoutSec) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	transport := &http.Transport{}
	client := &http.Client{Timeout: timeout, Transport: transport}

	req, err := http.NewRequest("POST", baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP 请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	type claudeResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	var result claudeResp
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}
	if len(result.Content) == 0 || result.Content[0].Text == "" {
		return "", fmt.Errorf("API 返回空响应")
	}
	return result.Content[0].Text, nil
}

// callChatAPI 向兼容 OpenAI 格式的 API 发起单次请求（DeepSeek 生成用）
//
// @param apiKey      Bearer Token
// @param baseURL     API 基地址
// @param model       模型名称
// @param temperature 温度参数
// @param timeoutSec  超时秒数（0 默认 60s）
// @param system      系统提示词
// @param user        用户消息
// @returns           AI 回复文本，或错误
func callChatAPI(apiKey, baseURL, model string, temperature float64,
	timeoutSec int, system, user string) (string, error) {

	type chatMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type chatReq struct {
		Model       string    `json:"model"`
		Messages    []chatMsg `json:"messages"`
		Temperature float64   `json:"temperature"`
		Stream      bool      `json:"stream"`
	}

	body, _ := json.Marshal(chatReq{
		Model: model,
		Messages: []chatMsg{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: temperature,
		Stream:      false,
	})

	timeout := time.Duration(timeoutSec) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	transport := &http.Transport{}
	client := &http.Client{Timeout: timeout, Transport: transport}

	req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP 请求失败: %v", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	type chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	var result chatResp
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("API 返回空响应")
	}
	return result.Choices[0].Message.Content, nil
}

// ── 配置加载 ──────────────────────────────────────────────────────────────────

func loadTestEnvConfig(path string) (*testEnvConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg testEnvConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// DeepSeek 默认值
	if cfg.DeepSeek.BaseURL == "" {
		cfg.DeepSeek.BaseURL = "https://api.deepseek.com"
	}
	if cfg.DeepSeek.Model == "" {
		cfg.DeepSeek.Model = "deepseek-chat"
	}
	if cfg.DeepSeek.Timeout == 0 {
		cfg.DeepSeek.Timeout = 120
	}

	// Anthropic 默认值
	if cfg.Anthropic.BaseURL == "" {
		cfg.Anthropic.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Anthropic.Model == "" {
		cfg.Anthropic.Model = "claude-sonnet-4-6"
	}
	if cfg.Anthropic.MaxTokens == 0 {
		cfg.Anthropic.MaxTokens = 1024
	}
	if cfg.Anthropic.Timeout == 0 {
		cfg.Anthropic.Timeout = 60
	}
	if cfg.Anthropic.APIKey == "" {
		cfg.Anthropic.APIKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	return &cfg, nil
}

func validateTestConfig(cfg *testEnvConfig) error {
	if cfg.DeepSeek.APIKey == "" {
		return fmt.Errorf("deepseek.api_key 未配置（configs/env.yaml）")
	}
	if cfg.Anthropic.APIKey == "" {
		return fmt.Errorf("anthropic.api_key 未配置\n" +
			"  方式1：configs/env.yaml 添加 anthropic.api_key\n" +
			"  方式2：环境变量 ANTHROPIC_API_KEY=sk-ant-xxx")
	}
	return nil
}

// ── 工具函数 ──────────────────────────────────────────────────────────────────

func stripMarkdownFence(s string) string {
	s = strings.TrimSpace(s)
	for _, pfx := range []string{"```json", "```"} {
		if strings.HasPrefix(s, pfx) {
			s = strings.TrimPrefix(s, pfx)
			break
		}
	}
	return strings.TrimSpace(strings.TrimSuffix(s, "```"))
}

func askQuestion() string {
	samples := []string{
		"Please describe the most memorable consumer experience you've had in the past year.",
		"What do you think is the biggest challenge in your current job?",
		"In your own words, how has technology changed your daily life?",
	}

	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  选择测试题目 (英文题更贴近真实问卷场景):")
	for i, s := range samples {
		fmt.Printf("  %d. %s\n", i+1, s)
	}
	fmt.Println("  0. 自定义输入")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Print("  输入编号 [默认 1]: ")

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	switch line {
	case "0":
		fmt.Print("  请输入题目: ")
		q, _ := reader.ReadString('\n')
		return strings.TrimSpace(q)
	case "2":
		return samples[1]
	case "3":
		return samples[2]
	default:
		return samples[0]
	}
}

// ── CLI 输出 ──────────────────────────────────────────────────────────────────

func printBanner() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║       开放题动态提示词迭代验证  test_opentext.go             ║")
	fmt.Println("║  DeepSeek生成 → Claude验证 → 动态追加补充规则 → 重生成      ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
}

// printStepHeader 打印每轮开始时的标题，展示当前所有补充规则
//
// @param round       当前轮次
// @param supplements 已累积的补充规则列表
func printStepHeader(round int, supplements []DynamicSupplement) {
	if len(supplements) == 0 {
		fmt.Printf("┌─ Round #%d [DeepSeek 生成]  [open_text.initial]\n", round)
	} else {
		fmt.Printf("┌─ Round #%d [DeepSeek 生成 + 动态补充]  [open_text.initial + %d条补充规则]\n",
			round, len(supplements))
		for _, s := range supplements {
			fmt.Printf("│  补充规则（第%d轮追加）：\n", s.Round)
			for _, line := range strings.Split(s.Note, "\n") {
				if strings.TrimSpace(line) != "" {
					fmt.Printf("│    %s\n", line)
				}
			}
		}
	}
}

func printGenerated(answer string) {
	fmt.Println("│  [DeepSeek 生成结果]")
	for _, line := range strings.Split(answer, "\n") {
		fmt.Printf("│    %s\n", line)
	}
	fmt.Println("│")
}

func printVerifyResult(r *verifyResult) {
	status := "PASS ✓"
	if r.AIScore > aiScorePass {
		status = "FAIL ✗"
	}
	// AI分进度条（满分50，20格）
	filled := r.AIScore * 20 / 50
	if filled > 20 {
		filled = 20
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)
	fmt.Printf("│  验证结果 : %s\n", status)
	fmt.Printf("│  Human    : %3d/50\n", r.HumanScore)
	fmt.Printf("│  AI       : %3d  [%s]  (通过线: ai_score <= %d)\n", r.AIScore, bar, aiScorePass)
	fmt.Printf("│  说明     : %s\n", r.ScoreNote)
	if r.Note != "" {
		fmt.Printf("│  改进建议 : %s\n", r.Note)
	}
	fmt.Println("└──────────────────────────────────────────────────────────────")
	fmt.Println()
}

func printFinal(answer string, round int) {
	fmt.Println("══════════════════════════════════════════════════════════════")
	fmt.Printf("  ✅ 验证通过（共 %d 轮迭代）\n\n", round)
	fmt.Println("  最终答案：")
	fmt.Printf("  %s\n", answer)
	fmt.Println("══════════════════════════════════════════════════════════════")
}

func printMaxReached(answer string) {
	fmt.Println("══════════════════════════════════════════════════════════════")
	fmt.Println("  ⚠️  已达最大迭代轮次（10次），使用最后一次生成的答案：")
	fmt.Printf("  %s\n", answer)
	fmt.Println("══════════════════════════════════════════════════════════════")
}

func printLineError(msg string) {
	fmt.Printf("│  ❌ %s\n", msg)
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "❌ "+format+"\n", args...)
	os.Exit(1)
}
