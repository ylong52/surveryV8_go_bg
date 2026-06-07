package extractor

// opentext_validator.go — 开放题迭代验证器
//
//	流程：DeepSeek 生成 → Claude 验证 → 动态补充规则 → 重生成（最多 5 轮）
//	     通过后执行 Humanize 扰动 + 二次验证，选最优结果返回

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"survey.cli/core/database"
)

// =============================================================================
// 常量
// =============================================================================

const (
	maxOpenTextRounds     = 5  // 最大迭代轮次
	openTextAIScorePass   = 30 // 通过线：ai_score <= 30
	openTextHumanScorePass = 60 // 通过线：human_score >= 60
	openTextRelevancePass = 80 // 通过线：relevance_score > 80
)

// =============================================================================
// 数据结构
// =============================================================================

// WordLimit 字数限制结构（从 YAML 或页面文本解析）
type WordLimit struct {
	Min int // 0 = 无下限
	Max int // 0 = 无上限
}

// buildLengthRule 根据 min/max 生成 <<<LENGTH_RULE>>> 替换文本
//
//	Case 1: min>0 && max>0 → "Your answer must be between {min} and {max} words..."
//	Case 2: min>0 && max==0 → "Write at least {min} words. Target around {min+5}–{min+20} words..."
//	Case 3: min==0 && max>0 → "Keep your answer under {max} words. Target around {max*0.7}–{max} words..."
//	Case 4: min==0 && max==0 → "Keep your answer concise — around 50–100 words..."
func buildLengthRule(limit WordLimit) string {
	switch {
	case limit.Min > 0 && limit.Max > 0:
		return fmt.Sprintf(
			"Your answer must be between %d and %d words. Do NOT go below %d or above %d words.",
			limit.Min, limit.Max, limit.Min, limit.Max)
	case limit.Min > 0 && limit.Max == 0:
		return fmt.Sprintf(
			"Write at least %d words. Target around %d–%d words for a natural conversational length.",
			limit.Min, limit.Min+5, limit.Min+20)
	case limit.Min == 0 && limit.Max > 0:
		lower := int(float64(limit.Max) * 0.7)
		return fmt.Sprintf(
			"Keep your answer under %d words. Target around %d–%d words.",
			limit.Max, lower, limit.Max)
	default:
		return "Keep your answer concise — around 50–100 words, natural and conversational."
	}
}

// extractShowAnswer 从 DeepSeek 返回的 JSON 中提取 showAnswer 字段
// 如果不是合法 JSON 或缺少字段，返回原始字符串和空 raw
func extractShowAnswer(rawJSON string) (answer string, raw string) {
	if rawJSON == "" {
		return "", ""
	}
	cleaned := stripMarkdownFence(rawJSON)
	var wrapper struct {
		ShowAnswer interface{} `json:"showAnswer"`
	}
	if err := json.Unmarshal([]byte(cleaned), &wrapper); err == nil && wrapper.ShowAnswer != nil {
		switch v := wrapper.ShowAnswer.(type) {
		case string:
			return strings.TrimSpace(v), cleaned
		case []interface{}:
			// 多选项时取第一个
			if len(v) > 0 {
				if s, ok := v[0].(string); ok {
					return strings.TrimSpace(s), cleaned
				}
			}
		}
	}
	// 兜底：直接尝试解析 showAnswer
	var simple struct {
		ShowAnswer string `json:"showAnswer"`
	}
	if err := json.Unmarshal([]byte(cleaned), &simple); err == nil && simple.ShowAnswer != "" {
		return strings.TrimSpace(simple.ShowAnswer), cleaned
	}
	return "", cleaned
}

// countWords 统计英文词数（空格/换行分隔）
func countWords(text string) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	fields := strings.Fields(text)
	return len(fields)
}

// isLengthInRange 判断词数是否在限制范围内
// min==0 时不检查下限；max==0 时不检查上限
func isLengthInRange(wordCount, min, max int) bool {
	if min > 0 && wordCount < min {
		return false
	}
	if max > 0 && wordCount > max {
		return false
	}
	return true
}

// injectLengthRule 将 <<<LENGTH_RULE>>> 占位符替换为实际字数规则
func injectLengthRule(prompt string, limit WordLimit) string {
	if !strings.Contains(prompt, "<<<LENGTH_RULE>>>") {
		return prompt
	}
	rule := buildLengthRule(limit)
	return strings.Replace(prompt, "<<<LENGTH_RULE>>>", rule, 1)
}

