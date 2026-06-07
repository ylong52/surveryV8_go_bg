package extractor

// yaml_extractor.go — 步骤 3: YAML 模板提取（含题型检测）
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                        数据源架构                                 │
//	├─────────────────────────────────────────────────────────────────┤
//	│  page_results.yaml_template  ← 首次生成后缓存（优先命中）          │
//	│  prompts 表 yaml_template   ← 缓存未命中时加载的标准模板           │
//	└─────────────────────────────────────────────────────────────────┘
//
//	流程：
//	  1. 查询 page_results 表 → 命中则返回 cachedYaml（并解析题型）
//	  2. 未命中 → AI 检测题型 → 调用 GetFixedTemplate 加载模板 → 合并返回
//
//	依赖：
//	  - yaml_templates.go  → GetFixedTemplate 从数据库 prompts 表加载模板
//	  - yaml_structure.go  → parseQuestionsTypeFromYaml 解析题型
//
import (
	"encoding/json"
	"fmt"
	"strings"

	"survey.cli/core/database"
)

// YamlResult 包含步骤 3 的完整结果
type YamlResult struct {
	YamlContent   string       // YAML 占位符模板内容
	Meta          PageMetadata // 页面元数据
	DetectedTypes []string     // 检测到的题型列表（cache miss 时填充）
	Source        string       // "db_cache"=命中数据库，"ai"=AI检测生成
}

// ExtractYamlStructureWithPageID 执行步骤 3：查询或生成 YAML 占位符模板
//
//	命中缓存直接返回；未命中时调用 AI 检测题型后加载并合并模板
//
// @param surveyID     问卷 ID
// @param surveyIDInt  问卷 ID（整型，用于数据库查询）
// @param pageID       已知的 page_id
// @param cleanHTML    清洗后 HTML
// @param basePath     存储根目录
// @returns            YamlResult，或错误
func ExtractYamlStructureWithPageID(surveyID string, surveyIDInt int64, pageID, cleanHTML, basePath string) (*YamlResult, error) {
	meta := PageMetadata{
		SurveyID: surveyID,
		PageID:   pageID,
	}

	// 步骤 1：查询数据库 yaml_template 缓存（忽略 fingerprint，同一 survey+page 模板相同）
	pageResultRepo := database.NewPageResultRepo()
	cachedYaml, err := pageResultRepo.FindYamlTemplate(surveyIDInt, pageID)
	if err != nil {
		fmt.Printf("[ExtractYamlStructureWithPageID] 查询 yaml_template 失败: %v\n", err)
	}

	if cachedYaml != "" {
		fmt.Printf("[ExtractYamlStructureWithPageID] 命中数据库 yaml_template\n")
		printYamlResultSummary(meta, cachedYaml, "db_cache")
		// 从缓存的 YAML 模板中解析题型，确保 DetectedTypes 不为空
		detectedTypes := parseQuestionsTypeFromYaml(cachedYaml)
		if len(detectedTypes) == 0 {
			detectedTypes = []string{"mixed"}
		}
		return &YamlResult{
			YamlContent:   cachedYaml,
			Meta:          meta,
			DetectedTypes: detectedTypes,
			Source:        "db_cache",
		}, nil
	}

	// 步骤 2：加载有效题型列表和检测提示词
	fmt.Printf("[ExtractYamlStructureWithPageID] 未命中 yaml_template，执行题型检测\n")

	questionTypes, err := database.LoadQuestionTypes()
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlStructureWithPageID: LoadQuestionTypes: %w", err)
	}
	if len(questionTypes) == 0 {
		questionTypes = []string{"mixed"}
	}

	typeDetectorPrompt, err := database.LoadTypeDetectorPrompt()
	if err != nil {
		return nil, fmt.Errorf("ExtractYamlStructureWithPageID: LoadTypeDetectorPrompt: %w", err)
	}

	// 步骤 3a：HTML 特征预检测（优先于 AI，避免 cardsort 被误判为 matrix_radio）
	detectedTypes := detectByHTMLSignature(cleanHTML)
	if len(detectedTypes) > 0 {
		fmt.Printf("[题型检测] HTML特征命中题型: %v（跳过AI）\n", detectedTypes)
	} else {
		// 步骤 3b：AI 检测题型
		var err error
		detectedTypes, err = detectQuestionTypesWithAI(surveyID, cleanHTML, basePath, typeDetectorPrompt, questionTypes)
		if err != nil {
			fmt.Printf("[警告] detectQuestionTypesWithAI 失败: %v，降级到 mixed\n", err)
			detectedTypes = []string{"mixed"}
		}
		if len(detectedTypes) == 0 {
			detectedTypes = []string{"mixed"}
		}
	}
	fmt.Printf("[题型检测] 检测到题型: %v\n", detectedTypes)

	// 步骤 4：按题型加载 yaml_template 并合并
	yamlContent, err := buildMergedYamlTemplate(detectedTypes)
	if err != nil {
		fmt.Printf("[警告] buildMergedYamlTemplate 失败: %v，使用 mixed 兜底模板\n", err)
		yamlContent = GetFixedTemplate("mixed")
	}

	printYamlResultSummary(meta, yamlContent, "ai")

	return &YamlResult{
		YamlContent:   yamlContent,
		Meta:          meta,
		DetectedTypes: detectedTypes,
		Source:        "ai",
	}, nil
}

