package cleaner

// Page ID 提取与清洗模块
//   ExtractPageID - 从 HTML 中按优先级提取并清洗 page_id
//   CleanPageID   - 将原始页面文本清洗为 page_id（只保留字母+数字，空格合并为单个）

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// 常量
const (
	MaxPageIDLength = 100
)

// 预编译正则（包级别，只编译一次）
var (
	// 只保留字母、数字、空格；其他字符全部替换为空格
	reKeepAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
)

// 默认的 page_id 提取模式（按优先级）
var defaultPatterns = []string{
	`<legend[^>]*>([\s\S]*?)</legend>`,
	`<label[^>]*>([\s\S]*?)</label>`,
	`<div[^>]*>([\s\S]*?)</div>`,
}

// ExtractPageID 从 HTML 中按优先级提取并清洗 page_id
// @param html  原始 HTML 字符串
// @returns     清洗后的 page_id，或错误（空 page_id 时返回错误）
func ExtractPageID(html string) (string, error) {
	return ExtractPageIDWithPatterns(html, defaultPatterns)
}

// ExtractPageIDWithPatterns 从 HTML 中按指定模式提取并清洗 page_id
// @param html     原始 HTML 字符串
// @param patterns 正则表达式列表（按优先级排序，提取第1个捕获组）
// @returns        清洗后的 page_id，或错误
func ExtractPageIDWithPatterns(html string, patterns []string) (string, error) {
	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern)
		matches := re.FindAllStringSubmatch(html, -1)
		for _, match := range matches {
			if len(match) >= 2 {
				cleaned := CleanPageID(match[1])
				if cleaned != "" {
					return cleaned, nil
				}
			}
		}
	}
	return "", fmt.Errorf("ExtractPageID: no valid page_id found (all patterns empty or no match)")
}

// CleanPageID 将 rawpage_id 清洗为 page_id
// 规则：
//   1. 移除所有 HTML 标签
//   2. 解码 HTML 实体（如 &nbsp; → 空格）
//   3. 转小写
//   4. 只保留字母、数字、空格，其他字符替换为空格
//   5. 合并多余空格为单个空格
//   6. 去除首尾空格
//   7. 截断至 100 字符
// @param raw  原始页面文本
// @returns    清洗后的 page_id
func CleanPageID(raw string) string {
	// 1. 移除 HTML 标签
	s := StripTags(raw)

	// 2. 解码 HTML 实体
	s = decodeHTMLEntities(s)

	// 3. 转小写
	s = strings.ToLower(s)

	// 4. 只保留字母、数字、空格
	s = reKeepAlphanumeric.ReplaceAllString(s, " ")

	// 5. 合并多余空格
	s = reSpaces.ReplaceAllString(s, " ")

	// 6. 去除首尾空格
	s = strings.TrimSpace(s)

	// 7. 截断至 100 字符
	if len(s) > MaxPageIDLength {
		s = s[:MaxPageIDLength]
		s = strings.TrimRight(s, " ")
	}

	return s
}

// decodeHTMLEntities 解码常见 HTML 实体为对应字符
// &nbsp; → 空格, &amp; → &, &lt; → <, &gt; → >, &quot; → ", &apos; → '
func decodeHTMLEntities(s string) string {
	s = html.UnescapeString(s)
	// 额外处理非断行空格 (U+00A0) → 普通空格
	s = strings.ReplaceAll(s, "\u00A0", " ")
	return s
}
