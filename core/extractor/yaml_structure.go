package extractor

// yaml_structure.go — YAML 解析 + 最终输出构建
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                        职责说明                                   │
//	├─────────────────────────────────────────────────────────────────┤
//	│  1. 类型定义     → FinalAnswerJson, InteractionItem, PageSettings │
//	│  2. YAML 解析   → parseYamlPageSettings, parseQuestionsTypeFromYaml
//	│  3. JSON 构建   → buildFinalAnswerJson, buildInteractions       │
//	│  4. 导航按钮    → buildNavButtons, loadNextButtonConfig         │
//	│  5. YAML 清洗   → StripYamlHardcodedValues, StripRecommendedAnswers
//	└─────────────────────────────────────────────────────────────────┘
//
//	依赖关系：
//	  ws_server/main.go
//	    └─ GenerateRealtimeAnswerOnlyWithMeta (yaml_answer.go)
//	          ├─ parseYamlPageSettings       ← 本文件
//	     	  └─ buildFinalAnswerJson        ← 本文件
//
//	  yaml_extractor.go
//	      └─ parseQuestionsTypeFromYaml      ← 本文件
//
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"survey.cli/core/cleaner"
	"survey.cli/core/database"
)

// =============================================================================
// 1. 类型定义
// =============================================================================

// PageSettings 页面按钮/类型设置
type PageSettings struct {
	HasNextButton     bool `json:"has_next_button"`
	HasSubmitButton   bool `json:"has_submit_button"`
	IsThankYouPage    bool `json:"is_thank_you_page"`
	IsDescriptionPage bool `json:"is_description_page"`
}

// NavButtonItem 导航按钮项
type NavButtonItem struct {
	Role           string            `json:"role"`
	XPath          string            `json:"xpath"`
	Text           string            `json:"text"`
	Attributes     map[string]string `json:"attributes"`
	WaitAfterClick int               `json:"wait_after_click,omitempty"`
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
	QuestionType         string      `json:"question_type"`
	Subject              string      `json:"subject"`
	RecommendedAnswers   interface{} `json:"recommended_answers"`           // string or []string
	HumansPass           *bool       `json:"humans_pass,omitempty"`         // nil=未验证，true/false=验证结果
	PromptSuggestionNote string      `json:"prompt_suggestion_note,omitempty"` // ai_score>40 时的改进建议
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
// 2. YAML 解析工具
// =============================================================================

// parseYamlPageSettings 从 yamlData 中解析 page_metadata 下的布尔字段
//
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

// parseYamlBool 解析 YAML 内联布尔值（"key: true/false/yes/1"）
func parseYamlBool(line string) bool {
	parts := strings.Split(line, ":")
	if len(parts) < 2 {
		return false
	}
	val := strings.TrimSpace(parts[1])
	return val == "true" || val == "yes" || val == "1"
}

// =============================================================================
// 3. FinalAnswerJson 构建
// =============================================================================

// parseQuestionsTypeFromYaml 按题目顺序提取 YAML 中每题的 questions_type
//
// @param yamlData  YAML 字符串（含具体值，如 number/single）
// @returns         按 q_index 顺序排列的题型列表（0-based）
func parseQuestionsTypeFromYaml(yamlData string) []string {
	var types []string
	inQuestions := false

	for _, line := range strings.Split(yamlData, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "questions:") {
			inQuestions = true
			continue
		}
		if !inQuestions {
			continue
		}
		if strings.HasPrefix(trimmed, "- q_index:") || strings.HasPrefix(trimmed, "- title:") || strings.HasPrefix(trimmed, "- subject:") {
			types = append(types, "")
		} else if strings.HasPrefix(trimmed, "questions_type:") && len(types) > 0 {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				qt := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				if qt != "" && !strings.HasPrefix(qt, "<<<") {
					types[len(types)-1] = qt
				}
			}
		}
	}
	return types
}