// detectQuestionTypesWithAI 调用 AI 检测页面题型，返回 JSON 数组
//
// @param surveyID            问卷 ID
// @param cleanHTML           清洗后 HTML
// @param basePath            存储根目录
// @param typeDetectorPrompt  system.type_detector 内容（含 {question_types} 占位符）
// @param questionTypes       有效题型列表
// @returns                   检测到的题型列表，error
func detectQuestionTypesWithAI(surveyID, cleanHTML, basePath, typeDetectorPrompt string, questionTypes []string) ([]string, error) {
	typeList := strings.Join(questionTypes, ", ")
	systemPrompt := strings.ReplaceAll(typeDetectorPrompt, "{question_types}", typeList)
	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = fmt.Sprintf(
			"你是题型识别助手。分析HTML中的表单题目，返回题型 JSON 数组，仅使用以下有效题型: [%s]", typeList)
	}

	userPrompt := "请分析以下HTML中的题目类型，返回JSON数组（例如 [\"single\", \"matrix_radio\"]）：\n\n" + cleanHTML

	aiResponse, err := CallAI(surveyID, systemPrompt, userPrompt, basePath)
	if err != nil {
		return nil, fmt.Errorf("detectQuestionTypesWithAI: CallAI: %w", err)
	}

	return parseTypeDetectionResponse(aiResponse, questionTypes)
}

// parseTypeDetectionResponse 从 AI 响应中解析题型 JSON 数组
//
// @param response    AI 响应（期望含 JSON 数组如 ["matrix_radio","single"]）
// @param validTypes  有效题型列表（用于过滤非法值）
// @returns           解析后的有效题型列表
func parseTypeDetectionResponse(response string, validTypes []string) ([]string, error) {
	cleaned := stripMarkdownFence(response)
	cleaned = strings.TrimSpace(cleaned)

	start := strings.Index(cleaned, "[")
	end := strings.LastIndex(cleaned, "]")
	if start == -1 || end == -1 || end <= start {
		previewLen := len(cleaned)
		if previewLen > 200 {
			previewLen = 200
		}
		return nil, fmt.Errorf("parseTypeDetectionResponse: no JSON array in: %s", cleaned[:previewLen])
	}

	var types []string
	if err := json.Unmarshal([]byte(cleaned[start:end+1]), &types); err != nil {
		return nil, fmt.Errorf("parseTypeDetectionResponse: unmarshal: %w", err)
	}

	validSet := make(map[string]bool, len(validTypes))
	for _, t := range validTypes {
		validSet[strings.ToLower(t)] = true
	}

	var result []string
	for _, t := range types {
		t = strings.TrimSpace(strings.ToLower(t))
		if validSet[t] || t == "unknown" || t == "mixed" {
			result = append(result, t)
		} else {
			fmt.Printf("[题型检测] 忽略无效题型: %q\n", t)
		}
	}

	if len(result) == 0 {
		return []string{"mixed"}, nil
	}
	return result, nil
}

// detectByHTMLSignature 根据 HTML 特征预检测题型，无需 AI。
// 目前覆盖 cardsort_drag / cardsort_select，以避免其被误判为 matrix_radio。
// @param html  清洗后的 HTML 字符串
// @returns     命中的题型列表（未命中返回 nil，交由 AI 继续检测）
func detectByHTMLSignature(html string) []string {
	// cardsort 特征：存在 sq-cardsort 容器元素
	if strings.Contains(html, "sq-cardsort") || strings.Contains(html, "Survey.question.cardsort") {
		// dragAndDrop:1 或 "dragAndDrop":true → cardsort_drag
		if strings.Contains(html, "dragAndDrop: 1") || strings.Contains(html, `"dragAndDrop": true`) {
			return []string{"cardsort_drag"}
		}
		return []string{"cardsort_select"}
	}
	return nil
}

// questionEntry 单个题型的模板条目
type questionEntry struct {
	questionType string
	yamlBlock    string
}