// lengthFixSupplement 生成字数纠正的补充规则
func lengthFixSupplement(wordCount, min, max int) string {
	switch {
	case min > 0 && max > 0:
		return fmt.Sprintf(
			"字数严重不足或超额！当前 %d 词，必须严格控制在 %d~%d words 之间，禁止少于 %d 词或超过 %d 词！",
			wordCount, min, max, min, max)
	case min > 0 && max == 0:
		return fmt.Sprintf(
			"字数不足！当前 %d 词，至少写 %d 词，目标 %d~%d 词！",
			wordCount, min, min+5, min+20)
	case min == 0 && max > 0:
		lower := int(float64(max) * 0.7)
		return fmt.Sprintf(
			"字数超额！当前 %d 词，必须控制在 %d~%d words 之间，目标 %d~%d 词！",
			wordCount, lower, max, lower, max)
	default:
		return fmt.Sprintf(
			"字数异常！当前 %d 词，必须控制在 50~100 words 之间！",
			wordCount)
	}
}

// DynamicSupplement 记录每轮追加到提示词的改进建议
type DynamicSupplement struct {
	Round int
	Note  string
}

// OpenTextPrompts 开放题三段提示词（DB 优先，fallback 兜底）
type OpenTextPrompts struct {
	Initial  string
	Verify   string
	Humanize string
}

// openTextVerifyResult Claude 审核返回的评分结构
type openTextVerifyResult struct {
	RelevanceScore      int    `json:"relevance_score"`
	HumanScore          int    `json:"human_score"`
	AIScore             int    `json:"ai_score"`
	ScoreNote           string `json:"score_note"`
	PromptSuggestionNote string `json:"prompt_suggestion_note"`
}

// =============================================================================
// 提示词加载
// =============================================================================

// loadOpenTextPrompts 从数据库加载 open_text.* 提示词，缺失时使用 fallback
func loadOpenTextPrompts() *OpenTextPrompts {
	p := &OpenTextPrompts{
		Initial:  fallbackInitialPrompt,
		Verify:   fallbackVerifyPrompt,
		Humanize: fallbackHumanizePrompt,
	}

	prompts, err := database.LoadAllPrompts()
	if err != nil {
		fmt.Printf("[OpenText] 加载数据库提示词失败，使用 fallback: %v\n", err)
		return p
	}

	loaded := 0
	if v := prompts["open_text.initial"]; v != "" {
		p.Initial = v
		loaded++
	}
	if v := prompts["open_text.verify"]; v != "" {
		p.Verify = v
		loaded++
	}
	if v := prompts["open_text.humanize"]; v != "" {
		p.Humanize = v
		loaded++
	}

	if loaded > 0 {
		fmt.Printf("[OpenText] 从数据库加载 %d/3 条提示词\n", loaded)
	} else {
		fmt.Println("[OpenText] 数据库中未找到 open_text.* 提示词，使用内置 fallback")
	}
	return p
}

// =============================================================================
// 开放题检测
// =============================================================================

// IsOpenTextQuestion 判断 Interaction 是否为需要人工化验证的开放题
//
// 仅依赖 QuestionType 字段（含 "open" 即视为开放题）。
// 调用方需保证 QuestionType 已明确设置，不在此做类型推断。
func IsOpenTextQuestion(interaction Interaction) bool {
	return strings.Contains(strings.ToLower(interaction.QuestionType), "open")
}

// =============================================================================
// 提示词构建
// =============================================================================

// buildOpenTextDynamicPrompt 拼接初始提示词 + 历轮补充规则
//
// @param initial     初始提示词
// @param supplements 历轮追加的改进建议列表
// @returns           完整系统提示词
func buildOpenTextDynamicPrompt(initial string, supplements []DynamicSupplement) string {
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

// =============================================================================
// Claude 验证
// =============================================================================

// verifyAnswerWithClaude 调用 Claude API 验证答案，返回评分结构
//
// @param cfg      Anthropic 配置
// @param prompts  提示词集合
// @param question 原题目文本
// @param answer   待审核答案
// @returns        openTextVerifyResult，或错误
func verifyAnswerWithClaude(cfg *anthropicCfg, prompts *OpenTextPrompts, question, answer string) (*openTextVerifyResult, error) {
	userMsg := fmt.Sprintf("题目：%s\n\n答案：%s", question, answer)

	raw, err := callClaudeAPIInternal(cfg, prompts.Verify, userMsg)
	if err != nil {
		return nil, fmt.Errorf("verifyAnswerWithClaude: API 调用失败: %w", err)
	}

	raw = stripMarkdownFence(raw)
	fmt.Printf("[OpenText] Claude 原始返回: %s\n", raw[:min(200, len(raw))])

	var result openTextVerifyResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("verifyAnswerWithClaude: 解析JSON失败: %v | 原始: %s", err, raw[:min(200, len(raw))])
	}
	return &result, nil
}

// =============================================================================
// 主验证入口
// =============================================================================

