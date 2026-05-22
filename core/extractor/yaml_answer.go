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

	"github.com/patrickmn/go-cache"
	"gopkg.in/yaml.v3"
)

// =============================================================================
// 1. 数据结构定义
// =============================================================================

// YamlAnswerConfig 对应 configs/yaml_structure_prompt.yaml 的答案推荐字段
type YamlAnswerConfig struct {
	AnswerSystemPrompt     string `yaml:"ANSWER_SYSTEM_PROMPT"`
	AnswerUserPromptPrefix string `yaml:"ANSWER_USER_PROMPT_PREFIX"`
	PersonaContextPrefix   string `yaml:"PERSONA_CONTEXT_PREFIX"`
}

// Interaction 单个题目的推荐答案
type Interaction struct {
	Subject      string      `yaml:"subject" json:"subject"`
	ShowAnswer   interface{} `yaml:"showAnswer" json:"showAnswer"`     // string 或 []string
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

// RealtimeAnswerOnlyResult 合并后的推荐答案结果（单次AI调用）
// 用于缓存命中时，一次性生成推荐答案
type RealtimeAnswerOnlyResult struct {
	Interactions  []Interaction   // 推荐答案列表
	PageSummary   string          // 页面摘要
	PageID        string          // 页面ID
	PageSettings  PageSettings    // 页面设置（从cachedYaml解析）
	YamlStructure string          // YAML结构（复用cachedYaml）
	FinalJson     FinalAnswerJson // 最终JSON
	Source        string          // "realtime"
}

// PageSummaryCacheItem page_summary 缓存项
type PageSummaryCacheItem struct {
	PageSummary string    // 页面摘要
	Timestamp   time.Time // 缓存时间
	SessionID   string    // 会话ID
	PageID      string    // 页面ID
}

// =============================================================================
// 1.5 page_summary 持久化缓存（内存 + 文件，TTL 60分钟）
// =============================================================================

const (
	// PageSummaryCacheTTL page_summary 缓存过期时间（60分钟）
	PageSummaryCacheTTL = 60 * time.Minute
	// PageSummaryCacheCleanupInterval 缓存清理间隔
	PageSummaryCacheCleanupInterval = 65 * time.Minute
	// pageSummaryCacheFileName 缓存文件名
	pageSummaryCacheFileName = "page_summary_cache.json"
)

// pageSummaryCache page_summary 内存缓存（session_id:page_id → PageSummaryCacheItem）
var pageSummaryCache = cache.New(PageSummaryCacheTTL, PageSummaryCacheCleanupInterval)

// pageSummaryCacheBasePath 缓存文件路径（启动时初始化）
var pageSummaryCacheBasePath string

// InitPageSummaryCache 初始化 page_summary 缓存，从文件加载已有缓存
// @param basePath 存储根目录
func InitPageSummaryCache(basePath string) {
	pageSummaryCacheBasePath = basePath
	loadPageSummaryCacheFromFile()
}

// loadPageSummaryCacheFromFile 从文件加载缓存到内存
func loadPageSummaryCacheFromFile() {
	if pageSummaryCacheBasePath == "" {
		return
	}
	cacheFile := filepath.Join(pageSummaryCacheBasePath, pageSummaryCacheFileName)
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		// 文件不存在或读取失败，忽略
		return
	}

	var items map[string]PageSummaryCacheItem
	if err := json.Unmarshal(data, &items); err != nil {
		fmt.Printf("[PageSummaryCache] 解析缓存文件失败: %v\n", err)
		return
	}

	now := time.Now()
	loadedCount := 0
	for key, item := range items {
		// 检查是否过期（超过 TTL）
		if now.Sub(item.Timestamp) > PageSummaryCacheTTL {
			continue
		}
		pageSummaryCache.Set(key, item, cache.DefaultExpiration)
		loadedCount++
	}
	if loadedCount > 0 {
		fmt.Printf("[PageSummaryCache] 从文件加载缓存: %d 条有效记录\n", loadedCount)
	}
}

