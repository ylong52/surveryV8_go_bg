package extractor

// yaml_structure.go — YAML 结构化提示词配置与构建
//   从 configs/yaml_structure_prompt.yaml 加载提示词配置
//   提供独立的 Prompt 构建函数，供外部调用

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// =============================================================================
// 1. 数据结构定义
// =============================================================================

// YamlStructureConfig 对应 configs/yaml_structure_prompt.yaml 的字段
type YamlStructureConfig struct {
	// YAML 结构提取
	SystemPrompt     string `yaml:"STRUCTURE_SYSTEM_PROMPT"`
	UserPromptPrefix string `yaml:"STRUCTURE_USER_PROMPT_PREFIX"`
	// 答案推荐
	AnswerSystemPrompt     string `yaml:"ANSWER_SYSTEM_PROMPT"`
	AnswerUserPromptPrefix string `yaml:"ANSWER_USER_PROMPT_PREFIX"`
	PersonaContextPrefix   string `yaml:"PERSONA_CONTEXT_PREFIX"`
	// 模板转YAML数据
	TemplateToYamlSystemPrompt string `yaml:"TEMPLATE_TO_YAML_SYSTEM_PROMPT"`
	TemplateToYamlUserPrompt   string `yaml:"TEMPLATE_TO_YAML_USER_PROMPT"`
}

// YamlStructurePrompt 构建后的完整提示词
type YamlStructurePrompt struct {
	System string // 系统提示词
	User   string // 用户提示词（含 HTML）
}

// YamlStructureResult 提取结果
type YamlStructureResult struct {
	YamlContent string       // YAML 内容（已去除 markdown 包裹）
	CachePath   string       // 缓存路径
	Meta        PageMetadata // 页面元数据
	Prompt      YamlStructurePrompt
	Source      string // "cache"=命中本地模板，"ai"=AI生成
}

// =============================================================================
// 1.5 PageSettings 页面设置
// =============================================================================

// PageSettings 页面设置
type PageSettings struct {
	HasNextButton     bool `json:"has_next_button"`
	HasSubmitButton   bool `json:"has_submit_button"`
	IsThankYouPage    bool `json:"is_thank_you_page"`
	IsDescriptionPage bool `json:"is_description_page"`
}

// =============================================================================
// 1.6 FinalAnswerJson 最终 JSON 输出
// =============================================================================

// NavButtonItem 导航按钮项
type NavButtonItem struct {
	Role           string            `json:"role"`
	XPath          string            `json:"xpath"`
	Text           string            `json:"text"`
	WaitAfterClick int               `json:"wait_after_click"`
	Attributes     map[string]string `json:"attributes"`
}

// NavButtonsOutput 导航按钮输出
type NavButtonsOutput struct {
	Buttons       []NavButtonItem `json:"buttons"`
	PrimaryButton string          `json:"primary_button"`
}

// PageMetadataOutput page_metadata 输出
type PageMetadataOutput struct {
	PageID          string `json:"page_id"`
	IsMatrixPage    bool   `json:"is_matrix_page"`
	HasNextButton   bool   `json:"has_next_button"`
	HasSubmitButton bool   `json:"has_submit_button"`
	IsThankYouPage  bool   `json:"is_thank_you_page"`
}

// InteractionItem interactions 项
type InteractionItem struct {
	QuestionType       string      `json:"question_type"`
	Subject            string      `json:"subject"`
	RecommendedAnswers interface{} `json:"recommended_answers"` // string or []string
}

// DataOutput data 输出
type DataOutput struct {
	PageMetadata PageMetadataOutput `json:"page_metadata"`
	Interactions []InteractionItem  `json:"interactions"`
	NavButtons   NavButtonsOutput   `json:"nav_buttons"`
	PageSummary  string             `json:"page_summary"`
}

// FinalAnswerJson 最终 JSON 输出
type FinalAnswerJson struct {
	Data DataOutput `json:"data"`
}

// =============================================================================
// 2. 配置加载
// =============================================================================

const yamlStructureConfigPath = "configs/yaml_structure_prompt.yaml"

// LoadYamlStructureConfig 从 configs/yaml_structure_prompt.yaml 读取配置
func LoadYamlStructureConfig() (*YamlStructureConfig, error) {
	// 支持相对路径和绝对路径
	configPath := yamlStructureConfigPath
	if !filepath.IsAbs(configPath) {
		// 尝试相对于工作目录
		wd, err := os.Getwd()
		if err == nil {
			configPath = filepath.Join(wd, configPath)
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("LoadYamlStructureConfig: read %s: %w", configPath, err)
	}

	var cfg YamlStructureConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("LoadYamlStructureConfig: parse yaml: %w", err)
	}

	if cfg.SystemPrompt == "" {
		return nil, fmt.Errorf("LoadYamlStructureConfig: STRUCTURE_SYSTEM_PROMPT is empty in %s", configPath)
	}

	return &cfg, nil
}

// =============================================================================
// 3. Prompt 构建
// =============================================================================

