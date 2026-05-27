package cleaner

// Page ID 提取与清洗模块
//   ExtractPageID - 从 HTML 中按优先级提取并清洗 page_id
//   CleanPageID   - 将原始页面文本清洗为 page_id（只保留字母+数字，空格合并为单个）

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"survey.cli/core/database"
)

// 常量
const (
	MaxPageIDLength = 100
)

// 预编译正则（包级别，只编译一次）
var (
	// 只保留字母、数字、空格、连字符、下划线、撇号；其他字符全部替换为空格（包括点号）
	reKeepAlphanumeric = regexp.MustCompile(`[^a-zA-Z0-9 _\-'` + "`" + `]+`)
)

// LoadPatternsForSurvey 根据 surveyID 从内存缓存加载 page_id 正则配置
// @param surveyID  问卷 ID
// @returns         正则模式列表，或错误
func LoadPatternsForSurvey(surveyID string) ([]string, error) {
	sw := database.FindSiteBySurveyName(surveyID)
	if sw == nil {
		return nil, fmt.Errorf("LoadPatternsForSurvey: survey_name=%s not found in site cache", surveyID)
	}
	if sw.Fingerprint == nil {
		return nil, fmt.Errorf("LoadPatternsForSurvey: survey_name=%s has no fingerprint config", surveyID)
	}
	if len(sw.Fingerprint.PageIDPatternRegexes) == 0 {
		return nil, fmt.Errorf("LoadPatternsForSurvey: survey_name=%s has empty regex patterns", surveyID)
	}
	return sw.Fingerprint.PageIDPatternRegexes, nil
}

// ExtractPageIDWithPatterns 从 HTML 中按指定模式提取并清洗 page_id
// @param html     原始 HTML 字符串
// @param patterns 正则表达式列表（按优先级排序，提取第1个捕获组）
//                   格式支持 JavaScript 风格：/pattern/flags，如 /<legend>.../gi
// @returns        清洗后的 page_id，或错误
func ExtractPageIDWithPatterns(html string, patterns []string) (string, error) {
	for _, pattern := range patterns {
		// 去掉 JavaScript 风格的 /gi 或 /g 或 /i 修饰符
		// 输入: /<legend>.../gi -> <legend>...
		cleanPattern := strings.TrimPrefix(pattern, "/")
		if strings.HasSuffix(cleanPattern, "/gi") {
			cleanPattern = strings.TrimSuffix(cleanPattern, "/gi")
		} else if strings.HasSuffix(cleanPattern, "/g") {
			cleanPattern = strings.TrimSuffix(cleanPattern, "/g")
		} else if strings.HasSuffix(cleanPattern, "/i") {
			cleanPattern = strings.TrimSuffix(cleanPattern, "/i")
		}

		re, err := regexp.Compile(`(?i)` + cleanPattern)
		if err != nil {
			return "", fmt.Errorf("ExtractPageIDWithPatterns: invalid regex '%s': %w", pattern, err)
		}
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
//   3. 处理 JSON/JS 字符串转义（如 \" → 空, \\' → ', \\n → 空格）
//   4. 转小写
//   5. 只保留字母、数字、空格、连字符(-)、下划线(_)、撇号(')，其他字符替换为空格（包括点号）
//   6. 合并多余空格为单个空格
//   7. 去除首尾空格
//   8. 截断至 100 字符
// @param raw  原始页面文本
// @returns    清洗后的 page_id
func CleanPageID(raw string) string {
	// 1. 移除 HTML 标签
	s := StripTags(raw)

	// 2. 解码 HTML 实体
	s = decodeHTMLEntities(s)

	// 3. 处理 JSON/JS 字符串转义
	s = cleanJsonEscapes(s)

	// 4. 转小写
	s = strings.ToLower(s)

	// 5. 只保留字母、数字、空格、连字符、下划线、撇号，其他替换为空格（包括点号）
	s = reKeepAlphanumeric.ReplaceAllString(s, " ")

	// 6. 合并多余空格
	s = reSpaces.ReplaceAllString(s, " ")

	// 7. 去除首尾空格
	s = strings.TrimSpace(s)

	// 8. 截断至 100 字符
	if len(s) > MaxPageIDLength {
		s = s[:MaxPageIDLength]
		s = strings.TrimRight(s, " ")
	}

	return s
}

// cleanJsonEscapes 处理 JSON/JS 字符串转义序列
//   \" → 空 (移除 JSON 引号)
//   \\' → ' (保留撇号)
//   \\n, \\t, \\r → 空格
//   \\ → 空 (移除剩余反斜杠)
func cleanJsonEscapes(s string) string {
	// 移除 JSON 字符串包裹的引号 \" 或 \" 或 "
	s = reJsonQuote.ReplaceAllString(s, "")

	// 处理常见的 JSON/JS 转义
	s = strings.ReplaceAll(s, `\"`, "") // JSON 双引号转义
	s = strings.ReplaceAll(s, `\'`, "'") // JS 单引号转义
	s = strings.ReplaceAll(s, `\\`, "") // 剩余反斜杠
	s = strings.ReplaceAll(s, `\n`, " ")
	s = strings.ReplaceAll(s, `\t`, " ")
	s = strings.ReplaceAll(s, `\r`, " ")

	// 处理 Unicode 转义 \uXXXX
	s = reUnicodeEscape.ReplaceAllString(s, "")

	return s
}

// reJsonQuote 匹配 JSON 字符串引号包裹
var reJsonQuote = regexp.MustCompile(`^[\"']|[\"']$`)

// reUnicodeEscape 匹配 Unicode 转义序列
var reUnicodeEscape = regexp.MustCompile(`\\u[0-9a-fA-F]{4}`)

// decodeHTMLEntities 解码常见 HTML 实体为对应字符
// &nbsp; → 空格, &amp; → &, &lt; → <, &gt; → >, &quot; → ", &apos; → '
func decodeHTMLEntities(s string) string {
	s = html.UnescapeString(s)
	// 额外处理非断行空格 (U+00A0) → 普通空格
	s = strings.ReplaceAll(s, "\u00A0", " ")
	return s
}
