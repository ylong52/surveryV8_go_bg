package extractor

// yaml_extractor.go — 步骤 1-5：整合 Extract + CallAI + 缓存

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// YamlResult 包含步骤 1-5 的完整结果
type YamlResult struct {
	YamlContent string       // YAML内容（已去除markdown包裹）
	CachePath   string       // 缓存路径：storage/templates/{surveyId}/{pageID}.yaml
	Meta        PageMetadata // 页面元数据
	YamlPrompt  YamlPrompt  // 提示词（用于日志）
	Source      string       // 来源："cache"=命中本地模板，"ai"=AI生成
}

// ExtractYamlStructure 执行步骤 1-5
//   步骤1: 参数验证
//   步骤2: 提取 PageMetadata
//   步骤3: 组装 AI Prompt
//   步骤4: 整理 YAML 专用提示词
//   步骤5: 调用 AI 提取 YAML 结构
//   步骤6: 保存 YAML 到缓存目录
// @param surveyID  问卷ID
// @param cleanHTML 清洗后HTML
// @param basePath  存储根目录
// @returns         YamlResult，或错误（错误直接返回，程序中断）
func ExtractYamlStructure(surveyID, cleanHTML, basePath string) (*YamlResult, error) {
	// 步骤 1-4：调用 Extract() 获取 YamlPrompt
	result, err := Extract(surveyID, cleanHTML, basePath)
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlStructure: Extract failed: %w", err)
	}
	yamlPrompt := result.YamlPrompt
	meta := result.Meta

	// 查找本地 YAML 模板
	templatePath := filepath.Join(basePath, "templates", surveyID, meta.PageID+".yaml")

	if yamlContent, err := os.ReadFile(templatePath); err == nil {
		// 命中本地模板，跳过 AI 调用
		writeCacheHitLog(basePath, surveyID, meta, templatePath)
		printCacheHitSummary(meta, templatePath, string(yamlContent))

		return &YamlResult{
			YamlContent: string(yamlContent),
			CachePath:   templatePath,
			Meta:        meta,
			YamlPrompt:  yamlPrompt,
			Source:      "cache",
		}, nil
	}

	// 未命中模板，继续调用 AI
	writeCacheMissLog(basePath, surveyID, meta, templatePath)

	// 写步骤5开始日志
	writeStep5StartLog(basePath, surveyID, meta, yamlPrompt)

	// 步骤 5：调用 AI 提取 YAML 结构
	aiResponse, err := CallAI(
		surveyID,
		yamlPrompt.System, // 系统提示词
		yamlPrompt.User,   // 用户提示词（含HTML）
		basePath,
	)
	if err != nil {
		// 直接返回错误，程序中断
		return nil, fmt.Errorf("ExtractYamlStructure: CallAI failed: %w", err)
	}

	// 写步骤5完成日志
	writeStep5CompleteLog(basePath, surveyID, meta, aiResponse)

	// 步骤 6：保存 YAML 到缓存目录
	cachePath, err := saveYamlToCache(basePath, surveyID, meta.PageID, aiResponse)
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlStructure: saveYamlToCache failed: %w", err)
	}

	// 控制台输出摘要
	printYamlResultSummary(meta, cachePath, aiResponse)

	return &YamlResult{
		YamlContent: aiResponse,
		CachePath:   cachePath,
		Meta:       meta,
		YamlPrompt: yamlPrompt,
		Source:     "ai",
	}, nil
}

// writeStep5StartLog 写入步骤5开始日志
func writeStep5StartLog(basePath, surveyID string, meta PageMetadata, yamlPrompt YamlPrompt) {
	logDir := filepath.Join(basePath, "logs")
	_ = os.MkdirAll(logDir, 0755)

	date := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", surveyID, date))

	sep := strings.Repeat("=", 80)
	var sb strings.Builder

	sb.WriteString(sep + "\n")
	sb.WriteString("【步骤 5】调用 AI 提取 YAML 结构\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("时间: %s\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", surveyID))
	sb.WriteString(fmt.Sprintf("page_id: %s\n", meta.PageID))
	sb.WriteString(fmt.Sprintf("System Prompt: %d字符\n", len(yamlPrompt.System)))
	sb.WriteString(fmt.Sprintf("User Prompt: %d字符\n", len(yamlPrompt.User)))
	sb.WriteString("状态: 开始调用 AI...\n")
	sb.WriteString(sep + "\n")

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(sb.String())
}

// writeStep5CompleteLog 写入步骤5完成日志（包含AI响应）
func writeStep5CompleteLog(basePath, surveyID string, meta PageMetadata, aiResponse string) {
	logDir := filepath.Join(basePath, "logs")
	_ = os.MkdirAll(logDir, 0755)

	date := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", surveyID, date))

	sep := strings.Repeat("=", 80)
	var sb strings.Builder

	sb.WriteString(sep + "\n")
	sb.WriteString("【步骤 5】AI 响应\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("时间: %s\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", surveyID))
	sb.WriteString(fmt.Sprintf("page_id: %s\n", meta.PageID))
	sb.WriteString(fmt.Sprintf("YAML 长度: %d字符\n", len(aiResponse)))
	sb.WriteString("\n" + sep + "\n")
	sb.WriteString("=== AI 响应 (完整YAML) ===\n")
	sb.WriteString(aiResponse + "\n")
	sb.WriteString(sep + "\n")
	sb.WriteString("状态: 成功\n")
	sb.WriteString(sep + "\n")

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(sb.String())
}

// Windows 文件系统非法字符列表
var illegalFileNameChars = []string{"*", "\"", "<", ">", "|", "?", ":", "\\", "/"}

// sanitizeFileName 清理文件名中的非法字符，并截断过长名称
// @param name 原始文件名（pageID）
// @returns 清理后的安全文件名
func sanitizeFileName(name string) string {
	result := name

	// 替换所有非法字符为空（移除）
	for _, char := range illegalFileNameChars {
		result = strings.ReplaceAll(result, char, "")
	}

	// 移除前后空白
	result = strings.TrimSpace(result)

	// 连续下划线替换为单个下划线
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}

	// 移除首尾下划线
	result = strings.Trim(result, "_")

	// 截断到最大长度（Windows 路径限制）
	const maxLen = 200
	if len(result) > maxLen {
		result = result[:maxLen]
	}

	return result
}

// saveYamlToCache 保存YAML到缓存目录
// @param basePath  存储根目录
// @param surveyID  问卷ID
// @param pageID    页面ID
// @param content   YAML内容
// @returns         缓存路径，或错误
func saveYamlToCache(basePath, surveyID, pageID, content string) (string, error) {
	// 清理文件名中的非法字符
	safePageID := sanitizeFileName(pageID)

	// 构建缓存路径: storage/templates/{surveyId}/{safePageID}.yaml
	cacheDir := filepath.Join(basePath, "templates", surveyID)
	cachePath := filepath.Join(cacheDir, safePageID+".yaml")

	// 创建目录（如果不存在）
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("saveYamlToCache: mkdir %s: %w", cacheDir, err)
	}

	// 写入文件
	if err := os.WriteFile(cachePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("saveYamlToCache: write %s: %w", cachePath, err)
	}

	return cachePath, nil
}