// BuildYamlStructurePrompt 构建 YAML 结构提取专用提示词
//   - 检查 HTML 长度，超过 60000 直接报错
//   - 传给 AI 时保留完整 HTML
//
// @param surveyID  问卷 ID
// @param pageID    页面 ID
// @param cleanHTML 清洗后 HTML（不超过 60000 字符）
// @returns         YamlStructurePrompt，或错误
func BuildYamlStructurePrompt(surveyID, pageID, cleanHTML string) (*YamlStructurePrompt, error) {
	if len(cleanHTML) > 60000 {
		return nil, fmt.Errorf("HTML 长度 %d 超过 60000 字符限制", len(cleanHTML))
	}

	cfg, err := LoadYamlStructureConfig()
	if err != nil {
		return nil, fmt.Errorf("BuildYamlStructurePrompt: %w", err)
	}

	systemPrompt := cfg.SystemPrompt

	userPrompt := cfg.UserPromptPrefix + "\n" +
		"survey_id: " + surveyID + "\n" +
		"page_id: " + pageID + "\n\n" +
		cleanHTML

	return &YamlStructurePrompt{
		System: systemPrompt,
		User:   userPrompt,
	}, nil
}

// BuildYamlStructurePromptFromMeta 从 PageMetadata 构建提示词
// @param surveyID  问卷 ID
// @param meta      页面元数据
// @param cleanHTML 清洗后 HTML
// @returns         YamlStructurePrompt，或错误
func BuildYamlStructurePromptFromMeta(surveyID string, meta PageMetadata, cleanHTML string) (*YamlStructurePrompt, error) {
	return BuildYamlStructurePrompt(surveyID, meta.PageID, cleanHTML)
}

// =============================================================================
// 4. 日志写入
// =============================================================================

// WriteYamlStructurePromptLog 将提示词写入日志文件
// @param basePath   存储根目录
// @param surveyID   问卷 ID
// @param meta       页面元数据
// @param prompt     YAML 提示词
// @param fullHTML   完整 HTML（不截断）
func WriteYamlStructurePromptLog(basePath, surveyID string, meta PageMetadata, prompt YamlStructurePrompt, fullHTML string) error {
	logDir := filepath.Join(basePath, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("WriteYamlStructurePromptLog: mkdir %s: %w", logDir, err)
	}

	sep := strings.Repeat("=", 80)
	var sb strings.Builder

	sb.WriteString(sep + "\n")
	sb.WriteString("【YAML Structure】提示词整理\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("时间: %s\n", meta.Timestamp))
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", surveyID))
	sb.WriteString(fmt.Sprintf("page_id: %s\n", meta.PageID))

	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("=== System Prompt (%d字符) ===\n", len(prompt.System)))
	sb.WriteString(prompt.System + "\n")

	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("=== User Prompt (%d字符，含HTML) ===\n", len(prompt.User)))
	sb.WriteString(prompt.User + "\n")

	date := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", surveyID, date))

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("WriteYamlStructurePromptLog: open %s: %w", logPath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(sb.String()); err != nil {
		return fmt.Errorf("WriteYamlStructurePromptLog: write %s: %w", logPath, err)
	}
	return nil
}

// =============================================================================
// 5. 提取入口（整合 Extract + CallAI + 缓存）
// =============================================================================

// ExtractYamlStructureV2 执行 YAML 结构提取（v2 版本）
//
//	步骤1: 参数验证
//	步骤2: 提取 PageMetadata
//	步骤3: 组装 AI Prompt
//	步骤4: 查找本地 YAML 模板
//	步骤5: 未命中则调用 AI
//	步骤6: 保存 YAML 到缓存目录
//
// @param surveyID  问卷 ID
// @param cleanHTML 清洗后 HTML
// @param basePath  存储根目录
// @returns         YamlStructureResult，或错误
func ExtractYamlStructureV2(surveyID, cleanHTML, basePath string) (*YamlStructureResult, error) {
	// 步骤 1-3：构建提示词
	prompt, err := BuildYamlStructurePrompt(surveyID, "unknown_page_id", cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlStructureV2: BuildYamlStructurePrompt failed: %w", err)
	}

	// 步骤 2：从 HTML 提取 PageMetadata
	meta, err := extractPageMetadata(surveyID, cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlStructureV2: extractPageMetadata failed: %w", err)
	}

	// 更新 prompt 的 pageID
	prompt, err = BuildYamlStructurePrompt(surveyID, meta.PageID, cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlStructureV2: rebuild prompt failed: %w", err)
	}

	// 写入日志
	if err := WriteYamlStructurePromptLog(basePath, surveyID, meta, *prompt, cleanHTML); err != nil {
		// 日志失败不影响主流程
		fmt.Printf("[警告] WriteYamlStructurePromptLog 失败: %v\n", err)
	}

	// 步骤 4：查找本地 YAML 模板
	templatePath := filepath.Join(basePath, "templates", surveyID, meta.PageID+".yaml")

	if yamlContent, err := os.ReadFile(templatePath); err == nil {
		// 命中本地模板
		fmt.Printf("[缓存命中] %s\n", templatePath)
		return &YamlStructureResult{
			YamlContent: string(yamlContent),
			CachePath:   templatePath,
			Meta:        meta,
			Prompt:      *prompt,
			Source:      "cache",
		}, nil
	}

	// 步骤 5：调用 AI
	aiResponse, err := CallAI(surveyID, prompt.System, prompt.User, basePath)
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlStructureV2: CallAI failed: %w", err)
	}

	// 步骤 6：保存到缓存
	cachePath, err := saveYamlToCache(basePath, surveyID, meta.PageID, aiResponse)
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlStructureV2: saveYamlToCache failed: %w", err)
	}

	// 输出摘要
	printYamlStructureResult(meta, cachePath, aiResponse)

	return &YamlStructureResult{
		YamlContent: aiResponse,
		CachePath:   cachePath,
		Meta:        meta,
		Prompt:      *prompt,
		Source:      "ai",
	}, nil
}

