package extractor

// yaml_answer.go — YAML 结构 + Persona 生成推荐答案
//   根据 YAML 结构 + HTML + Persona 调用 DeepSeek API 生成推荐答案
//   输出格式复用 prompts.yaml 的 interactions 结构（subject + showAnswer）

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// =============================================================================
// 1. 数据结构定义
// =============================================================================

// YamlAnswerConfig 对应 configs/yaml_structure_prompt.yaml 的答案推荐字段
type YamlAnswerConfig struct {
	AnswerSystemPrompt    string `yaml:"ANSWER_SYSTEM_PROMPT"`
	AnswerUserPromptPrefix string `yaml:"ANSWER_USER_PROMPT_PREFIX"`
	PersonaContextPrefix  string `yaml:"PERSONA_CONTEXT_PREFIX"`
}

// Interaction 单个题目的推荐答案
type Interaction struct {
	Subject      string      `yaml:"subject" json:"subject"`
	ShowAnswer   interface{} `yaml:"showAnswer" json:"showAnswer"` // string 或 []string
	QuestionType string      `yaml:"questionType" json:"questionType"` // 题目类型
}

// YamlAnswerResult 推荐答案结果
type YamlAnswerResult struct {
	Interactions []Interaction // 推荐答案列表
	PageSummary  string        // 页面总结（AI 回复）
	Source       string        // "cache" / "ai"
	Meta         PageMetadata
	CachePath    string
}

// InteractionsResponse AI 返回的 JSON 结构
type InteractionsResponse struct {
	Interactions []Interaction `json:"interactions"`
	PageSummary  string        `json:"page_summary"`
}

// =============================================================================
// 2. 配置加载
// =============================================================================

// LoadYamlAnswerConfig 从 configs/yaml_structure_prompt.yaml 读取答案推荐配置
func LoadYamlAnswerConfig() (*YamlAnswerConfig, error) {
	cfg, err := LoadYamlStructureConfig()
	if err != nil {
		return nil, fmt.Errorf("LoadYamlAnswerConfig: %w", err)
	}

	if cfg.AnswerSystemPrompt == "" {
		return nil, fmt.Errorf("LoadYamlAnswerConfig: ANSWER_SYSTEM_PROMPT is empty")
	}
	if cfg.AnswerUserPromptPrefix == "" {
		return nil, fmt.Errorf("LoadYamlAnswerConfig: ANSWER_USER_PROMPT_PREFIX is empty")
	}

	return &YamlAnswerConfig{
		AnswerSystemPrompt:    cfg.AnswerSystemPrompt,
		AnswerUserPromptPrefix: cfg.AnswerUserPromptPrefix,
		PersonaContextPrefix:  cfg.PersonaContextPrefix,
	}, nil
}

// =============================================================================
// 3. Prompt 构建
// =============================================================================

// BuildYamlAnswerPrompt 构建答案推荐 Prompt
// @param persona        人设信息（由 LoadPersona() 生成）
// @param yamlStructure  YAML 结构字符串
// @returns              systemPrompt, userPrompt, error
func BuildYamlAnswerPrompt(persona, yamlStructure string) (string, string, error) {
	cfg, err := LoadYamlAnswerConfig()
	if err != nil {
		return "", "", fmt.Errorf("BuildYamlAnswerPrompt: %w", err)
	}

	// System Prompt
	systemPrompt := cfg.AnswerSystemPrompt

	// User Prompt
	var userPrompt strings.Builder

	// 人设上下文
	if persona != "" {
		userPrompt.WriteString(cfg.PersonaContextPrefix)
		userPrompt.WriteString(persona)
		userPrompt.WriteString("\n\n")
	}

	// 用户提示前缀
	userPrompt.WriteString(cfg.AnswerUserPromptPrefix)
	userPrompt.WriteString("\n\n")

	// YAML 结构
	userPrompt.WriteString("```yaml\n")
	userPrompt.WriteString(yamlStructure)
	userPrompt.WriteString("\n```\n\n")

	return systemPrompt, userPrompt.String(), nil
}

// =============================================================================
// 4. 主流程
// =============================================================================

// ExtractYamlAnswers 根据 YAML 结构 + Persona 生成推荐答案
//   内部加载人设，调用 AI 生成推荐答案
// @param yamlStructure  YAML 结构字符串
// @param surveyID       问卷 ID
// @param pageID         页面 ID
// @param basePath       存储根目录
// @returns              YamlAnswerResult, error
func ExtractYamlAnswers(yamlStructure, surveyID, pageID, basePath string) (*YamlAnswerResult, error) {
	// 步骤 1: 加载人设
	persona, err := LoadPersona()
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlAnswers: LoadPersona failed: %w", err)
	}

	// 步骤 2: 构建 Prompt
	systemPrompt, userPrompt, err := BuildYamlAnswerPrompt(persona, yamlStructure)
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlAnswers: BuildYamlAnswerPrompt failed: %w", err)
	}

	// 步骤 3: 调用 AI
	aiResponse, err := CallAI(surveyID, systemPrompt, userPrompt, basePath)
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlAnswers: CallAI failed: %w", err)
	}

	// 步骤 4: 解析 AI 响应
	interactions, pageSummary, err := parseInteractionsResponse(aiResponse)
	if err != nil {
		saveAnswerFailureLog(basePath, surveyID, pageID, systemPrompt, userPrompt, aiResponse, err)
		return nil, fmt.Errorf("ExtractYamlAnswers: parse response failed: %w", err)
	}

	return &YamlAnswerResult{
		Interactions: interactions,
		PageSummary:  pageSummary,
		Source:      "ai",
	}, nil
}

