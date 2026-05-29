package extractor

// yaml_answer.go — 步骤 4：生成推荐答案
//   使用 YAML 占位符模板 + HTML + 人设 + 题型专属提示词 → 推荐答案 JSON

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"survey.cli/core/database"
)

// min 返回两个整数中较小的值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// =============================================================================
// 1. 数据结构定义
// =============================================================================

// Interaction 单个题目的推荐答案
type Interaction struct {
	Subject      string      `yaml:"subject" json:"subject"`
	ShowAnswer   interface{} `yaml:"showAnswer" json:"showAnswer"`      // string 或 []string
	QuestionType string      `yaml:"questionType" json:"questionType"`   // 题目类型
	// 开放题字数限制（非 AI 返回，验证前由代码填写）
	WordLimit WordLimit `yaml:"wordLimit" json:"wordLimit"`
	// 开放题验证结果（非 AI 返回，验证后由 validator 填写）
	HumansPass           *bool  `yaml:"-" json:"-"`
	PromptSuggestionNote string `yaml:"-" json:"-"`
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

// RealtimeAnswerOnlyResult 推荐答案结果（单次AI调用）
type RealtimeAnswerOnlyResult struct {
	Interactions  []Interaction   // 推荐答案列表
	PageSummary   string          // 页面摘要
	PageID        string          // 页面ID
	PageSettings  PageSettings    // 页面设置
	YamlStructure string          // YAML结构
	FinalJson     FinalAnswerJson // 最终JSON
	Source        string          // "realtime"
	SystemPrompt  string          // 发给AI的 system prompt
	UserPrompt    string          // 发给AI的 user prompt
}

// PageSummaryCacheItem page_summary 缓存项
type PageSummaryCacheItem struct {
	PageSummary string    `json:"page_summary"`
	Timestamp   time.Time `json:"timestamp"`
	PageID      string    `json:"page_id"`
}

// PageSummaryCacheFile 按 survey_id 存储的缓存文件结构
type PageSummaryCacheFile struct {
	Items     []PageSummaryCacheItem `json:"items"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// pageSummaryCacheBasePath 缓存文件路径
var pageSummaryCacheBasePath string

// =============================================================================
// 1.5 page_summary 持久化缓存
// =============================================================================

const (
	PageSummaryCacheTTL      = 30 * time.Minute
	PageSummaryCacheDir      = "page_summary_cache"
	PageSummaryCacheMaxItems = 99
)

// InitPageSummaryCache 初始化 page_summary 缓存（设置 basePath）
func InitPageSummaryCache(basePath string) {
	pageSummaryCacheBasePath = basePath
}

// loadPageSummaryCacheFile 加载指定 survey_id 的缓存文件
func loadPageSummaryCacheFile(surveyIDKey string) *PageSummaryCacheFile {
	cacheDir := filepath.Join(pageSummaryCacheBasePath, PageSummaryCacheDir)
	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%s.json", surveyIDKey))

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil
	}

	var cacheData PageSummaryCacheFile
	if err := json.Unmarshal(data, &cacheData); err != nil {
		fmt.Printf("[PageSummaryCache] 解析缓存文件失败: %v\n", err)
		return nil
	}

	if time.Since(cacheData.UpdatedAt) > PageSummaryCacheTTL {
		os.Remove(cacheFile)
		return nil
	}

	return &cacheData
}

// savePageSummaryCacheFile 保存指定 survey_id 的缓存文件
func savePageSummaryCacheFile(surveyIDKey string, cacheData *PageSummaryCacheFile) {
	cacheDir := filepath.Join(pageSummaryCacheBasePath, PageSummaryCacheDir)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("[PageSummaryCache] 创建缓存目录失败: %v\n", err)
		return
	}

	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%s.json", surveyIDKey))
	cacheData.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(cacheData, "", "  ")
	if err != nil {
		fmt.Printf("[PageSummaryCache] 序列化缓存失败: %v\n", err)
		return
	}

	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		fmt.Printf("[PageSummaryCache] 保存缓存文件失败: %v\n", err)
		return
	}

	fmt.Printf("[PageSummaryCache] 保存缓存: survey_id=%s, items=%d\n", surveyIDKey, len(cacheData.Items))
}

// SavePageSummaryCache 保存 page_summary 到缓存文件
func SavePageSummaryCache(surveyIDKey string, pageID, pageSummary string) {
	cacheData := loadPageSummaryCacheFile(surveyIDKey)
	if cacheData == nil {
		cacheData = &PageSummaryCacheFile{
			Items:     make([]PageSummaryCacheItem, 0),
			UpdatedAt: time.Now(),
		}
	}

	cacheData.Items = append(cacheData.Items, PageSummaryCacheItem{
		PageSummary: pageSummary,
		Timestamp:   time.Now(),
		PageID:      pageID,
	})

	if len(cacheData.Items) > PageSummaryCacheMaxItems {
		cacheData.Items = cacheData.Items[len(cacheData.Items)-PageSummaryCacheMaxItems:]
	}

	savePageSummaryCacheFile(surveyIDKey, cacheData)
}

// GetPageSummaryContext 获取指定 survey 的答题上下文（排除当前页面）
func GetPageSummaryContext(surveyIDKey string, currentPageID string) string {
	cacheData := loadPageSummaryCacheFile(surveyIDKey)
	if cacheData == nil || len(cacheData.Items) == 0 {
		return ""
	}

	var contextItems []string
	for _, item := range cacheData.Items {
		if item.PageID == currentPageID {
			continue
		}
		if item.PageSummary != "" {
			contextItems = append(contextItems, fmt.Sprintf("· %s: %s", item.PageID, item.PageSummary))
		}
	}

	if len(contextItems) == 0 {
		return ""
	}

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
	fmt.Printf("[PageSummaryCache] 获取上下文: survey_id=%s, items=%d, chars=%d\n", surveyIDKey, len(contextItems), len(context))
	return context
}

// GetPageSummaryCacheStats 获取缓存统计信息
func GetPageSummaryCacheStats() (itemCount int, hitCount int64) {
	cacheDir := filepath.Join(pageSummaryCacheBasePath, PageSummaryCacheDir)
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return 0, 0
	}

	totalItems := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		key := strings.TrimSuffix(entry.Name(), ".json")
		cacheData := loadPageSummaryCacheFile(key)
		if cacheData != nil {
			totalItems += len(cacheData.Items)
		}
	}
	return totalItems, 0
}

// =============================================================================
// 2. 人设加载
// =============================================================================

// LoadSemanticPersona 加载语义化人设描述（survey_persona.__default__.description）
//
// @returns 描述文本，error
func LoadSemanticPersona() (string, error) {
	rec, err := database.LoadDefaultPersona()
	if err != nil {
		return "", fmt.Errorf("LoadSemanticPersona: %w", err)
	}

	descStr := strings.TrimSpace(rec.Description)
	if descStr == "" {
		return "", nil
	}

	fmt.Printf("[LoadSemanticPersona] 读取到语义化人设描述: %d 字符\n", len(descStr))
	return descStr, nil
}

// =============================================================================
// 3. Prompt 构建（DB 驱动）
// =============================================================================

// buildAnswerSystemPrompt 从数据库组装答案推荐系统提示词
//
//	使用 analyze 组装顺序，qtype.* 键仅包含匹配 detectedTypes 的部分；
//	人设追加到末尾。
//
// @param detectedTypes  检测到的题型列表（空时包含所有题型块）
// @param persona        人设文本（追加到末尾）
// @returns              系统提示词，error
func buildAnswerSystemPrompt(detectedTypes []string, persona string) (string, error) {
	allPrompts, err := database.LoadAllPrompts()
	if err != nil {
		return "", fmt.Errorf("buildAnswerSystemPrompt: LoadAllPrompts: %w", err)
	}

	assemblyKeys, err := database.LoadAssemblyOrder("analyze")
	if err != nil {
		return "", fmt.Errorf("buildAnswerSystemPrompt: LoadAssemblyOrder: %w", err)
	}

	// 构建 detectedTypes 集合（空表示不过滤）
	typeSet := make(map[string]bool, len(detectedTypes))
	for _, t := range detectedTypes {
		typeSet[strings.ToLower(t)] = true
	}
	filterTypes := len(typeSet) > 0

	// system.analyze 作为前缀（与 buildPrompt 保持一致）
	blocks := []string{allPrompts["system.analyze"]}

	for _, key := range assemblyKeys {
		if strings.HasPrefix(key, "qtype.") && filterTypes {
			// qtype.<type> — 仅保留命中的题型
			parts := strings.SplitN(key, ".", 3)
			if len(parts) >= 2 && !typeSet[strings.ToLower(parts[1])] {
				continue
			}
		}
		if block := allPrompts[key]; block != "" {
			blocks = append(blocks, block)
		}
	}

	if persona != "" {
		blocks = append(blocks, "【当前受访者人设】\n"+persona)
	}

	return strings.Join(blocks, "\n\n"), nil
}

// buildAnswerUserPrompt 组装答案推荐用户提示词
//
// @param answerContext  答题上下文（来自 page_summary 缓存）
// @param yamlTemplate   YAML 占位符模板
// @param htmlContent    清洗后 HTML
// @returns              用户提示词字符串
func buildAnswerUserPrompt(answerContext, yamlTemplate, htmlContent string) string {
	var sb strings.Builder

	if answerContext != "" {
		sb.WriteString(answerContext)
		sb.WriteString("\n\n")
	}

	sb.WriteString("--- 当前页面HTML（数据来源）---\n")
	sb.WriteString("```html\n")
	sb.WriteString(htmlContent)
	sb.WriteString("\n```\n\n")

	sb.WriteString("--- YAML模板（格式参考，禁止使用占位符文本！）---\n")
	sb.WriteString("```yaml\n")
	sb.WriteString(yamlTemplate)
	sb.WriteString("\n```\n")

	return sb.String()
}

// =============================================================================
// 4. 响应解析
// =============================================================================

// parseInteractionsResponse 解析 AI 返回的 JSON 响应
//
//	支持两种格式：
//	  直接格式: {"interactions":[...],"page_summary":"..."}
//	  data 包装: {"data":{"interactions":[...],"page_summary":"...",...}}
func parseInteractionsResponse(content string) ([]Interaction, string, error) {
	cleaned := stripMarkdownFence(content)

	fmt.Println()
	fmt.Println("=== [DEBUG] AI Raw Response ===")
	if len(cleaned) > 1000 {
		fmt.Printf("Length: %d chars\n", len(cleaned))
		fmt.Printf("First 500 chars:\n%s\n...\n", cleaned[:500])
		fmt.Printf("Last 200 chars:\n...%s\n", cleaned[len(cleaned)-200:])
	} else {
		fmt.Printf("Length: %d chars\n", len(cleaned))
		fmt.Printf("Content:\n%s\n", cleaned)
	}
	fmt.Println("=== [DEBUG] End Response ===")
	fmt.Println()

	// 优先尝试 data 包装格式（DB output_format 要求的格式）
	var wrapped struct {
		Data InteractionsResponse `json:"data"`
	}
	if err := json.Unmarshal([]byte(cleaned), &wrapped); err == nil && len(wrapped.Data.Interactions) > 0 {
		return wrapped.Data.Interactions, wrapped.Data.PageSummary, nil
	}

	// 备选：直接格式
	var resp InteractionsResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		return nil, "", fmt.Errorf("json.Unmarshal failed: %w, content: %s", err, cleaned[:min(200, len(cleaned))])
	}

	if len(resp.Interactions) == 0 {
		return nil, "", fmt.Errorf("no interactions found in response")
	}

	return resp.Interactions, resp.PageSummary, nil
}

