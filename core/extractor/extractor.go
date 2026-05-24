package extractor

// extractor.go — 步骤 1-3 入口
//   接收参数 → 提取 PageMetadata → 组装 AI Prompt → 写日志

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExtractResult 包含步骤 2、3、4 的输出
type ExtractResult struct {
	Meta       PageMetadata
	Prompt     AssembledPrompt // 步骤3：答题用AI Prompt
	YamlPrompt YamlPrompt      // 步骤4：YAML提取专用提示词
}

// ExtractWithPageID 执行步骤 1-4，但 page_id 已知（从原始 HTML 提取）
// 用于 page_id 提取需要保留 class 属性的场景
// @param surveyID  问卷标识
// @param pageID    已知的 page_id（从原始 HTML 提取）
// @param cleanHTML 清洗后的 HTML 字符串
// @param basePath  存储根目录
// @returns         ExtractResult，或错误
func ExtractWithPageID(surveyID, pageID, cleanHTML, basePath string) (*ExtractResult, error) {
	if surveyID == "" {
		return nil, fmt.Errorf("ExtractWithPageID: surveyID cannot be empty")
	}
	if pageID == "" {
		return nil, fmt.Errorf("ExtractWithPageID: pageID cannot be empty")
	}
	if cleanHTML == "" {
		return nil, fmt.Errorf("ExtractWithPageID: cleanHTML cannot be empty")
	}
	if basePath == "" {
		basePath = "./storage"
	}

	// 使用已知的 pageID 构建 PageMetadata
	meta := PageMetadata{
		SurveyID:       surveyID,
		PageID:         pageID,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		QuestionsCount: 0,
	}

	// 步骤 3：组装 AI Prompt（答题用）
	prompt, err := buildPrompt(surveyID, meta.PageID, cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("ExtractWithPageID step3: %w", err)
	}

	// 写步骤1-3的日志
	if err := writeLog(basePath, surveyID, meta, prompt, len(cleanHTML)); err != nil {
		return nil, fmt.Errorf("ExtractWithPageID writeLog: %w", err)
	}

	// 步骤 4：整理 YAML 专用提示词
	yamlPrompt, err := BuildYamlPrompt(surveyID, meta.PageID, cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("ExtractWithPageID step4: %w", err)
	}

	// 写步骤4的YAML提示词日志（完整保存，不截断）
	if err := WriteYamlPromptLog(basePath, surveyID, meta, yamlPrompt, cleanHTML); err != nil {
		return nil, fmt.Errorf("ExtractWithPageID writeYamlPromptLog: %w", err)
	}

	// 控制台输出YAML提示词摘要（HTML截断300字符）
	printYamlPromptSummary(surveyID, meta, yamlPrompt, cleanHTML)

	return &ExtractResult{Meta: meta, Prompt: prompt, YamlPrompt: yamlPrompt}, nil
}

// Extract 执行步骤 1-4：接收参数、提取 PageMetadata、组装 AI Prompt、整理 YAML Prompt，并写日志
// @param surveyID  问卷标识（不能为空）
// @param cleanHTML 清洗后的 HTML 字符串（不能为空，超过60000字符直接报错）
// @param basePath  存储根目录（空字符串时默认 "./storage"）
// @returns         ExtractResult（含步骤2、3、4的输出），或错误
func Extract(surveyID, cleanHTML, basePath string) (*ExtractResult, error) {
	// 步骤 1：验证参数
	if surveyID == "" {
		return nil, fmt.Errorf("Extract: surveyID cannot be empty")
	}
	if cleanHTML == "" {
		return nil, fmt.Errorf("Extract: cleanHTML cannot be empty")
	}
	if basePath == "" {
		basePath = "./storage"
	}

	// 步骤 2：提取 PageMetadata
	meta, err := extractPageMetadata(surveyID, cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("Extract step2: %w", err)
	}

	// 步骤 3：组装 AI Prompt（答题用）
	prompt, err := buildPrompt(surveyID, meta.PageID, cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("Extract step3: %w", err)
	}

	// 写步骤1-3的日志
	if err := writeLog(basePath, surveyID, meta, prompt, len(cleanHTML)); err != nil {
		return nil, fmt.Errorf("Extract writeLog: %w", err)
	}

	// 步骤 4：整理 YAML 专用提示词
	yamlPrompt, err := BuildYamlPrompt(surveyID, meta.PageID, cleanHTML)
	if err != nil {
		return nil, fmt.Errorf("Extract step4: %w", err)
	}

	// 写步骤4的YAML提示词日志（完整保存，不截断）
	if err := WriteYamlPromptLog(basePath, surveyID, meta, yamlPrompt, cleanHTML); err != nil {
		return nil, fmt.Errorf("Extract writeYamlPromptLog: %w", err)
	}

	// 控制台输出YAML提示词摘要（HTML截断300字符）
	printYamlPromptSummary(surveyID, meta, yamlPrompt, cleanHTML)

	return &ExtractResult{Meta: meta, Prompt: prompt, YamlPrompt: yamlPrompt}, nil
}

