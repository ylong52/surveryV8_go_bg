package simulator

import (
	"regexp"
	"strings"
)

// ParseYamlOptions 从 YAML 结构中提取 options[]
// YamlStructure 格式：
//
//	response_format:
//	  type: "single"
//	  options:
//	    - label: "男"
//	      value: "1"
//	    - label: "女"
//	      value: "2"
//
// @param yamlStr YAML 原始字符串
// @returns []SimOption 选项列表（Index 为 1-based）
func ParseYamlOptions(yamlStr string) []SimOption {
	var options []SimOption

	optionsRe := regexp.MustCompile(`options:\s*\n((?:\s{2,}-\s+.*\n?)*)`)
	optionsMatch := optionsRe.FindStringSubmatch(yamlStr)
	if optionsMatch == nil || len(optionsMatch) < 2 {
		return options
	}

	optionsBlock := optionsMatch[1]

	itemRe := regexp.MustCompile(`-\s+(.*?)(?=\n\s*-|\n\s*\S|\z)`)
	itemMatches := itemRe.FindAllStringSubmatch(optionsBlock, -1)

	for idx, itemMatch := range itemMatches {
		if itemMatch == nil || len(itemMatch) < 2 {
			continue
		}

		itemStr := itemMatch[1]
		label := extractYamlField(itemStr, "label")
		value := extractYamlField(itemStr, "value")

		if label != "" && value != "" {
			options = append(options, SimOption{
				Index: idx + 1, // 1-based
				Label: label,
				Value: value,
			})
		}
	}

	return options
}

// extractYamlField 从 YAML item 字符串中提取指定字段的值
// 支持两种格式：
//   - label: "男"
//   - label: 男
func extractYamlField(yamlItem, fieldName string) string {
	// 正则：fieldName: "value" 或 fieldName: value
	pattern := regexp.MustCompile(fieldName + `:\s*"?([^"\n]+)"?(?:\n|$)`)
	match := pattern.FindStringSubmatch(yamlItem)
	if match != nil && len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

// FindOptionByLabel 在 options[] 中按标签查找选项
// 返回选项索引（1-based），或 -1 如果未找到
func FindOptionByLabel(options []SimOption, label string) int {
	for _, opt := range options {
		if opt.Label == label {
			return opt.Index
		}
	}
	return -1
}

// FormatOptionDisplay 格式化选项用于显示
// 例如: 0. 男 | 1. 女 | 2. 不知道
func FormatOptionDisplay(options []SimOption) string {
	parts := make([]string, len(options))
	for i, opt := range options {
		parts[i] = strings.Join([]string{
			strings.TrimSpace(opt.Label),
		}, "")
	}
	lines := make([]string, len(options))
	for i, opt := range options {
		lines[i] = strings.Join([]string{
			strings.TrimPrefix(strings.TrimSpace(opt.Label), "-"),
		}, "")
	}

	var result strings.Builder
	for i, opt := range options {
		if i > 0 {
			result.WriteString(" | ")
		}
		result.WriteString(strings.TrimPrefix(strings.TrimSpace(opt.Label), "-"))
	}
	return result.String()
}