// printYamlStructureResult 输出提取结果摘要
func printYamlStructureResult(meta PageMetadata, cachePath, yamlContent string) {
	fmt.Println()
	fmt.Println("=== YAML Structure 提取完成 ===")
	fmt.Printf("SurveyID: %s\n", meta.SurveyID)
	fmt.Printf("PageID: %s\n", meta.PageID)
	fmt.Printf("Timestamp: %s\n", meta.Timestamp)
	fmt.Printf("YAML长度: %d字符\n", len(yamlContent))
	fmt.Printf("缓存路径: %s\n", cachePath)
	fmt.Printf("来源: ai\n")
	fmt.Println()
}

// GetHtmlPreview 返回 HTML 的前 300 字符预览（用于控制台输出）
func (p *YamlStructurePrompt) GetHtmlPreview() string {
	if len(p.User) <= 300 {
		return p.User
	}
	return p.User[:300] + "..."
}

// =============================================================================
// 6. 模板转YAML数据（步骤1核心）
// =============================================================================

// TemplateToYamlResult 模板转YAML结果
type TemplateToYamlResult struct {
	YamlData  string // 最新YAML数据
	CachePath string // 缓存路径
	PageID    string
	SurveyID  string
	PageMeta  PageMetadata
	Source    string // 来源标记: "realtime"=实时生成, "template"=模板转换
}

// GenerateLatestYamlData 根据模板 + 实时HTML 生成最新YAML数据
//
//	流程：读取YAML模板 → 构建Prompt → 调用AI → 保存到storage/logs
//
// @param html         清洗后的HTML
// @param yamlTemplate YAML模板内容（结构参考）
// @param surveyID     问卷ID
// @param basePath     存储根目录
// @returns            TemplateToYamlResult, error
func GenerateLatestYamlData(html, yamlTemplate, surveyID, basePath string) (*TemplateToYamlResult, error) {
	// 步骤1: 提取PageMetadata（获取pageID）
	meta, err := extractPageMetadata(surveyID, html)
	if err != nil {
		return nil, fmt.Errorf("GenerateLatestYamlData: extractPageMetadata failed: %w", err)
	}

	// 步骤2: 加载配置
	cfg, err := LoadYamlStructureConfig()
	if err != nil {
		return nil, fmt.Errorf("GenerateLatestYamlData: LoadYamlStructureConfig failed: %w", err)
	}

	if cfg.TemplateToYamlSystemPrompt == "" {
		return nil, fmt.Errorf("GenerateLatestYamlData: TEMPLATE_TO_YAML_SYSTEM_PROMPT is empty")
	}
	if cfg.TemplateToYamlUserPrompt == "" {
		return nil, fmt.Errorf("GenerateLatestYamlData: TEMPLATE_TO_YAML_USER_PROMPT is empty")
	}

	// 步骤3: 构建Prompt
	systemPrompt := cfg.TemplateToYamlSystemPrompt
	userPrompt := strings.Replace(cfg.TemplateToYamlUserPrompt, "{yamlTemplate}", yamlTemplate, 1)
	userPrompt = strings.Replace(userPrompt, "{html}", html, 1)

	// 步骤4: 调用AI
	aiResponse, err := CallAI(surveyID, systemPrompt, userPrompt, basePath)
	if err != nil {
		return nil, fmt.Errorf("GenerateLatestYamlData: CallAI failed: %w", err)
	}

	// 步骤5: 去除markdown包裹
	yamlData := stripYamlMarkdown(aiResponse)

	// 步骤6: 保存到storage/logs/{surveyID}/{pageID}_yamldata_{YMDHis}.yaml
	cachePath, err := saveTemplateToYamlCache(basePath, surveyID, meta.PageID, yamlData)
	if err != nil {
		// 保存失败不影响主流程，仅打印警告
		fmt.Printf("[警告] saveTemplateToYamlCache 失败: %v\n", err)
		cachePath = ""
	}

	// 步骤7: 打印摘要
	printTemplateToYamlResult(meta, yamlData, cachePath)

	return &TemplateToYamlResult{
		YamlData:  yamlData,
		CachePath: cachePath,
		PageID:    meta.PageID,
		SurveyID:  surveyID,
		PageMeta:  meta,
	}, nil
}

// stripYamlMarkdown 去除markdown代码块包裹
func stripYamlMarkdown(s string) string {
	s = strings.TrimSpace(s)
	// 去除 ```yaml 或 ``` 或 ```yml 开头的行
	if strings.HasPrefix(s, "```") {
		lines := strings.SplitN(s, "\n", 2)
		if len(lines) > 1 {
			s = lines[1]
		}
	}
	// 去除结尾的 ```
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

// saveTemplateToYamlCache 保存模板生成的YAML数据到缓存
func saveTemplateToYamlCache(basePath, surveyID, pageID, yamlData string) (string, error) {
	cacheDir := filepath.Join(basePath, "logs", surveyID)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("saveTemplateToYamlCache: mkdir %s: %w", cacheDir, err)
	}

	ts := time.Now().Format("20060102150405")
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%s_yamldata_%s.yaml", pageID, ts))

	if err := os.WriteFile(cachePath, []byte(yamlData), 0644); err != nil {
		return "", fmt.Errorf("saveTemplateToYamlCache: write %s: %w", cachePath, err)
	}

	return cachePath, nil
}