// savePageSummaryCacheToFile 将内存缓存保存到文件
func savePageSummaryCacheToFile() {
	if pageSummaryCacheBasePath == "" {
		return
	}

	// 收集所有未过期的缓存项
	items := make(map[string]PageSummaryCacheItem)
	for _, itemRaw := range pageSummaryCache.Items() {
		item, ok := itemRaw.Object.(PageSummaryCacheItem)
		if !ok {
			continue
		}
		// 检查是否过期
		if time.Now().Sub(item.Timestamp) > PageSummaryCacheTTL {
			continue
		}
		key := makePageSummaryKey(item.SessionID, item.PageID)
		items[key] = item
	}

	if len(items) == 0 {
		return
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		fmt.Printf("[PageSummaryCache] 序列化缓存失败: %v\n", err)
		return
	}

	cacheFile := filepath.Join(pageSummaryCacheBasePath, pageSummaryCacheFileName)
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		fmt.Printf("[PageSummaryCache] 保存缓存文件失败: %v\n", err)
		return
	}

	fmt.Printf("[PageSummaryCache] 保存缓存到文件: %d 条记录\n", len(items))
}

// makePageSummaryKey 生成缓存键
// @param sessionID 会话ID
// @param pageID   页面ID
// @return 缓存键格式: sessionID:pageID
func makePageSummaryKey(sessionID, pageID string) string {
	return sessionID + ":" + pageID
}

// SavePageSummaryCache 保存 page_summary 到缓存（自动去重）+ 持久化到文件
// @param sessionID  会话ID
// @param pageID    页面ID
// @param pageSummary 页面摘要
func SavePageSummaryCache(sessionID, pageID, pageSummary string) {
	key := makePageSummaryKey(sessionID, pageID)
	item := PageSummaryCacheItem{
		PageSummary: pageSummary,
		Timestamp:   time.Now(),
		SessionID:   sessionID,
		PageID:      pageID,
	}
	pageSummaryCache.Set(key, item, cache.DefaultExpiration)
	fmt.Printf("[PageSummaryCache] 保存缓存: %s (%d 字符)\n", key, len(pageSummary))

	// 持久化到文件
	savePageSummaryCacheToFile()
}

// GetPageSummaryContext 获取指定 session 的答题上下文（排除当前页面）
// @param sessionID    会话ID
// @param currentPageID 当前页面ID（排除自身）
// @return 上下文字符串，空则返回空字符串
func GetPageSummaryContext(sessionID, currentPageID string) string {
	var contextItems []string

	// 遍历缓存，收集同 session 的 page_summary
	for _, itemRaw := range pageSummaryCache.Items() {
		item, ok := itemRaw.Object.(PageSummaryCacheItem)
		if !ok {
			continue
		}

		// 只收集同 session、非当前页面的记录
		if item.SessionID != sessionID || item.PageID == currentPageID {
			continue
		}

		if item.PageSummary != "" {
			contextItems = append(contextItems, fmt.Sprintf("· %s: %s", item.PageID, item.PageSummary))
		}
	}

	if len(contextItems) == 0 {
		return ""
	}

	// 拼接上下文
	var sb strings.Builder
	sb.WriteString("【答题上下文】（仅作背景参考）\n")
	for _, ctx := range contextItems {
		sb.WriteString(ctx)
		sb.WriteString("\n")
	}
	sb.WriteString("\n注意: 所有题目上下文（如\"题目N\"说明）仅作背景参考，")
	sb.WriteString("当前答题必须严格依据当前题干内容；")
	sb.WriteString("若当前题干与上下文冲突或重复，以当前题干为准，")
	sb.WriteString("主动忽略上下文中对\"题目N\"的冗余描述或约束。")

	context := sb.String()
	fmt.Printf("[PageSummaryCache] 获取上下文: %d 条记录, %d 字符\n", len(contextItems), len(context))
	return context
}

// GetPageSummaryCacheStats 获取缓存统计信息
func GetPageSummaryCacheStats() (itemCount int, hitCount int64) {
	return pageSummaryCache.ItemCount(), 0
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
		AnswerSystemPrompt:     cfg.AnswerSystemPrompt,
		AnswerUserPromptPrefix: cfg.AnswerUserPromptPrefix,
		PersonaContextPrefix:   cfg.PersonaContextPrefix,
	}, nil
}