// writeLog 将步骤 1-3 的关键信息写入日志文件
// @param basePath  存储根目录
// @param surveyID  问卷标识
// @param meta      步骤 2 输出的 PageMetadata
// @param prompt    步骤 3 输出的 AssembledPrompt
// @param htmlLen   cleanHTML 的字节数
func writeLog(basePath, surveyID string, meta PageMetadata, prompt AssembledPrompt, htmlLen int) error {
	logDir := filepath.Join(basePath, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("writeLog: mkdir %s: %w", logDir, err)
	}

	date := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", surveyID, date))

	sep := strings.Repeat("=", 80)
	var sb strings.Builder

	sb.WriteString(sep + "\n\n\n\n")
	sb.WriteString(sep + ">>>>>>>>>>>>>>>>>>>>>>>>> 开始执行 Extract 流程 >>>>>>>>>>>>>>>>>>>>>>>>>\n")
	sb.WriteString("【步骤 1】接收参数\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("时间: %s\n", meta.Timestamp))
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", surveyID))
	sb.WriteString(fmt.Sprintf("cleanHTML 长度: %d bytes\n", htmlLen))

	sb.WriteString(sep + "\n")
	sb.WriteString("【步骤 2】提取 PageMetadata\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", meta.SurveyID))
	sb.WriteString(fmt.Sprintf("page_id: %s\n", meta.PageID))
	sb.WriteString(fmt.Sprintf("timestamp: %s\n", meta.Timestamp))
	sb.WriteString(fmt.Sprintf("questions_count: %d\n", meta.QuestionsCount))

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("writeLog: open %s: %w", logPath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(sb.String()); err != nil {
		return fmt.Errorf("writeLog: write %s: %w", logPath, err)
	}
	return nil
}

// printYamlPromptSummary 控制台输出YAML提示词摘要（HTML截断300字符）
func printYamlPromptSummary(surveyID string, meta PageMetadata, yamlPrompt YamlPrompt, cleanHTML string) {
	fmt.Println()
	fmt.Println("=== YAML Prompt 已整理 ===")
	fmt.Printf("SurveyID: %s\n", surveyID)
	fmt.Printf("PageID: %s\n", meta.PageID)
	fmt.Printf("Timestamp: %s\n", meta.Timestamp)
	fmt.Printf("System Prompt: %d字符\n", len(yamlPrompt.System))
	fmt.Printf("User Prompt: %d字符\n", len(yamlPrompt.User))
	fmt.Printf("HTML: %d字符\n", len(cleanHTML))
	fmt.Println()

	fmt.Println("=== System Prompt (完整) ===")
	fmt.Println(yamlPrompt.System)
	fmt.Println()

	fmt.Println("=== User Prompt (完整) ===")
	fmt.Println(yamlPrompt.User)
	fmt.Println()

	fmt.Println("=== HTML预览 (前300字符) ===")
	if len(cleanHTML) <= 300 {
		fmt.Println(cleanHTML)
	} else {
		fmt.Printf("%s...\n", cleanHTML[:300])
	}
}