// printTemplateToYamlResult 打印模板转YAML结果
func printTemplateToYamlResult(meta PageMetadata, yamlData, cachePath string) {
	fmt.Println()
	fmt.Println("=== 模板转YAML数据生成完成 ===")
	fmt.Printf("SurveyID: %s\n", meta.SurveyID)
	fmt.Printf("PageID: %s\n", meta.PageID)
	fmt.Printf("YAML长度: %d字符\n", len(yamlData))
	if cachePath != "" {
		fmt.Printf("缓存路径: %s\n", cachePath)
	}
	fmt.Println()
}

// =============================================================================
// 7. 步骤 7: 生成实时 YAML 数据（仅当 Source="cache" 时触发）
// =============================================================================

// RealtimeYamlResult 实时 YAML 数据生成结果
type RealtimeYamlResult struct {
	YamlData     string       // 实时的最新 YAML 数据
	CachePath    string       // 保存路径
	PageID       string       // 页面 ID
	SurveyID     string       // 问卷 ID
	PageMeta     PageMetadata // 页面元数据
	PageSettings PageSettings // 页面设置
	Source       string       // "realtime" = 实时生成
}

// GenerateRealtimeYamlData 根据缓存的 YAML 模板 + 实时 HTML 生成最新 YAML 数据
//
//	仅当 YamlStructureResult.Source == "cache" 时调用
//	流程：提取元数据 → 加载配置 → 构建Prompt → 调用AI → 保存到 realtime-yamldata/
//
// @param cleanHTML    清洗后的实时 HTML
// @param cachedYaml   步骤 6 返回的 YamlStructureResult.YamlContent（缓存的模板）
// @param surveyID     问卷 ID
// @param basePath     存储根目录
// @returns            RealtimeYamlResult, error
func GenerateRealtimeYamlData(cleanHTML, cachedYaml, surveyID, basePath string) (*RealtimeYamlResult, error) {
	// 步骤 1: 前置条件校验
	if cleanHTML == "" {
		return nil, fmt.Errorf("GenerateRealtimeYamlData: cleanHTML 不能为空")
	}
	if cachedYaml == "" {
		return nil, fmt.Errorf("GenerateRealtimeYamlData: cachedYaml 不能为空")
	}

	// 步骤 2: 提取页面元数据
	meta, err := extractPageMetadata(surveyID, cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("GenerateRealtimeYamlData: extractPageMetadata failed: %w", err)
	}

	// 步骤 3: 加载 Prompt 配置
	cfg, err := LoadYamlStructureConfig()
	if err != nil {
		return nil, fmt.Errorf("GenerateRealtimeYamlData: LoadYamlStructureConfig failed: %w", err)
	}

	if cfg.TemplateToYamlSystemPrompt == "" {
		return nil, fmt.Errorf("GenerateRealtimeYamlData: TEMPLATE_TO_YAML_SYSTEM_PROMPT 配置为空")
	}
	if cfg.TemplateToYamlUserPrompt == "" {
		return nil, fmt.Errorf("GenerateRealtimeYamlData: TEMPLATE_TO_YAML_USER_PROMPT 配置为空")
	}

	// 步骤 4: 构建 AI Prompt
	systemPrompt := cfg.TemplateToYamlSystemPrompt
	userPrompt := cfg.TemplateToYamlUserPrompt
	userPrompt = strings.Replace(userPrompt, "{yamlTemplate}", cachedYaml, 1)
	userPrompt = strings.Replace(userPrompt, "{html}", cleanHTML, 1)

	// 步骤 4.5: 从 cachedYaml 中提取 page_metadata
	pageMetadataBlock := extractPageMetadataBlock(cachedYaml)

	// 步骤 5: 调用 AI（失败则记录日志并返回错误）
	aiResponse, err := CallAI(surveyID, systemPrompt, userPrompt, basePath)
	if err != nil {
		// 记录详细错误日志
		saveRealtimeYamlFailureLog(basePath, surveyID, meta.PageID, systemPrompt, userPrompt, "", err)
		fmt.Printf("[错误] GenerateRealtimeYamlData: CallAI 失败 - %v\n", err)
		return nil, fmt.Errorf("GenerateRealtimeYamlData: CallAI failed: %w", err)
	}

	// 步骤 6: 去除 markdown 包裹
	yamlData := stripYamlMarkdown(aiResponse)

	// 步骤 6.5: 确保 page_metadata 正确
	yamlData = mergePageMetadata(yamlData, pageMetadataBlock)

	// 步骤 6.6: 解析 page_settings
	pageSettings := parseYamlPageSettings(yamlData)

	// 步骤 7: 保存到 storage/logs/{surveyID}/realtime-yamldata/{pageID}_{YMDHis}.yaml
	cachePath, err := saveRealtimeYamlData(basePath, surveyID, meta.PageID, yamlData)
	if err != nil {
		// 保存失败不影响主流程，仅打印警告
		fmt.Printf("[警告] saveRealtimeYamlData 失败: %v\n", err)
		cachePath = ""
	}

	// 步骤 8: 记录 Prompt 日志
	writeRealtimePromptLog(basePath, surveyID, meta, systemPrompt, userPrompt, yamlData)

	// 步骤 9: 打印摘要
	printRealtimeYamlResult(meta, yamlData, cachePath)

	return &RealtimeYamlResult{
		YamlData:     yamlData,
		CachePath:    cachePath,
		PageID:       meta.PageID,
		SurveyID:     surveyID,
		PageMeta:     meta,
		PageSettings: pageSettings,
		Source:       "realtime",
	}, nil
}

