package extractor

// yaml_templates.go — 固定 YAML 模板加载与题型提取
//   从数据库 prompts 表的 yaml_template 字段获取各题型的标准占位符模板
//   提供 GetFixedTemplate / ExtractQuestionsTypeFromYaml 供 yaml_extractor.go 调用

import (
	"fmt"
	"regexp"
	"strings"

	"survey.cli/core/database"
)

// GetFixedTemplate 根据题型从数据库查询对应的固定占位符模板
//
//	未找到目标题型时降级到 mixed 模板，再失败则返回内置兜底模板
//
// @param questionType  题型字符串（如 matrix_radio / single / multi 等）
// @returns             YAML 模板字符串（已 TrimSpace）
func GetFixedTemplate(questionType string) string {
	tmpl, err := database.GetFixedTemplateByType(questionType)
	if err != nil {
		fmt.Printf("[GetFixedTemplate] 数据库查询失败: %v，使用内置兜底模板\n", err)
		return builtinFallbackTemplate()
	}

	if tmpl != "" {
		fmt.Printf("[GetFixedTemplate] 命中固定模板: %s\n", questionType)
		return strings.TrimSpace(tmpl)
	}

	// 未找到目标题型，降级到 mixed
	mixedTmpl, err := database.GetFixedTemplateByType("mixed")
	if err == nil && mixedTmpl != "" {
		fmt.Printf("[GetFixedTemplate] 未找到 %q 模板，降级使用 mixed\n", questionType)
		return strings.TrimSpace(mixedTmpl)
	}

	fmt.Printf("[GetFixedTemplate] mixed 模板也不存在，使用内置兜底模板\n")
	return builtinFallbackTemplate()
}

// ExtractQuestionsTypeFromYaml 从 AI 生成的 YAML 字符串中提取第一个 questions_type 值
//
//	排除占位符格式（<<<FROM_HTML:xxx>>>），未找到或为占位符时返回 "mixed"
//
// @param yamlStr  AI 输出的 YAML 字符串
// @returns        题型字符串（如 matrix_radio），未识别则 "mixed"
func ExtractQuestionsTypeFromYaml(yamlStr string) string {
	re := regexp.MustCompile(`(?m)^\s*questions_type:\s*["']?([^"'\s\n<]+)["']?`)
	match := re.FindStringSubmatch(yamlStr)
	if len(match) >= 2 {
		qt := strings.Trim(match[1], `"'`)
		if !strings.HasPrefix(qt, "<<<") && qt != "" {
			return qt
		}
	}
	return "mixed"
}

// builtinFallbackTemplate 内置兜底模板（数据库不可用时使用）
func builtinFallbackTemplate() string {
	return `survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: "<<<FROM_HTML:questions_type>>>"
      required: "<<<FROM_HTML:required>>>"
      response_format:
        type: "<<<FROM_HTML:type>>>"
        max_selections: null`
}
