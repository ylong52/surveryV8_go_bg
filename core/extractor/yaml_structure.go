package extractor

// yaml_structure.go — YAML 结构化提示词配置与构建
//   从 configs/yaml_structure_prompt.yaml 加载提示词配置
//   提供独立的 Prompt 构建函数，供外部调用

import (
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
	SystemPrompt      string `yaml:"STRUCTURE_SYSTEM_PROMPT"`
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
//   步骤1: 参数验证
//   步骤2: 提取 PageMetadata
//   步骤3: 组装 AI Prompt
//   步骤4: 查找本地 YAML 模板
//   步骤5: 未命中则调用 AI
//   步骤6: 保存 YAML 到缓存目录
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
	YamlData  string       // 最新YAML数据
	CachePath string       // 缓存路径
	PageID    string
	SurveyID  string
	PageMeta  PageMetadata
	Source    string // 来源标记: "realtime"=实时生成, "template"=模板转换
}

// GenerateLatestYamlData 根据模板 + 实时HTML 生成最新YAML数据
//   流程：读取YAML模板 → 构建Prompt → 调用AI → 保存到storage/logs
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
	YamlData   string       // 实时的最新 YAML 数据
	CachePath  string       // 保存路径
	PageID     string       // 页面 ID
	SurveyID   string       // 问卷 ID
	PageMeta   PageMetadata // 页面元数据
	Source     string       // "realtime" = 实时生成
}

// GenerateRealtimeYamlData 根据缓存的 YAML 模板 + 实时 HTML 生成最新 YAML 数据
//   仅当 YamlStructureResult.Source == "cache" 时调用
//   流程：提取元数据 → 加载配置 → 构建Prompt → 调用AI → 保存到 realtime-yamldata/
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
		YamlData:  yamlData,
		CachePath: cachePath,
		PageID:    meta.PageID,
		SurveyID:  surveyID,
		PageMeta:  meta,
		Source:    "realtime",
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
	YamlData  string           // 实时 YAML 数据
	Answers   []Interaction    // 推荐答案列表
	PageID    string           // 页面 ID
	SurveyID  string           // 问卷 ID
	PageMeta  PageMetadata     // 页面元数据
	Source    string           // "realtime"
}

// GenerateRealtimeYamlWithAnswers 生成实时 YAML + 推荐答案（步骤 7 + 步骤 8）
//   流程：提取元数据 → 加载配置 → 生成 YAML → 调用 AI → 生成答案 → 合并返回
// @param cleanHTML   清洗后的实时 HTML
// @param cachedYaml  缓存的 YAML 模板
// @param surveyID    问卷 ID
// @param basePath    存储根目录
// @returns           RealtimeYamlWithAnswersResult, error
func GenerateRealtimeYamlWithAnswers(cleanHTML, cachedYaml, surveyID, basePath string) (*RealtimeYamlWithAnswersResult, error) {
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

	// 步骤 4: 调用 AI 生成实时 YAML
	aiResponse, err := CallAI(surveyID, systemPrompt, userPrompt, basePath)
	if err != nil {
		saveRealtimeYamlFailureLog(basePath, surveyID, meta.PageID, systemPrompt, userPrompt, "", err)
		return nil, fmt.Errorf("GenerateRealtimeYamlWithAnswers: CallAI (YAML) failed: %w", err)
	}

	// 步骤 5: 去除 markdown 包裹
	yamlData := stripYamlMarkdown(aiResponse)

	// 步骤 6: 保存实时 YAML
	cachePath, err := saveRealtimeYamlData(basePath, surveyID, meta.PageID, yamlData)
	if err != nil {
		fmt.Printf("[警告] saveRealtimeYamlData 失败: %v\n", err)
		cachePath = ""
	}

	// 步骤 7: 生成推荐答案
	answerResult, err := ExtractYamlAnswers(yamlData, surveyID, meta.PageID, basePath)
	if err != nil {
		return nil, fmt.Errorf("GenerateRealtimeYamlWithAnswers: ExtractYamlAnswers failed: %w", err)
	}

	// 步骤 8: 打印摘要
	printRealtimeYamlWithAnswersResult(meta, yamlData, answerResult.Interactions, cachePath)

	return &RealtimeYamlWithAnswersResult{
		YamlData:  yamlData,
		Answers:   answerResult.Interactions,
		PageID:    meta.PageID,
		SurveyID:  surveyID,
		PageMeta:  meta,
		Source:    "realtime",
	}, nil
}

// printRealtimeYamlWithAnswersResult 打印步骤 7+8 结果摘要
func printRealtimeYamlWithAnswersResult(meta PageMetadata, yamlData string, answers []Interaction, cachePath string) {
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
}