// =============================================================================
// 5. 辅助方法
// =============================================================================

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

// writeAnswerPromptLog 记录 Prompt 日志
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
	_, _ = f.WriteString(sb.String())
}

// printRealtimeAnswerOnlyResult 打印推荐答案摘要
func printRealtimeAnswerOnlyResult(meta PageMetadata, interactions []Interaction, pageSummary string, pageSettings PageSettings, jsonPath string) {
	fmt.Println()
	fmt.Println("=== 单次AI调用：推荐答案生成完成 ===")
	fmt.Printf("SurveyID: %s\n", meta.SurveyID)
	fmt.Printf("PageID: %s\n", meta.PageID)
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

// =============================================================================
// 6. 主入口：GenerateRealtimeAnswerOnlyWithMeta
// =============================================================================

// GenerateRealtimeAnswerOnlyWithMeta 生成推荐答案，使用已知的 meta 和检测到的题型
//
//	步骤：加载人设 → 组装 Prompt（DB驱动）→ 调用AI → 解析 → 构建 FinalJson
//
// @param meta           已知页面元数据（含 page_id）
// @param cleanHTML      清洗后 HTML
// @param cachedYaml     YAML 占位符模板
// @param surveyID       问卷 ID
// @param surveyIDInt    问卷 ID（整型）
// @param basePath       存储根目录
// @param detectedTypes  题型列表（步骤 3 检测结果）
// @returns              RealtimeAnswerOnlyResult, error
func GenerateRealtimeAnswerOnlyWithMeta(meta PageMetadata, cleanHTML, cachedYaml, surveyID string, surveyIDInt int64, basePath string, detectedTypes []string) (*RealtimeAnswerOnlyResult, error) {
	pageSettings := parseYamlPageSettings(cachedYaml)

	fmt.Println("[GenerateRealtimeAnswerOnlyWithMeta] 开始加载人设...")

	semanticPersona, err := LoadSemanticPersona()
	if err != nil {
		fmt.Printf("[警告] LoadSemanticPersona 失败: %v，将使用系统预设人设\n", err)
	}

	var effectivePersona string
	var personaSource string

	if semanticPersona != "" {
		effectivePersona = semanticPersona
		personaSource = "semantic (persona_generic.description)"
	} else {
		effectivePersona, err = LoadPersona()
		if err != nil {
			return nil, fmt.Errorf("GenerateRealtimeAnswerOnlyWithMeta: LoadPersona failed: %w", err)
		}
		personaSource = "system (persona_fixed/generic)"
	}
	fmt.Printf("[人设加载] 完成，有效人设长度: %d 字符\n", len(effectivePersona))

	surveyIDKey := strconv.FormatInt(surveyIDInt, 10)
	answerContext := GetPageSummaryContext(surveyIDKey, meta.PageID)

	// 组装系统提示词（DB 驱动，按 detectedTypes 过滤 qtype.* 块）
	systemPrompt, err := buildAnswerSystemPrompt(detectedTypes, effectivePersona)
	if err != nil {
		return nil, fmt.Errorf("GenerateRealtimeAnswerOnlyWithMeta: buildAnswerSystemPrompt failed: %w", err)
	}

	// 组装用户提示词
	userPrompt := buildAnswerUserPrompt(answerContext, cachedYaml, cleanHTML)

	// 调用 AI（JSON 验证模式）
	aiResponse, err := CallAIWithValidation(surveyID, systemPrompt, userPrompt, basePath, true)
	if err != nil {
		saveAnswerFailureLog(basePath, surveyID, meta.PageID, systemPrompt, userPrompt, aiResponse, err)
		return nil, fmt.Errorf("GenerateRealtimeAnswerOnlyWithMeta: CallAI failed: %w", err)
	}

	interactions, pageSummary, err := parseInteractionsResponse(aiResponse)
	if err != nil {
		saveAnswerFailureLog(basePath, surveyID, meta.PageID, systemPrompt, userPrompt, aiResponse, err)
		return nil, fmt.Errorf("GenerateRealtimeAnswerOnlyWithMeta: parse response failed: %w", err)
	}

	if pageSummary != "" {
		SavePageSummaryCache(surveyIDKey, meta.PageID, pageSummary)
	}

		// 步骤 4b：对开放题执行迭代验证（方案 A：强制，方案 C：失败返回 humans_pass=false）
		openTextPrompts := loadOpenTextPrompts()
		for i := range interactions {
			if !IsOpenTextQuestion(interactions[i]) {
				continue
			}
			fmt.Printf("[OpenText] 检测到开放题 [%d]: %s\n", i+1, interactions[i].Subject)
			finalAnswer, humanPass, note, vErr := ValidateOpenTextAnswer(
				interactions[i].Subject, surveyID, basePath, openTextPrompts, interactions[i].WordLimit,
			)
		if vErr != nil {
			fmt.Printf("[OpenText] 验证失败，保留原始答案: %v\n", vErr)
			pass := false
			interactions[i].HumansPass = &pass
			continue
		}
		interactions[i].ShowAnswer = finalAnswer
		interactions[i].HumansPass = &humanPass
		interactions[i].PromptSuggestionNote = note
	}

	finalJson := buildFinalAnswerJson(
		&YamlAnswerResult{
			Interactions: interactions,
			PageSummary:  pageSummary,
		},
		cachedYaml,
		pageSettings,
		surveyID,
		meta.PageID,
		cleanHTML,
	)

	jsonPath, err := saveRealtimeAnswersJson(basePath, surveyID, meta.PageID, finalJson)
	if err != nil {
		fmt.Printf("[警告] saveRealtimeAnswersJson 失败: %v\n", err)
		jsonPath = ""
	}

	fmt.Println("[日志] 记录 Prompt 日志...")
	writeAnswerPromptLog(basePath, surveyID, meta, personaSource, systemPrompt, userPrompt, aiResponse)

	printRealtimeAnswerOnlyResult(meta, interactions, pageSummary, pageSettings, jsonPath)

	return &RealtimeAnswerOnlyResult{
		Interactions:  interactions,
		PageSummary:   pageSummary,
		PageID:        meta.PageID,
		PageSettings:  pageSettings,
		YamlStructure: cachedYaml,
		FinalJson:     finalJson,
		Source:        "realtime",
		SystemPrompt:  systemPrompt,
		UserPrompt:    userPrompt,
	}, nil
}
