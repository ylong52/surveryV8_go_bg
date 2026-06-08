package cleaner

// HTML 清洗模块：提供两种清洗策略
//   StripTags            - 移除所有 HTML 标签，返回纯文本（通用）
//   StripForDirectSurvey - 移除无效标签+属性，保留结构，用于直接问卷解析

import (
	"regexp"
	"strings"
)

// ── 预编译正则（包级别，只编译一次）──────────────────────────────────────────

var (
	reStyle       = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reScript      = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reNoscript    = regexp.MustCompile(`(?is)<noscript[^>]*>.*?</noscript>`)
	reSvg         = regexp.MustCompile(`(?is)<svg[^>]*>.*?</svg>`)
	reComment     = regexp.MustCompile(`(?s)<!--.*?-->`)
	reOpenTag     = regexp.MustCompile(`<[a-zA-Z][^>]*>`) // 只匹配开标签（非 </ <!  <?）
	reBetweenTags = regexp.MustCompile(`>\s+<`)           // 标签间空白（含换行）
	reMultiSpace  = regexp.MustCompile(` {2,}`)           // 连续空格
	reAttrToken   = regexp.MustCompile(`(?i)([\w-]+)(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s/>]*))?`)

	// StripTags 使用
	reAllTag = regexp.MustCompile(`<[^>]+>`)
	reSpaces = regexp.MustCompile(`\s{2,}`)
)

// attrWhitelist 属性保留白名单（其余全部剥除，含 style / class / aria-* / data-* 未列出的）
var attrWhitelist = map[string]bool{
	// 表单语义
	"name":     true,
	"value":    true,
	"id":       true,
	"type":     true,
	"checked":  true,
	"selected": true,
	"for":      true,
	"headers":  true,
	"role":     true,
	// data-* 精确白名单
	"data-name":  true,
	"data-value": true,
	"data-id":    true,
	"data-index": true,
	"data-qid":   true,
}

// ── 公开函数 ──────────────────────────────────────────────────────────────────

// StripForDirectSurvey 清洗 HTML，用于直接问卷（Direct Survey）解析
// 移除：<style> <svg> HTML注释；style/class/aria-*/未列出的 data-* 属性
// 保留：name value id type checked selected for headers role + data-name/value/id/index/qid
// 压缩：标签间空白（含换行）→ 无空隙；多个空格 → 单空格
// @param raw  原始 HTML 字符串
// @returns    清洗后的紧凑 HTML
func StripForDirectSurvey(raw string) string {
	s := reScript.ReplaceAllString(raw, "")   // 先删 script（含大型 JSON runtime blob）
	s = reNoscript.ReplaceAllString(s, "")
	s = reStyle.ReplaceAllString(s, "")
	s = reSvg.ReplaceAllString(s, "")
	s = reComment.ReplaceAllString(s, "")
	s = reOpenTag.ReplaceAllStringFunc(s, filterTagAttrs)
	s = reBetweenTags.ReplaceAllString(s, "><")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// StripTags 移除所有 HTML 标签，返回纯文本（通用场景使用）
// @param html  原始 HTML 字符串
// @returns     清洗后的纯文本
func StripTags(html string) string {
	html = reStyle.ReplaceAllString(html, "")
	html = reAllTag.ReplaceAllString(html, " ")
	html = reSpaces.ReplaceAllString(html, " ")
	return strings.TrimSpace(html)
}

// ExtractForm 从完整页面 HTML 中提取第一个 <form>…</form> 片段
// @param html  完整页面 HTML
// @returns     form 标签内容，未找到返回空字符串
func ExtractForm(html string) string {
	re := regexp.MustCompile(`(?is)<form[^>]*>.*?</form>`)
	return re.FindString(html)
}

// ── 内部实现 ──────────────────────────────────────────────────────────────────

// filterTagAttrs 对单个开标签进行属性过滤，只保留 attrWhitelist 中的属性
func filterTagAttrs(tag string) string {
	inner := tag[1 : len(tag)-1] // 去掉首尾 < >

	// 处理自闭合标签 <br/> <input/>
	selfClose := ""
	if strings.HasSuffix(inner, "/") {
		selfClose = "/"
		inner = inner[:len(inner)-1]
	}

	// 找到标签名与属性串的分界
	spaceIdx := strings.IndexAny(inner, " \t")
	if spaceIdx == -1 {
		// 无属性标签：<div> <br/>
		return "<" + strings.TrimSpace(inner) + selfClose + ">"
	}

	tagName := inner[:spaceIdx]
	attrStr := inner[spaceIdx:]

	// 逐个提取属性 token，只保留白名单
	var kept []string
	for _, m := range reAttrToken.FindAllString(attrStr, -1) {
		name := strings.ToLower(strings.TrimSpace(strings.SplitN(m, "=", 2)[0]))
		if attrWhitelist[name] {
			kept = append(kept, strings.TrimSpace(m))
		}
	}

	result := "<" + tagName
	if len(kept) > 0 {
		result += " " + strings.Join(kept, " ")
	}
	result += selfClose + ">"
	return result
}