// saveRealtimeYamlData 保存实时 YAML 数据到独立目录
// 路径: storage/logs/{surveyID}/realtime-yamldata/{pageID}_{YMDHis}.yaml
func saveRealtimeYamlData(basePath, surveyID, pageID, yamlData string) (string, error) {
	cacheDir := filepath.Join(basePath, "logs", surveyID, "realtime-yamldata")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("saveRealtimeYamlData: mkdir %s: %w", cacheDir, err)
	}

	ts := time.Now().Format("20060102150405")
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%s_%s.yaml", pageID, ts))

	if err := os.WriteFile(cachePath, []byte(yamlData), 0644); err != nil {
		return "", fmt.Errorf("saveRealtimeYamlData: write %s: %w", cachePath, err)
	}

	return cachePath, nil
}

// saveRealtimeYamlFailureLog 保存实时 YAML 生成失败的详细日志
func saveRealtimeYamlFailureLog(basePath, surveyID, pageID, systemPrompt, userPrompt, aiResponse string, err error) {
	logDir := filepath.Join(basePath, "logs", "realtime-yamldata-failures")
	if mkErr := os.MkdirAll(logDir, 0755); mkErr != nil {
		fmt.Printf("[警告] saveRealtimeYamlFailureLog: mkdir 失败: %v\n", mkErr)
		return
	}

	ts := time.Now().Format("20060102_150405")
	logPath := filepath.Join(logDir, fmt.Sprintf("%s_%s_%s.log", surveyID, pageID, ts))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("时间: %s\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", surveyID))
	sb.WriteString(fmt.Sprintf("page_id: %s\n", pageID))
	sb.WriteString(fmt.Sprintf("错误: %v\n", err))
	sb.WriteString("\n=== System Prompt ===\n")
	sb.WriteString(systemPrompt)
	sb.WriteString("\n\n=== User Prompt ===\n")
	sb.WriteString(userPrompt)
	if aiResponse != "" {
		sb.WriteString("\n\n=== AI Response ===\n")
		sb.WriteString(aiResponse)
	}

	if writeErr := os.WriteFile(logPath, []byte(sb.String()), 0644); writeErr != nil {
		fmt.Printf("[警告] saveRealtimeYamlFailureLog: write 失败: %v\n", writeErr)
	}
}

// writeRealtimePromptLog 记录实时 YAML Prompt 日志
func writeRealtimePromptLog(basePath, surveyID string, meta PageMetadata, systemPrompt, userPrompt, yamlData string) {
	logDir := filepath.Join(basePath, "logs", surveyID)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("[警告] writeRealtimePromptLog: mkdir 失败: %v\n", err)
		return
	}

	sep := strings.Repeat("=", 80)
	var sb strings.Builder

	sb.WriteString(sep + "\n")
	sb.WriteString("【实时 YAML 数据】Prompt 日志\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("时间: %s\n", meta.Timestamp))
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", surveyID))
	sb.WriteString(fmt.Sprintf("page_id: %s\n", meta.PageID))

	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("=== System Prompt (%d字符) ===\n", len(systemPrompt)))
	sb.WriteString(systemPrompt + "\n")

	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("=== User Prompt (%d字符) ===\n", len(userPrompt)))
	sb.WriteString(userPrompt + "\n")

	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("=== AI 响应 YAML (%d字符) ===\n", len(yamlData)))
	sb.WriteString(yamlData + "\n")

	date := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, fmt.Sprintf("realtime_%s_%s.log", surveyID, date))

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[警告] writeRealtimePromptLog: open 失败: %v\n", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(sb.String()); err != nil {
		fmt.Printf("[警告] writeRealtimePromptLog: write 失败: %v\n", err)
	}
}

// printRealtimeYamlResult 打印实时 YAML 生成结果摘要
func printRealtimeYamlResult(meta PageMetadata, yamlData, cachePath string) {
	fmt.Println()
	fmt.Println("=== 实时 YAML 数据生成完成 ===")
	fmt.Printf("SurveyID: %s\n", meta.SurveyID)
	fmt.Printf("PageID: %s\n", meta.PageID)
	fmt.Printf("Timestamp: %s\n", meta.Timestamp)
	fmt.Printf("YAML长度: %d字符\n", len(yamlData))
	if cachePath != "" {
		fmt.Printf("保存路径: %s\n", cachePath)
	}
	fmt.Printf("来源: realtime\n")
	fmt.Println()
}

// =============================================================================
// 8. 步骤 8: 生成推荐答案（整合到 GenerateRealtimeYamlWithAnswers）
// =============================================================================

// RealtimeYamlWithAnswersResult 步骤 7+8 的合并结果
type RealtimeYamlWithAnswersResult struct {
	YamlData     string          // 实时 YAML 数据
	Answers      []Interaction   // 推荐答案列表
	PageID       string          // 页面 ID
	SurveyID     string          // 问卷 ID
	PageMeta     PageMetadata    // 页面元数据
	PageSettings PageSettings    // 页面设置
	FinalJson    FinalAnswerJson // 最终 JSON 输出
	Source       string          // "realtime"
}