// ValidateOpenTextAnswer 开放题迭代验证主入口
//
//	流程：最多 maxOpenTextRounds 轮
//	  1. 注入字数限制规则 <<<LENGTH_RULE>>> → DeepSeek 生成
//	  2. 本地数词判断：达标才送 Claude；不达标 → 追加字数补充规则 → 重生成
//	  3. Claude 验证 → ai_score
//	  4. ai_score <= 30：Humanize + 二次验证 → 返回最优答案，humans_pass=true
//	  5. ai_score > 40 且 prompt_suggestion_note 非空：追加补充规则，重生成
//	  6. 达轮次上限：返回最优答案，humans_pass=false（方案 C）
//
// @param question  题目文本（Subject）
// @param surveyID  问卷 ID（日志用）
// @param basePath  存储根目录（日志用）
// @param prompts   提示词集合（调用方加载，避免每题重复查 DB）
// @param limit     字数限制（min/max，均为0表示无约束）
// @returns         finalAnswer, humanPass, promptSuggestionNote, error
func ValidateOpenTextAnswer(question, surveyID, basePath string, prompts *OpenTextPrompts, limit WordLimit) (string, bool, string, error) {
	anthropicCfg, err := loadAnthropicConfig()
	if err != nil {
		return "", false, "", fmt.Errorf("ValidateOpenTextAnswer: 加载 Anthropic 配置失败: %w", err)
	}

	var supplements []DynamicSupplement
	bestAnswer := ""
	bestAIScore := 100
	bestNote := ""

	for round := 1; round <= maxOpenTextRounds; round++ {
		// Step 0: 注入字数限制 <<<LENGTH_RULE>>>
		sysPrompt := buildOpenTextDynamicPrompt(injectLengthRule(prompts.Initial, limit), supplements)
		userMsg := "题目：" + question

		fmt.Printf("[OpenText] ┌─ Round %d/%d 生成答案\n", round, maxOpenTextRounds)

		// Step A: DeepSeek 生成（temperature=0.7 提升创意多样性）
		answer, err := CallAIRawText(surveyID, sysPrompt, userMsg, basePath, 0.7)
		if err != nil {
			fmt.Printf("[OpenText] │  DeepSeek 失败: %v，休眠后继续\n", err)
			time.Sleep(1 * time.Second)
			continue
		}
		answer = strings.TrimSpace(answer)
		// Step A1: 从 JSON 中提取 showAnswer（DeepSeek 返回的是 {"subject":..., "showAnswer":...}）
		showAnswer, _ := extractShowAnswer(answer)
		if showAnswer == "" {
			// 无法提取时用原始 JSON 文本（兜底）
			showAnswer = answer
		}
		fmt.Printf("[OpenText] │  生成结果: %s\n", answer[:min(80, len(answer))])

		// Step A2: 本地数词 gate（基于提取后的纯文本）
		wordCount := countWords(showAnswer)
		lengthOK := isLengthInRange(wordCount, limit.Min, limit.Max)
		if !lengthOK {
			fmt.Printf("[OpenText] │  showAnswer: %s\n", showAnswer[:min(60, len(showAnswer))])
			fmt.Printf("[OpenText] │  字数: %d words — 本地 gate 不通过，追加字数纠正规则\n", wordCount)
			lengthFix := lengthFixSupplement(wordCount, limit.Min, limit.Max)
			supplements = append(supplements, DynamicSupplement{
				Round: round,
				Note:  lengthFix,
			})
			time.Sleep(300 * time.Millisecond)
			continue // 不送 Claude，直接下一轮
		}
		fmt.Printf("[OpenText] │  字数: %d words — 本地 gate 通过\n", wordCount)

		// Step B: Claude 验证（用提取后的 showAnswer，而非完整 JSON）
		fmt.Println("[OpenText] │  Claude 验证中...")
		result, err := verifyAnswerWithClaude(anthropicCfg, prompts, question, showAnswer)
		if err != nil {
			fmt.Printf("[OpenText] │  Claude 失败: %v，休眠后继续\n", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// 判断三项是否通过
		relPass := result.RelevanceScore >= openTextRelevancePass
		humPass := result.HumanScore >= openTextHumanScorePass
		aiPass := result.AIScore <= openTextAIScorePass

		// 进度条（20字符宽度）
		barLen := 20
		relBar := strings.Repeat("█", barLen) + strings.Repeat("░", 0)
		humFilled := (result.HumanScore * barLen) / 100
		humBar := strings.Repeat("█", humFilled) + strings.Repeat("░", barLen-humFilled)

		passed := relPass && humPass && aiPass
		if passed {
			fmt.Printf("[OpenText] │  验证结果   : \x1b[32mPASS ✓\x1b[0m\n")
		} else {
			fmt.Printf("[OpenText] │  验证结果   : \x1b[31mFAIL ✗\x1b[0m\n")
		}
		fmt.Printf("[OpenText] │  Relevance  : %3d  [%s]  (需 == 100)\n", result.RelevanceScore, relBar)
		fmt.Printf("[OpenText] │  Human      : %3d  [%s]  (需 >= 60)\n", result.HumanScore, humBar)
		fmt.Printf("[OpenText] │  AI         : %3d                          (需 <= 30)\n", result.AIScore)
		fmt.Printf("[OpenText] │  说明       : %s\n", result.ScoreNote)
		if result.PromptSuggestionNote != "" {
			noteLines := strings.Split(result.PromptSuggestionNote, "\n")
			for i, line := range noteLines {
				prefix := "│  改进建议"
				if i > 0 {
					prefix = "│          "
				}
				fmt.Printf("%s : %s\n", prefix, line)
			}
		}

		// 记录全局最优（ai_score 最低者）
		if result.AIScore < bestAIScore {
			bestAIScore = result.AIScore
			bestAnswer = showAnswer
			bestNote = result.PromptSuggestionNote
		}

		if passed {
			// Step C: Humanize 反统计扰动（temperature=1.1 增加随机性）
			fmt.Println("[OpenText] │  通过验证，执行 Humanize 扰动...")
			humanized, herr := CallAIRawText(surveyID, prompts.Humanize, showAnswer, basePath, 1.1)
			if herr != nil {
				fmt.Printf("[OpenText] │  Humanize 失败，使用原答案: %v\n", herr)
				fmt.Printf("[OpenText] └─ Round %d 完成，humans_pass=true\n", round)
				return showAnswer, true, "", nil
			}
			humanized = strings.TrimSpace(humanized)

			// Step D: 二次验证，防止 Humanize 把答案改差（检查 ai_score 和 human_score）
			fmt.Println("[OpenText] │  Claude 二次验证 Humanize 结果...")
			result2, err2 := verifyAnswerWithClaude(anthropicCfg, prompts, question, humanized)
			if err2 != nil {
				fmt.Printf("[OpenText] │  二次验证失败，回滚至原答案: %v\n", err2)
				fmt.Printf("[OpenText] └─ Round %d 完成，humans_pass=true\n", round)
				return answer, true, "", nil
			}

			passed2 := result2.RelevanceScore >= openTextRelevancePass &&
				result2.HumanScore >= openTextHumanScorePass &&
				result2.AIScore <= openTextAIScorePass
			fmt.Printf("[OpenText] │  Humanize 对比:\n")
			fmt.Printf("[OpenText] │    Relevance: %d → %d\n", result.RelevanceScore, result2.RelevanceScore)
			fmt.Printf("[OpenText] │    Human   : %d → %d\n", result.HumanScore, result2.HumanScore)
			fmt.Printf("[OpenText] │    AI      : %d → %d\n", result.AIScore, result2.AIScore)

			if passed2 {
				fmt.Printf("[OpenText] └─ Humanize 有效，采用扰动后答案\n")
				return humanized, true, "", nil
			}
			fmt.Printf("[OpenText] └─ Humanize 评分下降，回滚至扰动前答案\n")
			return showAnswer, true, "", nil
		}

		// 达轮次上限：返回最优结果，humans_pass=false
		if round == maxOpenTextRounds {
			fmt.Printf("[OpenText] └─ 达到最大轮次 %d，返回最优结果 ai_score=%d，humans_pass=false\n",
				maxOpenTextRounds, bestAIScore)
			if bestNote != "" {
				fmt.Printf("[OpenText]    最优改进建议: %s\n", bestNote)
			}
			return bestAnswer, false, bestNote, nil
		}

		// Step E: 追加动态补充规则
		var missing []string
		if !relPass {
			missing = append(missing, fmt.Sprintf("Relevance=%d(需==100)", result.RelevanceScore))
		}
		if !humPass {
			missing = append(missing, fmt.Sprintf("Human=%d(需>=60)", result.HumanScore))
		}
		if !aiPass {
			missing = append(missing, fmt.Sprintf("AI=%d(需<=30)", result.AIScore))
		}

		supplementNote := result.PromptSuggestionNote
		if supplementNote != "" {
			supplementNote = fmt.Sprintf("未通过维度: %s\n%s", strings.Join(missing, " "), supplementNote)
		} else {
			supplementNote = fmt.Sprintf("未通过维度: %s", strings.Join(missing, " "))
		}
		supplements = append(supplements, DynamicSupplement{
			Round: round,
			Note:  supplementNote,
		})
		fmt.Printf("[OpenText] └─ 未通过: %s，追加补充规则，进入第 %d 轮\n",
			strings.Join(missing, " "), round+1)

		time.Sleep(300 * time.Millisecond)
	}

	// 理论上不会执行到此处（round==maxOpenTextRounds 已在循环内处理）
	return bestAnswer, false, bestNote, nil
}