// buildMergedYamlTemplate 根据检测到的题型加载并合并 yaml_template
//
// @param detectedTypes  题型列表
// @returns              合并后的 YAML 占位符模板，error
func buildMergedYamlTemplate(detectedTypes []string) (string, error) {
	if len(detectedTypes) == 0 {
		return GetFixedTemplate("mixed"), nil
	}

	if len(detectedTypes) == 1 {
		return GetFixedTemplate(detectedTypes[0]), nil
	}

	// 多题型：收集各题型的 question 块，合并到一个模板
	var entries []questionEntry
	var headerTemplate string

	for i, qt := range detectedTypes {
		_, yamlTmpl, err := database.LoadPromptByQuestionType(qt)
		if err != nil || strings.TrimSpace(yamlTmpl) == "" {
			yamlTmpl = GetFixedTemplate(qt)
		}

		if i == 0 {
			headerTemplate = yamlTmpl
		}

		block := extractFirstQuestionBlock(yamlTmpl)
		if block != "" {
			entries = append(entries, questionEntry{questionType: qt, yamlBlock: block})
		}
	}

	if len(entries) == 0 || headerTemplate == "" {
		return GetFixedTemplate("mixed"), nil
	}

	return mergeQuestionEntries(headerTemplate, entries), nil
}

// extractFirstQuestionBlock 从 YAML 模板中提取 questions: 下的第一个 question 条目块
//
// @param yamlTemplate  YAML 模板字符串
// @returns             question 条目块（含缩进），未找到返回空字符串
func extractFirstQuestionBlock(yamlTemplate string) string {
	lines := strings.Split(yamlTemplate, "\n")
	inQuestions := false
	inItem := false
	itemIndent := ""
	var result []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inQuestions {
			if strings.HasPrefix(trimmed, "questions:") {
				inQuestions = true
			}
			continue
		}

		if !inItem {
			if strings.HasPrefix(trimmed, "- ") || trimmed == "-" {
				inItem = true
				itemIndent = line[:len(line)-len(strings.TrimLeft(line, " \t"))]
				result = append(result, line)
			}
			continue
		}

		// 同级别新条目 → 停止
		if strings.HasPrefix(trimmed, "- ") {
			currentIndent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			if currentIndent == itemIndent {
				break
			}
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// extractTemplateHeader 提取 YAML 模板头部（survey_page 到 questions: 之前）
//
// @param yamlTemplate  YAML 模板字符串
// @returns             头部字符串（不含 questions: 行）
func extractTemplateHeader(yamlTemplate string) string {
	lines := strings.Split(yamlTemplate, "\n")
	var result []string
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "questions:") {
			break
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

// mergeQuestionEntries 将多个 question 块合并到单个 YAML 模板中
//
//	每个 question 条目前添加 q_index 字段
//
// @param headerTemplate  第一个模板（提供 page_metadata 等头部）
// @param entries         question 块列表
// @returns               合并后的 YAML 模板字符串
func mergeQuestionEntries(headerTemplate string, entries []questionEntry) string {
	header := extractTemplateHeader(headerTemplate)

	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\n  questions:")

	for i, entry := range entries {
		lines := strings.Split(entry.yamlBlock, "\n")
		if len(lines) == 0 {
			continue
		}
		firstLine := lines[0]
		indent := firstLine[:len(firstLine)-len(strings.TrimLeft(firstLine, " \t"))]

		// 写 q_index
		sb.WriteString(fmt.Sprintf("\n%s- q_index: %d", indent, i+1))

		// 写第一行属性（去掉 "- " 前缀）
		trimFirst := strings.TrimSpace(firstLine)
		if strings.HasPrefix(trimFirst, "- ") {
			rest := trimFirst[2:]
			sb.WriteString(fmt.Sprintf("\n%s  %s", indent, rest))
		}

		// 写后续属性行
		for _, line := range lines[1:] {
			if strings.TrimSpace(line) != "" {
				sb.WriteString("\n" + line)
			}
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

// printYamlResultSummary 控制台输出 YAML 结果摘要
func printYamlResultSummary(meta PageMetadata, yamlContent, source string) {
	fmt.Println()
	fmt.Println("=== YAML 提取完成 ===")
	fmt.Printf("SurveyID: %s\n", meta.SurveyID)
	fmt.Printf("PageID: %s\n", meta.PageID)
	fmt.Printf("YAML长度: %d字符\n", len(yamlContent))
	fmt.Printf("来源: %s\n", source)
	fmt.Println()
}

// sanitizeFileName 清理文件名中的非法字符，并截断过长名称
//
// @param name  原始文件名
// @returns     清理后的安全文件名
func sanitizeFileName(name string) string {
	result := name
	for _, char := range illegalFileNameChars {
		result = strings.ReplaceAll(result, char, "")
	}
	result = strings.TrimSpace(result)
	for strings.Contains(result, "__") {
		result = strings.ReplaceAll(result, "__", "_")
	}
	result = strings.Trim(result, "_")
	const maxLen = 200
	if len(result) > maxLen {
		result = result[:maxLen]
	}
	return result
}

// illegalFileNameChars Windows 文件系统非法字符列表
var illegalFileNameChars = []string{"*", "\"", "<", ">", "|", "?", ":", "\\", "/"}