// LoadSemanticPersona 加载语义化人设描述
//
//	从 configs/prompts.yaml 的 persona_generic.description 字段读取
//	若 description 非空则返回，作为"答案题目的人设"优先使用
//	否则返回空字符串，触发 fallback 到系统预设人设
//
// @returns 语义化人设描述文本，error
func LoadSemanticPersona() (string, error) {
	data, err := os.ReadFile(promptsYAMLPath)
	if err != nil {
		return "", fmt.Errorf("LoadSemanticPersona: read %s: %w", promptsYAMLPath, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("LoadSemanticPersona: parse yaml: %w", err)
	}

	// 提取 persona_generic.description
	generic, ok := raw["persona_generic"]
	if !ok {
		return "", nil
	}

	genericMap, ok := generic.(map[string]interface{})
	if !ok {
		return "", nil
	}

	description, ok := genericMap["description"]
	if !ok {
		return "", nil
	}

	descStr, ok := description.(string)
	if !ok || descStr == "" {
		return "", nil
	}

	// 去除首尾空白
	descStr = strings.TrimSpace(descStr)
	if descStr == "" {
		return "", nil
	}

	fmt.Printf("[LoadSemanticPersona] 读取到语义化人设描述: %d 字符\n", len(descStr))
	return descStr, nil
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
//
//	内部加载人设，调用 AI 生成推荐答案
//
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
		Source:       "ai",
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
	YamlContent string
	Answers     []Interaction
	Source      string // "cache" / "ai"
	Meta        PageMetadata
	CachePath   string // YAML 缓存路径
	AnswerCache string // 答案缓存路径
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
		YamlContent: yamlResult.YamlContent,
		Answers:     answerResult.Interactions,
		Source:      yamlResult.Source,
		Meta:        yamlResult.Meta,
		CachePath:   yamlResult.CachePath,
		AnswerCache: answerResult.CachePath,
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

// =============================================================================
// 7. 合并AI调用：GenerateRealtimeAnswerOnly（单次AI调用生成推荐答案）
// =============================================================================

// GenerateRealtimeAnswerOnly 缓存命中时，单次AI调用生成推荐答案
//
//	流程：提取元数据 → 解析pageSettings → 加载人设 → 构建Prompt → 调用AI → 解析响应 → 构建FinalJson
//
// @param cleanHTML   清洗后的实时HTML
// @param cachedYaml  缓存的YAML模板
// @param surveyID    问卷ID
// @param basePath    存储根目录
// @returns           RealtimeAnswerOnlyResult, error
func GenerateRealtimeAnswerOnly(cleanHTML, cachedYaml, surveyID, sessionID, basePath string) (*RealtimeAnswerOnlyResult, error) {
	// 步骤1: 提取页面元数据
	meta, err := extractPageMetadata(surveyID, cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("GenerateRealtimeAnswerOnly: extractPageMetadata failed: %w", err)
	}

	// 步骤2: 从cachedYaml解析pageSettings
	pageSettings := parseYamlPageSettings(cachedYaml)

	// 步骤3: 加载人设（优先级：语义化人设 > 系统预设人设）
	fmt.Println("[GenerateRealtimeAnswerOnly] 开始加载人设...")

	// 优先尝试加载语义化人设（来自 persona_generic.description）
	semanticPersona, err := LoadSemanticPersona()
	if err != nil {
		fmt.Printf("[警告] LoadSemanticPersona 失败: %v，将使用系统预设人设\n", err)
	}

	var effectivePersona string
	var personaSource string // 人设来源标识，用于日志备查

	if semanticPersona != "" {
		// 语义化人设存在，使用答案题目的人设
		effectivePersona = semanticPersona
		personaSource = "semantic (persona_generic.description)"
	} else {
		// 语义化人设为空，fallback 到系统预设人设
		effectivePersona, err = LoadPersona()
		if err != nil {
			return nil, fmt.Errorf("GenerateRealtimeAnswerOnly: LoadPersona failed: %w", err)
		}
		personaSource = "system (persona_fixed/generic)"
	}
	fmt.Printf("[人设加载] 有效人设: %s\n", effectivePersona)
	fmt.Printf("[人设加载] 完成，有效人设长度: %d 字符\n", len(effectivePersona))

	// 步骤4: 获取答题上下文（从 page_summary 缓存）
	answerContext := GetPageSummaryContext(sessionID, meta.PageID)

	// 步骤5: 构建Prompt（使用 effectivePersona + answerContext）
	systemPrompt, userPrompt, err := BuildYamlAnswerPromptWithHtml(effectivePersona, answerContext, cachedYaml, cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("GenerateRealtimeAnswerOnly: BuildYamlAnswerPromptWithHtml failed: %w", err)
	}

	// 步骤6: 调用AI（唯一一次）
	aiResponse, err := CallAI(surveyID, systemPrompt, userPrompt, basePath)
	if err != nil {
		saveAnswerFailureLog(basePath, surveyID, meta.PageID, systemPrompt, userPrompt, aiResponse, err)
		return nil, fmt.Errorf("GenerateRealtimeAnswerOnly: CallAI failed: %w", err)
	}

	// 步骤7: 解析响应
	interactions, pageSummary, err := parseInteractionsResponse(aiResponse)
	if err != nil {
		saveAnswerFailureLog(basePath, surveyID, meta.PageID, systemPrompt, userPrompt, aiResponse, err)
		return nil, fmt.Errorf("GenerateRealtimeAnswerOnly: parse response failed: %w", err)
	}

	// 步骤8: 保存 page_summary 到缓存（按 session_id:page_id 去重）
	if pageSummary != "" {
		SavePageSummaryCache(sessionID, meta.PageID, pageSummary)
	}

	// 步骤9: 构建FinalJson
	finalJson := buildFinalAnswerJson(
		&YamlAnswerResult{
			Interactions: interactions,
			PageSummary:  pageSummary,
		},
		cachedYaml,
		pageSettings,
		surveyID,
	)

	// 步骤10: 保存JSON缓存
	jsonPath, err := saveRealtimeAnswersJson(basePath, surveyID, meta.PageID, finalJson)
	if err != nil {
		fmt.Printf("[警告] saveRealtimeAnswersJson 失败: %v\n", err)
		jsonPath = ""
	}

	// 步骤11: 记录Prompt日志（保持独立）
	fmt.Println("[日志] 记录 Prompt 日志...")
	writeAnswerPromptLog(basePath, surveyID, meta, personaSource, systemPrompt, userPrompt, aiResponse)
	fmt.Printf("[日志] Prompt 日志记录完成 (来源: %s)\n", personaSource)

	// 步骤12: 打印摘要
	printRealtimeAnswerOnlyResult(meta, interactions, pageSummary, pageSettings, jsonPath)

	return &RealtimeAnswerOnlyResult{
		Interactions:  interactions,
		PageSummary:   pageSummary,
		PageID:        meta.PageID,
		PageSettings:  pageSettings,
		YamlStructure: cachedYaml,
		FinalJson:     finalJson,
		Source:        "realtime",
	}, nil
}

// BuildYamlAnswerPromptWithHtml 构建答案推荐Prompt（包含HTML内容）
//
//	在 BuildYamlAnswerPrompt 基础上，追加HTML内容供AI参考
//
// @param persona        人设信息
// @param yamlStructure  YAML结构字符串
// @param htmlContent    HTML内容
// @param answerContext  答题上下文（来自 page_summary 缓存）
// @returns              systemPrompt, userPrompt, error
func BuildYamlAnswerPromptWithHtml(persona, answerContext, yamlStructure, htmlContent string) (string, string, error) {
	cfg, err := LoadYamlAnswerConfig()
	if err != nil {
		return "", "", fmt.Errorf("BuildYamlAnswerPromptWithHtml: %w", err)
	}

	systemPrompt := cfg.AnswerSystemPrompt

	var userPrompt strings.Builder

	// 1. 人设上下文
	if persona != "" {
		userPrompt.WriteString(cfg.PersonaContextPrefix)
		userPrompt.WriteString(persona)
		userPrompt.WriteString("\n\n")
	}

	// 2. 答题上下文（人设之后、YAML 之前）
	if answerContext != "" {
		userPrompt.WriteString(answerContext)
		userPrompt.WriteString("\n\n")
	}

	// 3. 用户提示前缀
	userPrompt.WriteString(cfg.AnswerUserPromptPrefix)
	userPrompt.WriteString("\n\n")

	// 4. YAML结构
	userPrompt.WriteString("```yaml\n")
	userPrompt.WriteString(yamlStructure)
	userPrompt.WriteString("\n```\n\n")

	// 5. HTML内容（供AI参考生成答案）
	userPrompt.WriteString("--- 当前页面HTML ---\n")
	userPrompt.WriteString("```html\n")
	userPrompt.WriteString(htmlContent)
	userPrompt.WriteString("\n```\n")

	return systemPrompt, userPrompt.String(), nil
}

// writeAnswerPromptLog 记录Prompt日志
// @param personaSource 人设来源说明，用于日志备查
func writeAnswerPromptLog(basePath, surveyID string, meta PageMetadata, personaSource string, systemPrompt, userPrompt, aiResponse string) {
	logDir := filepath.Join(basePath, "logs", surveyID)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("[警告] writeAnswerPromptLog: mkdir 失败: %v\n", err)
		return
	}

	sep := strings.Repeat("=", 80)
	var sb strings.Builder

	sb.WriteString(sep + "\n")
	sb.WriteString("【单次AI调用】推荐答案 Prompt 日志\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("时间: %s\n", meta.Timestamp))
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", surveyID))
	sb.WriteString(fmt.Sprintf("page_id: %s\n", meta.PageID))
	sb.WriteString(fmt.Sprintf("人设来源: %s\n", personaSource))

	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("=== System Prompt (%d字符) ===\n", len(systemPrompt)))
	sb.WriteString(systemPrompt + "\n")

	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("=== User Prompt (%d字符) ===\n", len(userPrompt)))
	sb.WriteString(userPrompt + "\n")

	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("=== AI 响应 (%d字符) ===\n", len(aiResponse)))
	sb.WriteString(aiResponse + "\n")

	date := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, fmt.Sprintf("answer_only_%s_%s.log", surveyID, date))

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[警告] writeAnswerPromptLog: open 失败: %v\n", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(sb.String()); err != nil {
		fmt.Printf("[警告] writeAnswerPromptLog: write 失败: %v\n", err)
	}
}