// GenerateRealtimeYamlWithAnswers 生成实时 YAML + 推荐答案（步骤 7 + 步骤 8）
//
//	流程：提取元数据 → 加载配置 → 生成 YAML → 调用 AI → 生成答案 → 合并返回
//
// @param cleanHTML   清洗后的实时 HTML
// @param cachedYaml  缓存的 YAML 模板
// @param surveyID    问卷 ID
// @param basePath    存储根目录
// @returns           RealtimeYamlWithAnswersResult, error
func GenerateRealtimeYamlWithAnswers(cleanHTML, cachedYaml, surveyID, sessionID, basePath string, pageSettings PageSettings) (*RealtimeYamlWithAnswersResult, error) {
	// 步骤 1: 提取页面元数据
	meta, err := extractPageMetadata(surveyID, cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("GenerateRealtimeYamlWithAnswers: extractPageMetadata failed: %w", err)
	}

	// 步骤 2: 加载 Prompt 配置
	cfg, err := LoadYamlStructureConfig()
	if err != nil {
		return nil, fmt.Errorf("GenerateRealtimeYamlWithAnswers: LoadYamlStructureConfig failed: %w", err)
	}

	if cfg.TemplateToYamlSystemPrompt == "" {
		return nil, fmt.Errorf("GenerateRealtimeYamlWithAnswers: TEMPLATE_TO_YAML_SYSTEM_PROMPT is empty")
	}
	if cfg.TemplateToYamlUserPrompt == "" {
		return nil, fmt.Errorf("GenerateRealtimeYamlWithAnswers: TEMPLATE_TO_YAML_USER_PROMPT is empty")
	}

	// 步骤 3: 构建 YAML 生成 Prompt
	systemPrompt := cfg.TemplateToYamlSystemPrompt
	userPrompt := cfg.TemplateToYamlUserPrompt
	userPrompt = strings.Replace(userPrompt, "{yamlTemplate}", cachedYaml, 1)
	userPrompt = strings.Replace(userPrompt, "{html}", cleanHTML, 1)

	// 步骤 3.5: 从 cachedYaml 中提取 page_metadata
	pageMetadataBlock := extractPageMetadataBlock(cachedYaml)

	// 步骤 4: 调用 AI 生成实时 YAML
	aiResponse, err := CallAI(surveyID, systemPrompt, userPrompt, basePath)
	if err != nil {
		saveRealtimeYamlFailureLog(basePath, surveyID, meta.PageID, systemPrompt, userPrompt, "", err)
		return nil, fmt.Errorf("GenerateRealtimeYamlWithAnswers: CallAI (YAML) failed: %w", err)
	}

	// 步骤 5: 去除 markdown 包裹
	yamlData := stripYamlMarkdown(aiResponse)

	// 步骤 5.5: 确保 page_metadata 正确
	yamlData = mergePageMetadata(yamlData, pageMetadataBlock)

	// 步骤 6: 保存实时 YAML
	cachePath, err := saveRealtimeYamlData(basePath, surveyID, meta.PageID, yamlData)
	if err != nil {
		fmt.Printf("[警告] saveRealtimeYamlData 失败: %v\n", err)
		cachePath = ""
	}

	// 步骤 7: 生成推荐答案（使用人设 + 答题上下文）
	fmt.Println("[GenerateRealtimeYamlWithAnswers] 开始生成推荐答案...")
	answerContext := GetPageSummaryContext(sessionID, meta.PageID)
	if answerContext != "" {
		fmt.Printf("[GenerateRealtimeYamlWithAnswers] 获取答题上下文: %d 字符\n", len(answerContext))
	} else {
		fmt.Println("[GenerateRealtimeYamlWithAnswers] 无答题上下文（首次运行或缓存为空）")
	}
	answerResult, err := extractAnswersWithContext(yamlData, surveyID, sessionID, meta.PageID, basePath, answerContext)
	if err != nil {
		return nil, fmt.Errorf("GenerateRealtimeYamlWithAnswers: extractAnswersWithContext failed: %w", err)
	}

	// 步骤 8: 保存 page_summary 到缓存
	if answerResult.PageSummary != "" {
		SavePageSummaryCache(sessionID, meta.PageID, answerResult.PageSummary)
	}

	// 步骤 9: 构建最终 JSON 输出
	finalJson := buildFinalAnswerJson(answerResult, yamlData, pageSettings, surveyID)

	// 步骤 10: 保存 JSON 文件
	jsonPath, err := saveRealtimeAnswersJson(basePath, surveyID, meta.PageID, finalJson)
	if err != nil {
		fmt.Printf("[警告] saveRealtimeAnswersJson 失败: %v\n", err)
		jsonPath = ""
	}

	// 步骤 11: 打印摘要
	printRealtimeYamlWithAnswersResult(meta, yamlData, answerResult.Interactions, cachePath, finalJson, jsonPath)

	return &RealtimeYamlWithAnswersResult{
		YamlData:     yamlData,
		Answers:      answerResult.Interactions,
		PageID:       meta.PageID,
		SurveyID:     surveyID,
		PageMeta:     meta,
		PageSettings: pageSettings,
		FinalJson:    finalJson,
		Source:       "realtime",
	}, nil
}

// printRealtimeYamlWithAnswersResult 打印步骤 7+8 结果摘要
func printRealtimeYamlWithAnswersResult(meta PageMetadata, yamlData string, answers []Interaction, cachePath string, finalJson FinalAnswerJson, jsonPath string) {
	fmt.Println()
	fmt.Println("=== 实时 YAML + 推荐答案生成完成 ===")
	fmt.Printf("SurveyID: %s\n", meta.SurveyID)
	fmt.Printf("PageID: %s\n", meta.PageID)
	fmt.Printf("Timestamp: %s\n", meta.Timestamp)
	fmt.Printf("YAML长度: %d字符\n", len(yamlData))
	fmt.Printf("答案数量: %d\n", len(answers))
	if cachePath != "" {
		fmt.Printf("YAML保存路径: %s\n", cachePath)
	}
	if jsonPath != "" {
		fmt.Printf("JSON保存路径: %s\n", jsonPath)
	}
	fmt.Printf("来源: realtime\n")
	fmt.Println()

	for i, item := range answers {
		fmt.Printf("[%d] %s\n", i+1, item.Subject)
		if arr, ok := item.ShowAnswer.([]interface{}); ok {
			fmt.Printf("    → %v\n", arr)
		} else {
			fmt.Printf("    → %v\n", item.ShowAnswer)
		}
	}
	fmt.Println()

	// 打印 JSON 输出
	// fmt.Println("=== JSON 输出 ===")
	// jsonBytes, _ := json.MarshalIndent(finalJson, "", "  ")
	// fmt.Println(string(jsonBytes))
	// fmt.Println()
}