// buildFinalAnswerJson 构建最终 JSON 输出
//
// @param answerResult  AI 生成的结果
// @param yamlData      YAML 数据
// @param pageSettings  页面设置
// @param surveyID      问卷 ID
// @param pageID        页面 ID（由调用方直接传入）
// @param cleanHTML     清洗后的 HTML
// @returns             FinalAnswerJson
func buildFinalAnswerJson(answerResult *YamlAnswerResult, yamlData string, pageSettings PageSettings, surveyID string, pageID string, cleanHTML string) FinalAnswerJson {
	isMatrixPage := detectIsMatrixPage(yamlData)
	yamlQuestionTypes := parseQuestionsTypeFromYaml(yamlData)
	interactions := buildInteractions(answerResult.Interactions, yamlQuestionTypes)
	navButtons := buildNavButtons(pageSettings, surveyID, cleanHTML)

	return FinalAnswerJson{
		Data: DataOutput{
			PageMetadata: PageMetadataOutput{
				PageID:          cleaner.CleanPageID(pageID),
				IsMatrixPage:    isMatrixPage,
				HasNextButton:   pageSettings.HasNextButton,
				HasSubmitButton: pageSettings.HasSubmitButton,
				IsThankYouPage:  pageSettings.IsThankYouPage,
			},
			Interactions: interactions,
			NavButtons:   navButtons,
			PageSummary:  answerResult.PageSummary,
		},
	}
}

// detectIsMatrixPage 判断 YAML 中是否含矩阵题型
func detectIsMatrixPage(yamlData string) bool {
	for _, line := range strings.Split(yamlData, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "matrix_radio") || strings.Contains(trimmed, "matrix_checkbox") {
			return true
		}
	}
	return false
}