// printRealtimeAnswerOnlyResult 打印单次AI调用结果摘要
func printRealtimeAnswerOnlyResult(meta PageMetadata, interactions []Interaction, pageSummary string, pageSettings PageSettings, jsonPath string) {
	fmt.Println()
	fmt.Println("=== 单次AI调用：推荐答案生成完成 ===")
	fmt.Printf("SurveyID: %s\n", meta.SurveyID)
	fmt.Printf("PageID: %s\n", meta.PageID)
	fmt.Printf("Timestamp: %s\n", meta.Timestamp)
	fmt.Printf("题目数量: %d\n", len(interactions))
	fmt.Printf("PageSummary: %s\n", pageSummary)
	fmt.Printf("HasNextButton: %v\n", pageSettings.HasNextButton)
	fmt.Printf("HasSubmitButton: %v\n", pageSettings.HasSubmitButton)
	if jsonPath != "" {
		fmt.Printf("JSON保存路径: %s\n", jsonPath)
	}
	fmt.Printf("来源: realtime\n")
	fmt.Println()

	for i, item := range interactions {
		fmt.Printf("[%d] %s\n", i+1, item.Subject)
		if arr, ok := item.ShowAnswer.([]interface{}); ok {
			fmt.Printf("    → %v\n", arr)
		} else {
			fmt.Printf("    → %v\n", item.ShowAnswer)
		}
	}
	fmt.Println()
}