// extractPageMetadataBlock 从 cachedYaml 中提取 page_metadata 块
//
//	查找 "page_metadata:" 行到 "questions:" 行之间的内容
//
// @param cachedYaml  缓存的 YAML 模板
// @returns           page_metadata 块字符串（包含缩进），如果未找到返回空字符串
func extractPageMetadataBlock(cachedYaml string) string {
	lines := strings.Split(cachedYaml, "\n")

	inMetadata := false
	var result []string
	metadataIndent := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "page_metadata:") {
			inMetadata = true
			metadataIndent = line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			result = append(result, line)
			continue
		}

		if inMetadata {
			if strings.HasPrefix(trimmed, "questions:") || strings.HasPrefix(trimmed, "survey_page:") {
				break
			}

			if line != "" && !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, "  ") {
				break
			}

			if metadataIndent != "" && len(line) > 0 {
				currentIndent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				if len(currentIndent) <= len(metadataIndent) {
					break
				}
			}

			result = append(result, line)
		}
	}

	if len(result) == 0 {
		return ""
	}
	return strings.Join(result, "\n")
}

// removePageMetadataBlock 从 yamlData 中删除 page_metadata 块（如果存在）
// @param yamlData  AI 生成的 YAML 字符串
// @returns         删除 page_metadata 后的 YAML 字符串
func removePageMetadataBlock(yamlData string) string {
	lines := strings.Split(yamlData, "\n")
	var result []string
	inMetadata := false
	metadataIndent := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "page_metadata:") {
			inMetadata = true
			metadataIndent = line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			continue
		}

		if inMetadata {
			if strings.HasPrefix(trimmed, "questions:") || strings.HasPrefix(trimmed, "survey_page:") {
				inMetadata = false
				result = append(result, line)
				continue
			}

			if line != "" {
				currentIndent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				if len(currentIndent) <= len(metadataIndent) && !strings.HasPrefix(trimmed, "has_") && !strings.HasPrefix(trimmed, "is_") {
					inMetadata = false
					result = append(result, line)
					continue
				}
			}
			continue
		}

		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// mergePageMetadata 将 page_metadata 插入到 yamlData 中
//
//	逻辑：找到 title 行，在其后插入 page_metadata
//
// @param yamlData         AI 生成的 YAML 字符串
// @param pageMetadata      page_metadata 块字符串
// @returns                合并后的 YAML 字符串
func mergePageMetadata(yamlData, pageMetadata string) string {
	if pageMetadata == "" {
		return yamlData
	}

	yamlData = removePageMetadataBlock(yamlData)

	lines := strings.Split(yamlData, "\n")
	var result []string
	titleInserted := false

	for _, line := range lines {
		result = append(result, line)

		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "title:") && !titleInserted {
			metadataLines := strings.Split(pageMetadata, "\n")
			for i, metaLine := range metadataLines {
				if i == 0 {
					result = append(result, "  "+strings.TrimSpace(metaLine))
				} else {
					result = append(result, metaLine)
				}
			}
			titleInserted = true
		}
	}

	return strings.Join(result, "\n")
}

// parseYamlPageSettings 从 yamlData 中解析 page_settings
// @param yamlData  YAML 字符串
// @returns         PageSettings
func parseYamlPageSettings(yamlData string) PageSettings {
	settings := PageSettings{}

	lines := strings.Split(yamlData, "\n")
	inMetadata := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "page_metadata:") {
			inMetadata = true
			continue
		}

		if inMetadata {
			if strings.HasPrefix(trimmed, "questions:") || strings.HasPrefix(trimmed, "survey_page:") {
				break
			}

			if strings.Contains(trimmed, "has_next_button:") {
				settings.HasNextButton = parseYamlBool(trimmed)
			} else if strings.Contains(trimmed, "has_submit_button:") {
				settings.HasSubmitButton = parseYamlBool(trimmed)
			} else if strings.Contains(trimmed, "is_thank_you_page:") {
				settings.IsThankYouPage = parseYamlBool(trimmed)
			} else if strings.Contains(trimmed, "is_description_page:") {
				settings.IsDescriptionPage = parseYamlBool(trimmed)
			}
		}
	}

	return settings
}

// parseYamlBool 解析 YAML 布尔值
func parseYamlBool(line string) bool {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return false
	}
	val := strings.TrimSpace(parts[1])
	return val == "true" || val == "yes" || val == "1"
}