// parseInteractionsResponse 解析 AI 返回的 JSON 响应
func parseInteractionsResponse(content string) ([]Interaction, string, error) {
	// 去除可能的 markdown 代码块
	cleaned := stripMarkdownFence(content)

	var resp InteractionsResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, "", fmt.Errorf("json.Unmarshal failed: %w", err)
	}

	if len(resp.Interactions) == 0 {
		return nil, "", fmt.Errorf("no interactions found in response")
	}

	return resp.Interactions, resp.PageSummary, nil
}

// stripMarkdownFence 去除 markdown 代码块包裹（复用 ai_caller.go 中的实现）

// saveAnswerToCache 保存答案到缓存文件
func saveAnswerToCache(cachePath string, interactions []Interaction) error {
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("saveAnswerToCache: mkdir %s: %w", cacheDir, err)
	}

	data, err := json.MarshalIndent(interactions, "", "  ")
	if err != nil {
		return fmt.Errorf("saveAnswerToCache: marshal: %w", err)
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("saveAnswerToCache: write: %w", err)
	}

	return nil
}

// saveAnswerFailureLog 保存失败日志
func saveAnswerFailureLog(basePath, surveyID, pageID, systemPrompt, userPrompt, response string, err error) {
	dir := filepath.Join(basePath, "logs", "answer_failures")
	_ = os.MkdirAll(dir, 0755)

	ts := time.Now().Format("20060102_150405")
	path := filepath.Join(dir, fmt.Sprintf("%s_%s_%s.log", surveyID, pageID, ts))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("时间: %s\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", surveyID))
	sb.WriteString(fmt.Sprintf("page_id: %s\n", pageID))
	sb.WriteString(fmt.Sprintf("错误: %v\n", err))
	sb.WriteString("\n=== System Prompt ===\n")
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n=== User Prompt ===\n")
	sb.WriteString(userPrompt)
	sb.WriteString("\n\n=== AI Response ===\n")
	sb.WriteString(response)

	_ = os.WriteFile(path, []byte(sb.String()), 0644)
}

// =============================================================================
// 5. 完整提取流程（整合 YAML 结构 + 答案推荐）
// =============================================================================

// FullYamlExtractResult 完整提取结果
type FullYamlExtractResult struct {
	YamlContent  string
	Answers      []Interaction
	Source       string // "cache" / "ai"
	Meta         PageMetadata
	CachePath    string // YAML 缓存路径
	AnswerCache  string // 答案缓存路径
}

// FullYamlExtractProcess 完整提取流程：YAML 结构 + 答案推荐
// @param persona   人设信息（可为空）
// @param surveyID  问卷 ID
// @param html      清洗后的 HTML
// @param basePath  存储根目录
// @returns         FullYamlExtractResult, error
func FullYamlExtractProcess(persona, surveyID, html, basePath string) (*FullYamlExtractResult, error) {
	// 步骤 1: 提取 YAML 结构
	yamlResult, err := ExtractYamlStructureV2(surveyID, html, basePath)
	if err != nil {
		return nil, fmt.Errorf("FullYamlExtractProcess: ExtractYamlStructureV2 failed: %w", err)
	}

	// 步骤 2: 生成推荐答案
	answerResult, err := ExtractYamlAnswers(
		yamlResult.YamlContent,
		surveyID,
		yamlResult.Meta.PageID,
		basePath,
	)
	if err != nil {
		return nil, fmt.Errorf("FullYamlExtractProcess: ExtractYamlAnswers failed: %w", err)
	}

	return &FullYamlExtractResult{
		YamlContent:  yamlResult.YamlContent,
		Answers:      answerResult.Interactions,
		Source:       yamlResult.Source,
		Meta:         yamlResult.Meta,
		CachePath:    yamlResult.CachePath,
		AnswerCache:  answerResult.CachePath,
	}, nil
}

// =============================================================================
// 6. 辅助方法
// =============================================================================

// PrintAnswerSummary 打印答案摘要
func PrintAnswerSummary(result *YamlAnswerResult) {
	fmt.Println()
	fmt.Println("=== 推荐答案生成完成 ===")
	fmt.Printf("题目数量: %d\n", len(result.Interactions))
	fmt.Printf("来源: %s\n", result.Source)
	fmt.Printf("PageID: %s\n", result.Meta.PageID)
	fmt.Println()

	for i, item := range result.Interactions {
		fmt.Printf("[%d] %s\n", i+1, item.Subject)
		if arr, ok := item.ShowAnswer.([]interface{}); ok {
			fmt.Printf("    → %v\n", arr)
		} else {
			fmt.Printf("    → %v\n", item.ShowAnswer)
		}
	}
	fmt.Println()
}