// extractAnswersWithContext 生成推荐答案（带人设 + 答题上下文）
//   用于 GenerateRealtimeYamlWithAnswers 中
// @param yamlData      YAML 数据
// @param surveyID      问卷ID
// @param sessionID     会话ID
// @param currentPageID 当前页面ID（排除自身）
// @param basePath      存储根目录
// @param answerContext 答题上下文（已组装好的）
// @returns             YamlAnswerResult, error
func extractAnswersWithContext(yamlData, surveyID, sessionID, currentPageID, basePath, answerContext string) (*YamlAnswerResult, error) {
	// 加载人设
	semanticPersona, err := LoadSemanticPersona()
	var effectivePersona string
	if semanticPersona != "" {
		effectivePersona = semanticPersona
	} else {
		effectivePersona, err = LoadPersona()
		if err != nil {
			return nil, fmt.Errorf("extractAnswersWithContext: LoadPersona failed: %w", err)
		}
	}

	fmt.Printf("[人设加载] 有效人设长度: %d 字符\n", len(effectivePersona))

	// 构建 Prompt
	systemPrompt, userPrompt, err := BuildYamlAnswerPromptWithHtml(effectivePersona, answerContext, yamlData, "")
	if err != nil {
		return nil, fmt.Errorf("extractAnswersWithContext: BuildYamlAnswerPromptWithHtml failed: %w", err)
	}

	// 调用 AI
	aiResponse, err := CallAI(surveyID, systemPrompt, userPrompt, basePath)
	if err != nil {
		saveAnswerFailureLog(basePath, surveyID, currentPageID, systemPrompt, userPrompt, aiResponse, err)
		return nil, fmt.Errorf("extractAnswersWithContext: CallAI failed: %w", err)
	}

	// 解析响应
	interactions, pageSummary, err := parseInteractionsResponse(aiResponse)
	if err != nil {
		saveAnswerFailureLog(basePath, surveyID, currentPageID, systemPrompt, userPrompt, aiResponse, err)
		return nil, fmt.Errorf("extractAnswersWithContext: parse response failed: %w", err)
	}

	return &YamlAnswerResult{
		Interactions: interactions,
		PageSummary:  pageSummary,
		Source:      "ai",
	}, nil
}