// printYamlResultSummary 控制台输出YAML结果摘要
func printYamlResultSummary(meta PageMetadata, cachePath, yamlContent string) {
	fmt.Println()
	fmt.Println("=== YAML 提取完成 ===")
	fmt.Printf("SurveyID: %s\n", meta.SurveyID)
	fmt.Printf("PageID: %s\n", meta.PageID)
	fmt.Printf("Timestamp: %s\n", meta.Timestamp)
	fmt.Printf("YAML长度: %d字符\n", len(yamlContent))
	fmt.Printf("缓存路径: %s\n", cachePath)
	fmt.Printf("来源: ai\n")
	fmt.Println()
}

// writeCacheHitLog 写入命中本地模板的日志
func writeCacheHitLog(basePath, surveyID string, meta PageMetadata, templatePath string) {
	logDir := filepath.Join(basePath, "logs")
	_ = os.MkdirAll(logDir, 0755)

	date := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", surveyID, date))

	sep := strings.Repeat("=", 80)
	var sb strings.Builder

	sb.WriteString(sep + "\n")
	sb.WriteString("【YAML模板查找】命中本地模板，跳过AI调用\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("时间: %s\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", surveyID))
	sb.WriteString(fmt.Sprintf("page_id: %s\n", meta.PageID))
	sb.WriteString(fmt.Sprintf("模板路径: %s\n", templatePath))
	sb.WriteString("来源: cache\n")
	sb.WriteString("状态: 命中本地模板，跳过AI调用\n")
	sb.WriteString(sep + "\n")

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(sb.String())
}

// writeCacheMissLog 写入未命中本地模板的日志
func writeCacheMissLog(basePath, surveyID string, meta PageMetadata, templatePath string) {
	logDir := filepath.Join(basePath, "logs")
	_ = os.MkdirAll(logDir, 0755)

	date := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", surveyID, date))

	sep := strings.Repeat("=", 80)
	var sb strings.Builder

	sb.WriteString(sep + "\n")
	sb.WriteString("【YAML模板查找】未命中本地模板，继续调用AI\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("时间: %s\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", surveyID))
	sb.WriteString(fmt.Sprintf("page_id: %s\n", meta.PageID))
	sb.WriteString(fmt.Sprintf("查找路径: %s\n", templatePath))
	sb.WriteString(fmt.Sprintf("!!! page_id [%s] 未命中YAML模板目录的文件!!!\n", meta.PageID))
	sb.WriteString("来源: ai\n")
	sb.WriteString("状态: 未命中，继续调用AI\n")
	sb.WriteString(sep + "\n")

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(sb.String())
}

// printCacheHitSummary 控制台输出命中缓存的摘要
func printCacheHitSummary(meta PageMetadata, templatePath string, yamlContent string) {
	fmt.Println()
	fmt.Println("=== YAML 缓存命中 ===")
	fmt.Printf("SurveyID: %s\n", meta.SurveyID)
	fmt.Printf("PageID: %s\n", meta.PageID)
	fmt.Printf("Timestamp: %s\n", meta.Timestamp)
	fmt.Printf("模板路径: %s\n", templatePath)
	fmt.Printf("YAML长度: %d字符\n", len(yamlContent))
	fmt.Printf("来源: cache\n")
	fmt.Println()
	fmt.Println("=== YAML 内容预览 ===")
	if len(yamlContent) <= 300 {
		fmt.Println(yamlContent)
	} else {
		fmt.Printf("%s...\n", yamlContent[:300])
	}
	fmt.Println()
}