// buildFinalAnswerJson 构建最终 JSON 输出
// @param answerResult  AI 生成的结果
// @param yamlData     YAML 数据
// @param pageSettings 页面设置
// @param surveyID    问卷 ID
// @returns            FinalAnswerJson
func buildFinalAnswerJson(answerResult *YamlAnswerResult, yamlData string, pageSettings PageSettings, surveyID string) FinalAnswerJson {
	pageID := parseYamlPageID(yamlData)
	isMatrixPage := detectIsMatrixPage(yamlData)
	interactions := buildInteractions(answerResult.Interactions)
	navButtons := buildNavButtons(pageSettings, surveyID)

	return FinalAnswerJson{
		Data: DataOutput{
			PageMetadata: PageMetadataOutput{
				PageID:          pageID,
				IsMatrixPage:    isMatrixPage,
				HasNextButton:   pageSettings.HasNextButton,
				HasSubmitButton: pageSettings.HasSubmitButton,
				IsThankYouPage:  pageSettings.IsThankYouPage,
			},
			Interactions: interactions,
			NavButtons:   navButtons,
			PageSummary:  answerResult.PageSummary, // AI 回复
		},
	}
}

// parseYamlPageID 从 yamlData 中提取 page_id（fingerprint）
func parseYamlPageID(yamlData string) string {
	lines := strings.Split(yamlData, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "fingerprint:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// detectIsMatrixPage 判断是否矩阵页
func detectIsMatrixPage(yamlData string) bool {
	lines := strings.Split(yamlData, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "matrix_radio") || strings.Contains(trimmed, "matrix_checkbox") {
			return true
		}
	}
	return false
}

// buildInteractions 构建 interactions
func buildInteractions(answers []Interaction) []InteractionItem {
	result := make([]InteractionItem, 0, len(answers))

	for _, ans := range answers {
		item := InteractionItem{
			QuestionType: ans.QuestionType, // AI 回复
			Subject:      ans.Subject,
		}

		switch v := ans.ShowAnswer.(type) {
		case string:
			item.RecommendedAnswers = v
		case []interface{}:
			strs := make([]string, 0, len(v))
			for _, val := range v {
				if s, ok := val.(string); ok {
					strs = append(strs, s)
				}
			}
			if len(strs) == 1 {
				item.RecommendedAnswers = strs[0]
			} else {
				item.RecommendedAnswers = strs
			}
		default:
			item.RecommendedAnswers = nil
		}

		result = append(result, item)
	}

	return result
}

// NextButtonConfig Next/Submit 按钮配置
type NextButtonConfig struct {
	NextPattern    string `yaml:"next_pattern"`
	WaitAfterClick int    `yaml:"wait_after_click"`
}

// SiteConfig 网站配置
type SiteConfig struct {
	Keyword    string           `yaml:"keyword"`
	SurveyID   string           `yaml:"survey_id"`
	NextButton NextButtonConfig `yaml:"next_button"`
}

// PageFingerprintsConfig page-fingerprints.yaml 配置结构
type PageFingerprintsConfig struct {
	Sites []SiteConfig `yaml:"sites"`
}

const pageFingerprintsConfigPath = "configs/page-fingerprints.yaml"

// loadNextButtonConfig 根据 surveyID 从 page-fingerprints.yaml 加载 next_button 配置
func loadNextButtonConfig(surveyID string) (NextButtonConfig, error) {
	// 读取配置文件
	configPath := pageFingerprintsConfigPath
	if !filepath.IsAbs(configPath) {
		wd, err := os.Getwd()
		if err == nil {
			configPath = filepath.Join(wd, configPath)
		}
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return NextButtonConfig{}, fmt.Errorf("loadNextButtonConfig: read %s: %w", configPath, err)
	}

	var cfg PageFingerprintsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return NextButtonConfig{}, fmt.Errorf("loadNextButtonConfig: parse yaml: %w", err)
	}

	// 匹配 survey_id
	for _, site := range cfg.Sites {
		if site.SurveyID == surveyID {
			return site.NextButton, nil
		}
	}

	// 未找到匹配，返回默认配置
	return NextButtonConfig{}, nil
}

// buildNavButtons 构建 nav_buttons
func buildNavButtons(pageSettings PageSettings, surveyID string) NavButtonsOutput {
	var buttons []NavButtonItem
	primaryButton := ""

	// 从配置文件加载 next_button 配置
	nextButtonCfg, _ := loadNextButtonConfig(surveyID)

	if pageSettings.HasNextButton {
		buttons = append(buttons, NavButtonItem{
			Role:           "next",
			XPath:          nextButtonCfg.NextPattern,
			Text:           "Next",
			WaitAfterClick: nextButtonCfg.WaitAfterClick,
			Attributes:     map[string]string{},
		})
		primaryButton = "next"
	}

	if pageSettings.HasSubmitButton {
		buttons = append(buttons, NavButtonItem{
			Role:           "submit",
			XPath:          "",
			Text:           "Submit",
			WaitAfterClick: 0,
			Attributes:     map[string]string{},
		})
		primaryButton = "submit"
	}

	return NavButtonsOutput{
		Buttons:       buttons,
		PrimaryButton: primaryButton,
	}
}

// saveRealtimeAnswersJson 保存答案 JSON 到文件
// 路径: storage/logs/{surveyID}/realtime-answers/{pageID}_{timestamp}.json
func saveRealtimeAnswersJson(basePath, surveyID, pageID string, result FinalAnswerJson) (string, error) {
	cacheDir := filepath.Join(basePath, "logs", surveyID, "realtime-answers")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("saveRealtimeAnswersJson: mkdir %s: %w", cacheDir, err)
	}

	ts := time.Now().Format("20060102150405")
	fileName := fmt.Sprintf("%s_%s.json", sanitizeFileName(pageID), ts)
	filePath := filepath.Join(cacheDir, fileName)

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("saveRealtimeAnswersJson: marshal json: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("saveRealtimeAnswersJson: write file: %w", err)
	}

	return filePath, nil
}