// buildInteractions 将 []Interaction 转换为 []InteractionItem
// yamlTypes 为从 YAML 模板解析出的题型列表，当 AI 未返回 question_type 时作为回退值
func buildInteractions(answers []Interaction, yamlTypes []string) []InteractionItem {
	result := make([]InteractionItem, 0, len(answers))

	for i, ans := range answers {
		qt := ans.QuestionType
		if qt == "" && i < len(yamlTypes) && yamlTypes[i] != "" {
			qt = yamlTypes[i]
		}
		item := InteractionItem{
			QuestionType:         qt,
			Subject:              cleaner.CleanPageID(ans.Subject),
			HumansPass:           ans.HumansPass,
			PromptSuggestionNote: ans.PromptSuggestionNote,
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

// =============================================================================
// 4. 导航按钮
// =============================================================================

// NextButtonConfig Next/Submit 按钮配置
type NextButtonConfig struct {
	NextPattern    string
	WaitAfterClick int
}

// loadNextButtonConfig 根据 surveyID 从内存缓存加载 next_button 配置
// @param surveyID 问卷 ID
// @returns NextButtonConfig，未找到返回空结构（不报错，由调用方兜底）
func loadNextButtonConfig(surveyID string) (NextButtonConfig, error) {
	sw := database.FindSiteBySurveyName(surveyID)
	if sw == nil || sw.Fingerprint == nil {
		return NextButtonConfig{}, nil
	}
	return NextButtonConfig{
		NextPattern:    sw.Fingerprint.NextPattern,
		WaitAfterClick: sw.Fingerprint.WaitAfterClick,
	}, nil
}

// buildNavButtons 构建 nav_buttons 输出
//
// @param pageSettings 页面设置
// @param surveyID     问卷 ID
// @param cleanHTML    清洗后的 HTML 内容
// @returns            NavButtonsOutput
func buildNavButtons(pageSettings PageSettings, surveyID string, cleanHTML string) NavButtonsOutput {
	var buttons []NavButtonItem
	primaryButton := ""

	nextButtonConfig, err := loadNextButtonConfig(surveyID)
	if err == nil && nextButtonConfig.NextPattern != "" {
		xpath := regexToXPath(nextButtonConfig.NextPattern)
		attrs := extractButtonAttrs(cleanHTML, xpath)
		buttons = append(buttons, NavButtonItem{
			Role:           "next",
			XPath:          xpath,
			Text:           "Next",
			Attributes:     attrs,
			WaitAfterClick: nextButtonConfig.WaitAfterClick,
		})
		primaryButton = "next"
	} else if pageSettings.HasNextButton {
		xpath, text, attrs := extractButtonFromHTML(cleanHTML, "next")
		buttons = append(buttons, NavButtonItem{
			Role:       "next",
			XPath:      xpath,
			Text:       text,
			Attributes: attrs,
		})
		primaryButton = "next"
	}

	if pageSettings.HasSubmitButton {
		xpath, text, attrs := extractButtonFromHTML(cleanHTML, "submit")
		buttons = append(buttons, NavButtonItem{
			Role:       "submit",
			XPath:      xpath,
			Text:       text,
			Attributes: attrs,
		})
		primaryButton = "submit"
	}

	return NavButtonsOutput{
		Buttons:       buttons,
		PrimaryButton: primaryButton,
	}
}

// regexToXPath 将 JavaScript 风格的正则转换为 XPath
func regexToXPath(pattern string) string {
	cleanPattern := pattern
	for _, suffix := range []string{"/gi", "/g", "/"} {
		if strings.HasSuffix(cleanPattern, suffix) {
			cleanPattern = cleanPattern[:len(cleanPattern)-len(suffix)]
			break
		}
	}
	if strings.HasPrefix(cleanPattern, "/") {
		cleanPattern = cleanPattern[1:]
	}

	xpath := "//"
	tagMatch := regexp.MustCompile(`<([a-zA-Z]+)`).FindStringSubmatch(cleanPattern)
	if len(tagMatch) >= 2 {
		xpath += tagMatch[1]
	} else {
		return "//input[@id='NextButton']"
	}

	idMatch := regexp.MustCompile(`id=["']([^"']+)["']`).FindStringSubmatch(cleanPattern)
	if len(idMatch) >= 2 {
		xpath += "[@id='" + idMatch[1] + "']"
	} else {
		valueMatch := regexp.MustCompile(`value=["']([^"']+)["']`).FindStringSubmatch(cleanPattern)
		if len(valueMatch) >= 2 {
			xpath += "[@value='" + valueMatch[1] + "']"
		}
	}

	return xpath
}

// extractButtonAttrs 从 HTML 中根据 XPath 提取按钮属性
func extractButtonAttrs(html string, xpath string) map[string]string {
	attrs := make(map[string]string)

	var rePattern string
	if strings.Contains(xpath, "@id=") {
		idMatch := regexp.MustCompile(`@id='([^']+)'`).FindStringSubmatch(xpath)
		if len(idMatch) >= 2 {
			rePattern = `<[^>]*id=["']` + idMatch[1] + `["'][^>]*>`
		}
	} else if strings.Contains(xpath, "@value=") {
		valueMatch := regexp.MustCompile(`@value='([^']+)'`).FindStringSubmatch(xpath)
		if len(valueMatch) >= 2 {
			rePattern = `<[^>]*value=["']` + valueMatch[1] + `["'][^>]*>`
		}
	}

	if rePattern == "" {
		return attrs
	}

	re := regexp.MustCompile(`(?i)` + rePattern)
	match := re.FindString(html)
	if match != "" {
		if m := regexp.MustCompile(`type=["']([^"']+)["']`).FindStringSubmatch(match); len(m) >= 2 {
			attrs["type"] = m[1]
		}
		if m := regexp.MustCompile(`id=["']([^"']+)["']`).FindStringSubmatch(match); len(m) >= 2 {
			attrs["id"] = m[1]
		}
		if m := regexp.MustCompile(`value=["']([^"']+)["']`).FindStringSubmatch(match); len(m) >= 2 {
			attrs["value"] = m[1]
		}
	}

	return attrs
}

// extractButtonFromHTML 从 HTML 中提取按钮的 XPath、文本和属性
func extractButtonFromHTML(html string, role string) (string, string, map[string]string) {
	attrs := make(map[string]string)
	xpath := ""
	text := "Next"

	if role == "next" {
		for _, val := range []string{"Next", "Continue", "Proceed"} {
			if match, x, a, t := extractInputButton(html, val); match {
				return x, t, a
			}
		}
		xpath = "//input[@type='submit' and @value='Next']"
		attrs = map[string]string{"type": "submit", "value": "Next"}
	} else if role == "submit" {
		if match, x, a, t := extractInputButton(html, "Submit"); match {
			return x, t, a
		}
		xpath = "//input[@type='submit' and @value='Submit']"
		attrs = map[string]string{"type": "submit", "value": "Submit"}
	}

	return xpath, text, attrs
}

// extractInputButton 提取 input 按钮（匹配 value="xxx"）
func extractInputButton(html string, value string) (bool, string, map[string]string, string) {
	pattern := fmt.Sprintf(`<input[^>]*value=["']%s["'][^>]*>`, value)
	re := regexp.MustCompile(`(?i)` + pattern)
	idxs := re.FindStringIndex(html)
	if idxs == nil {
		return false, "", nil, ""
	}

	match := html[idxs[0]:idxs[1]]
	attrs := extractAttributes(match)
	xpath := fmt.Sprintf("//input[@type='submit' and @value='%s']", value)

	return true, xpath, attrs, value
}

// extractAttributes 从 HTML 标签中提取属性
func extractAttributes(tag string) map[string]string {
	attrs := make(map[string]string)
	re := regexp.MustCompile(`(\w+)=["']?([^"'>]+)["']?`)
	for _, m := range re.FindAllStringSubmatch(tag, -1) {
		if len(m) == 3 {
			attrs[m[1]] = m[2]
		}
	}
	return attrs
}

// =============================================================================
// 5. 文件存储
// =============================================================================

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

// =============================================================================
// 6. YAML 清洗工具
// =============================================================================

// StripRecommendedAnswers 移除 interactions JSON 中的 recommended_answers 字段
// 用于计算 interactions_md5
func StripRecommendedAnswers(jsonStr string) string {
	re1 := regexp.MustCompile(`,"recommended_answers":"[^"]*"|"recommended_answers":"[^"]*",`)
	result := re1.ReplaceAllString(jsonStr, "")

	re2 := regexp.MustCompile(`,"recommended_answers":\[[^\]]*\]|"recommended_answers":\[[^\]]*\],`)
	result = re2.ReplaceAllString(result, "")

	return result
}

// StripYamlHardcodedValues 将 YAML 中的硬编码值替换为 <<<FROM_HTML:xxx>>> 占位符
func StripYamlHardcodedValues(yamlStr string) string {
	result := yamlStr

	// 1. options 数组
	result = regexp.MustCompile(`(?m)^\s*options:\s*$\n(?:^\s*-\s*"[^"]*"\s*$\n?)+`).
		ReplaceAllStringFunc(result, func(match string) string {
			return extractIndentAndReplace(match, "options:", `options: "<<<FROM_HTML:options[]>>>"`)
		})

	// 2. columns 数组
	result = regexp.MustCompile(`(?m)^\s*columns:\s*$\n(?:^\s*-\s*"[^"]*"\s*$\n?)+`).
		ReplaceAllStringFunc(result, func(match string) string {
			return extractIndentAndReplace(match, "columns:", `columns: "<<<FROM_HTML:columns[]>>>"`)
		})

	// 3. rows 数组
	result = regexp.MustCompile(`(?m)^\s*rows:\s*$\n(?:^\s*-\s*"[^"]*"\s*$\n?)+`).
		ReplaceAllStringFunc(result, func(match string) string {
			return extractIndentAndReplace(match, "rows:", `rows: "<<<FROM_HTML:rows[]>>>"`)
		})
	result = regexp.MustCompile(`(?m)^\s*rows:\s*"<<<FROM_HTML:[^"]+>>>`).
		ReplaceAllStringFunc(result, func(match string) string {
			indent := regexp.MustCompile(`(?m)^(\s*)`).FindString(match)
			return indent + `rows: "<<<FROM_HTML:rows[]>>>"`
		})

	// 4. title 字段
	result = regexp.MustCompile(`(?m)^\s*(title):\s*(?:\"([^\"<]*)\"|([^<\s][^\s]*))`).
		ReplaceAllStringFunc(result, func(match string) string {
			indent := regexp.MustCompile(`(?m)^(\s*)`).FindString(match)
			return indent + `title: "<<<FROM_HTML:title>>>"`
		})

	// 5. placeholder 字段
	result = regexp.MustCompile(`(?m)^\s*(placeholder):\s*(?:\"([^\"<]*)\"|([^<\s][^\s]*))`).
		ReplaceAllStringFunc(result, func(match string) string {
			indent := regexp.MustCompile(`(?m)^(\s*)`).FindString(match)
			return indent + `placeholder: "<<<FROM_HTML:placeholder>>>"`
		})

	// 6. page_metadata 布尔字段
	result = regexp.MustCompile(`(?m)^\s*(has_next_button|has_submit_button|is_thank_you_page|is_description_page):\s*(?:\"(true|false)\"|(true|false))`).
		ReplaceAllStringFunc(result, func(match string) string {
			indent := regexp.MustCompile(`(?m)^(\s*)`).FindString(match)
			fieldName := ""
			switch {
			case strings.Contains(match, "has_next_button"):
				fieldName = "has_next_button"
			case strings.Contains(match, "has_submit_button"):
				fieldName = "has_submit_button"
			case strings.Contains(match, "is_thank_you_page"):
				fieldName = "is_thank_you_page"
			case strings.Contains(match, "is_description_page"):
				fieldName = "is_description_page"
			}
			return indent + fieldName + `: "<<<FROM_HTML:` + fieldName + `>>>"`
		})

	// 7. questions_type 字段
	result = regexp.MustCompile(`(?m)^\s*(questions_type):\s*(?:\"([^\"]+)\"|(\S+))`).
		ReplaceAllStringFunc(result, func(match string) string {
			indent := regexp.MustCompile(`(?m)^(\s*)`).FindString(match)
			return indent + `questions_type: "<<<FROM_HTML:questions_type>>>"`
		})

	// 8. required 字段
	result = regexp.MustCompile(`(?m)^\s*(required):\s*(?:\"(true|false)\"|(true|false))`).
		ReplaceAllStringFunc(result, func(match string) string {
			indent := regexp.MustCompile(`(?m)^(\s*)`).FindString(match)
			return indent + `required: "<<<FROM_HTML:required>>>"`
		})

	// 9. response_format 下的字段
	result = regexp.MustCompile(`(?m)^\s*(type|orientation|option_sort):\s*(?:\"([^\"]+)\"|(\S+))`).
		ReplaceAllStringFunc(result, func(match string) string {
			indent := regexp.MustCompile(`(?m)^(\s*)`).FindString(match)
			fieldName := ""
			if strings.Contains(match, "type:") && !strings.Contains(match, "questions_type") {
				fieldName = "type"
			} else if strings.Contains(match, "orientation:") {
				fieldName = "orientation"
			} else if strings.Contains(match, "option_sort:") {
				fieldName = "option_sort"
			}
			if fieldName != "" {
				return indent + fieldName + `: "<<<FROM_HTML:` + fieldName + `>>>"`
			}
			return match
		})

	// 10. max_selections 字段
	result = regexp.MustCompile(`(?m)^\s*(max_selections):\s*(?:\"?(\d+|null)\"?|(\d+|null))`).
		ReplaceAllStringFunc(result, func(match string) string {
			indent := regexp.MustCompile(`(?m)^(\s*)`).FindString(match)
			return indent + `max_selections: "<<<FROM_HTML:max_selections>>>"`
		})

	return result
}

// extractIndentAndReplace 提取缩进并替换数组为占位符
func extractIndentAndReplace(match, fieldName, replacement string) string {
	for _, line := range strings.Split(match, "\n") {
		if strings.Contains(line, fieldName) {
			trimmed := strings.TrimLeft(line, " \t")
			indent := line[:len(line)-len(trimmed)]
			return indent + replacement
		}
	}
	return replacement
}
