package simulator

// html_option_parser.go — 从原始 HTML 解析问卷选项（radio/checkbox/input）
//
// 支持题型：
//   single         → <input type="radio"> + <label>
//   multi          → <input type="checkbox"> + <label>
//   text           → <input type="number/text"> 或 <textarea>
//   matrix_radio   → <input type="radio"> 矩阵（按行分组）
//   matrix_checkbox → <input type="checkbox"> 矩阵
//
// 解析逻辑：
//   1. 按 question div 分块隔离每个问题
//   2. 在每个问题块内查找所有 radio/checkbox/input
//   3. 关联 label 提取选项文本
//   4. 按 DOM 顺序排列选项

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"survey.cli/core/models"
)

// HTMLOptionParser HTML 选项解析器
type HTMLOptionParser struct {
	rawHTML string
}

// NewHTMLOptionParser 创建解析器
// @param rawHTML 原始 HTML（不经过清洗，保留完整标签结构）
func NewHTMLOptionParser(rawHTML string) *HTMLOptionParser {
	return &HTMLOptionParser{rawHTML: rawHTML}
}

// ParseQuestions 解析所有问题，返回 []SimQuestion
// @param interactions AI 返回的推荐答案列表（用于确定题型）
// @returns 包含选项信息的题目列表
func (p *HTMLOptionParser) ParseQuestions(interactions []models.Interaction) []SimQuestion {
	questions := make([]SimQuestion, 0, len(interactions))

	fmt.Printf("[DEBUG ParseQuestions] 共 %d 个 interaction\n", len(interactions))

	for i, interaction := range interactions {
		block := p.extractQuestionBlock(i)
		qType := p.inferQuestionTypeFromBlock(block)
		fmt.Printf("[DEBUG ParseQuestions] qIndex=%d, qType=%q, interaction.QuestionType=%q\n",
			i, qType, interaction.QuestionType)
		if qType == "unknown" && interaction.QuestionType != "" {
			qType = p.normalizeQuestionType(interaction.QuestionType)
			fmt.Printf("[DEBUG ParseQuestions]   → 来自 AI normalize 后: %s\n", qType)
		}
		if qType == "unknown" {
			qType = "text"
		}
		options := p.parseOptionsForBlock(block, qType)
		fmt.Printf("[DEBUG ParseQuestions]   → 选项数=%d\n", len(options))

		// 如果有选项，推断为单选题或多选题
		if len(options) > 0 && (qType == "text" || qType == "unknown") {
			qType = "single"
			fmt.Printf("[DEBUG ParseQuestions]   → 有选项，回填 qType=single\n")
		}

		q := SimQuestion{
			Index:        i + 1, // 1-based
			QuestionType: qType,
			Subject:      interaction.Subject,
			Options:      options,
			RawHTML:      block,
		}

		// 映射推荐答案到选项索引
		// 优先使用 RecommendedAnswer（旧版 ShowAnswer 兼容）
		recommendedAnswer := interaction.RecommendedAnswer
		if recommendedAnswer == nil {
			recommendedAnswer = interaction.ShowAnswer
		}
		q.RecommendedIndex = p.mapRecommendedAnswer(&q, recommendedAnswer)
		fmt.Printf("[DEBUG ParseQuestions]   → RecommendedIndex=%v\n", q.RecommendedIndex)

		questions = append(questions, q)
	}

	return questions
}

// ParseQuestionsFromStructured 用 AI 已解析结构构建 SimQuestions（跳过 HTML 重解析）
//
// 适用场景：AI 已返回完整的 questions（matrix_radio / cardsort 等复杂题型），
// HTML 解析无法正确拆分多行时，直接用 AI 结果。
//
// @param interactions  AI 推荐答案列表（subject + recommended answer）
// @param structured    AI 已解析的题目结构（question_type + options with label/xpath）
// @returns             []SimQuestion
func (p *HTMLOptionParser) ParseQuestionsFromStructured(interactions []models.Interaction, structured []StructuredQuestion) []SimQuestion {
	questions := make([]SimQuestion, 0, len(interactions))

	for i, interaction := range interactions {
		var sq StructuredQuestion
		if i < len(structured) {
			sq = structured[i]
		}

		qType := p.normalizeQuestionType(sq.QuestionType)
		if qType == "unknown" && interaction.QuestionType != "" {
			qType = p.normalizeQuestionType(interaction.QuestionType)
		}
		if qType == "unknown" {
			qType = "text"
		}
		// 兜底：有选项但 type 仍是 text/unknown，从 HTML 推断真实类型
		if len(sq.Options) > 0 && (qType == "text" || qType == "unknown") {
			block := p.extractQuestionBlock(i)
			inferred := p.inferQuestionTypeFromBlock(block)
			if inferred != "unknown" {
				qType = inferred
			} else {
				qType = "single" // 有选项默认单选
			}
		}

		q := SimQuestion{
			Index:        i + 1,
			QuestionType: qType,
			Subject:      interaction.Subject,
			Options:      sq.Options,
		}

		recommendedAnswer := interaction.RecommendedAnswer
		if recommendedAnswer == nil {
			recommendedAnswer = interaction.ShowAnswer
		}
		q.RecommendedIndex = p.mapRecommendedAnswer(&q, recommendedAnswer)

		fmt.Printf("[DEBUG ParseQuestionsFromStructured] qIndex=%d, qType=%q, subject=%q, options=%d, rec=%v\n",
			i, qType, interaction.Subject, len(sq.Options), q.RecommendedIndex)

		questions = append(questions, q)
	}
	return questions
}

// parseOptionsForBlock 解析单个问题块的选项
func (p *HTMLOptionParser) parseOptionsForBlock(block string, qType string) []SimOption {
	switch qType {
	case "single", "dropdown":
		return p.parseInputOptions(block, "radio")
	case "multi":
		return p.parseInputOptions(block, "checkbox")
	case "matrix_radio":
		return p.parseMatrixOptions(block, "radio")
	case "matrix_checkbox":
		return p.parseMatrixOptions(block, "checkbox")
	default:
		return nil
	}
}

// parseInputOptions 通用解析 radio/checkbox 选项
func (p *HTMLOptionParser) parseInputOptions(block string, inputType string) []SimOption {
	var options []SimOption

	inputRe := regexp.MustCompile(
		`(?i)<input[^>]*type=["']?` + inputType + `["']?[^>]*>`,
	)
	inputs := inputRe.FindAllStringSubmatchIndex(block, -1)

	for i, loc := range inputs {
		inputTag := block[loc[0]:loc[1]]

		// 提取 value
		value := p.extractAttr(inputTag, "value")
		if value == "" {
			value = p.extractAttr(inputTag, "id")
		}

		// 提取 label
		label := p.extractLabelForInput(block, inputTag, loc[0], loc[1])

		// 生成 XPath（基于 input 元素属性，优先用稳定属性）
		xpath := p.generateInputXPath(inputTag, loc[0], loc[1])

		options = append(options, SimOption{
			Index: i + 1, // 1-based
			Label: label,
			Value: value,
			XPath: xpath,
		})
	}

	return options
}

// extractLabelForInput 提取 input 对应的 label 文本
//
// 优先级：
//   1. <label for="input_id">  → 精确匹配
//   2. <label> 包裹 input      → 父元素
//   3. <label> 紧邻 input     → 兄弟元素
//   4. <span> 在 input 附近    → 兜底
func (p *HTMLOptionParser) extractLabelForInput(block, inputTag string, inputStart, inputEnd int) string {
	// 1. 从 input 的 id 属性查找 for（(?s) 使 . 匹配换行，支持多行 label 内容）
	inputID := p.extractAttr(inputTag, "id")
	if inputID != "" {
		labelRe := regexp.MustCompile(
			`(?is)<label[^>]*for=["']?` + regexp.QuoteMeta(inputID) + `["']?[^>]*>([\s\S]*?)</label>`,
		)
		if match := labelRe.FindStringSubmatch(block); len(match) >= 2 {
			return p.extractTextFromLabel(match[1])
		}
	}

	// 2. 搜索范围：严格限制在当前 input 所在容器的附近
	// 避免跨越到其他选项的 label
	searchStart := inputStart - 300
	if searchStart < 0 {
		searchStart = 0
	}
	searchEnd := inputEnd + 300
	if searchEnd > len(block) {
		searchEnd = len(block)
	}
	nearBlock := block[searchStart:searchEnd]

	// 3. 尝试找包裹 input 的 <label>（支持多行内容）
	wrapperRe := regexp.MustCompile(`(?is)<label[^>]*>([\s\S]*?)</label>`)
	if wrapperMatches := wrapperRe.FindAllStringSubmatchIndex(nearBlock, -1); len(wrapperMatches) > 0 {
		for _, m := range wrapperMatches {
			if len(m) >= 4 {
				labelStart, labelEnd := m[2], m[3]
				// 检查 label 内容是否包含当前 input 的位置
				if labelStart <= inputStart && labelEnd >= inputEnd {
					labelContent := nearBlock[labelStart:labelEnd]
					// 检查 label 内是否包含这个 input
					if strings.Contains(labelContent, inputTag) {
						return p.extractTextFromLabel(labelContent)
					}
				}
			}
		}
	}

	// 4. 找紧邻 input 之前的 <label> 兄弟元素
	prevLabelRe := regexp.MustCompile(`(?i)</label>\s*<input[^>]*>`)
	prevLabelMatches := prevLabelRe.FindAllStringSubmatchIndex(nearBlock, -1)
	for _, m := range prevLabelMatches {
		if len(m) >= 4 && m[0] <= inputStart && m[1] <= inputStart+10 {
			// 找到了 label 结束到 input 开始之间距离很近的
			// 提取这个 label
			searchArea := nearBlock[:m[1]]
			labelFindRe := regexp.MustCompile(`(?i)<label[^>]*>(.*?)</label>`)
			if lastMatch := labelFindRe.FindAllStringSubmatchIndex(searchArea, -1); len(lastMatch) > 0 {
				last := lastMatch[len(lastMatch)-1]
				if len(last) >= 4 {
					labelContent := searchArea[last[2]:last[3]]
					return p.extractTextFromLabel(labelContent)
				}
			}
		}
	}

	// 5. 兜底：在更窄范围找 <span> 或文本
	spanRe := regexp.MustCompile(`(?i)<span[^>]*class=["']?[^"']*choice[^"']*["'][^>]*>(.*?)</span>`)
	if match := spanRe.FindStringSubmatch(nearBlock); len(match) >= 2 {
		return strings.TrimSpace(match[1])
	}

	// 6. 最终兜底：找任何 <span>
	spanRe2 := regexp.MustCompile(`(?i)<span[^>]*>(.*?)</span>`)
	if match := spanRe2.FindStringSubmatch(nearBlock); len(match) >= 2 {
		text := p.extractTextFromLabel(match[1])
		if text != "" {
			return text
		}
	}

	return ""
}

// extractTextFromLabel 从 label 标签内容中提取纯文本
func (p *HTMLOptionParser) extractTextFromLabel(labelContent string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	text := re.ReplaceAllString(labelContent, " ")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// parseMatrixOptions 解析矩阵题选项
func (p *HTMLOptionParser) parseMatrixOptions(block string, inputType string) []SimOption {
	var options []SimOption

	rowRe := regexp.MustCompile(`(?i)<tr[^>]*>(.*?)</tr>`)
	rows := rowRe.FindAllStringSubmatchIndex(block, -1)

	for i, loc := range rows {
		rowContent := block[loc[2]:loc[3]]
		cellRe := regexp.MustCompile(`(?i)<td[^>]*>(.*?)</td>`)
		cells := cellRe.FindAllStringSubmatch(rowContent, -1)
		if len(cells) == 0 {
			continue
		}

		rowLabel := p.extractTextFromLabel(cells[0][1])
		// 矩阵题 xpath 定位到对应的 tr
		xpath := p.generateMatrixRowXPath(loc[0], rowLabel)
		options = append(options, SimOption{
			Index: i + 1,
			Label: rowLabel,
			Value: rowLabel,
			XPath: xpath,
		})
	}

	return options
}

// extractQuestionBlock 提取第 qIndex 个问题的 HTML 片段
//
// 用括号计数法定位每个 <div class="...question..."> 的完整内部内容，
// 避免正则终止符消耗下一个标签、或非贪婪匹配因嵌套 </div> 提前终止的问题。
func (p *HTMLOptionParser) extractQuestionBlock(qIndex int) string {
	// 找出所有 question div 开始标签的位置
	startRe := regexp.MustCompile(`(?i)<div[^>]*class=["'][^"']*question[^"']*["'][^>]*>`)
	starts := startRe.FindAllStringIndex(p.rawHTML, -1)

	if qIndex >= len(starts) {
		return ""
	}

	tagEnd := starts[qIndex][1] // 开始标签结束位置（内容开始）

	// 括号计数：找到与该 <div> 匹配的 </div>
	depth := 1
	pos := tagEnd
	openRe := regexp.MustCompile(`(?i)<div[^>]*>`)
	closeRe := regexp.MustCompile(`(?i)</div>`)

	for depth > 0 && pos < len(p.rawHTML) {
		tail := p.rawHTML[pos:]
		nextOpenIdx := openRe.FindStringIndex(tail)
		nextCloseIdx := closeRe.FindStringIndex(tail)

		if nextCloseIdx == nil {
			break
		}

		closeAbs := pos + nextCloseIdx[0]
		openAbs := len(p.rawHTML) + 1
		if nextOpenIdx != nil {
			openAbs = pos + nextOpenIdx[0]
		}

		if openAbs < closeAbs {
			// 先遇到内层 <div>，深度加1
			depth++
			pos = pos + nextOpenIdx[1]
		} else {
			// 先遇到 </div>，深度减1
			depth--
			if depth == 0 {
				block := p.rawHTML[tagEnd:closeAbs]
				fmt.Printf("[DEBUG extractQuestionBlock] qIndex=%d, block长度=%d, 前80字符=%q\n",
					qIndex, len(block), truncateStr(block, 80))
				return block
			}
			pos = pos + nextCloseIdx[1]
		}
	}

	return ""
}

// extractAttr 提取 HTML 标签中的属性值
func (p *HTMLOptionParser) extractAttr(tag, attrName string) string {
	re := regexp.MustCompile(
		`(?i)` + attrName + `=["']([^"']*)["']`,
	)
	if match := re.FindStringSubmatch(tag); len(match) >= 2 {
		return match[1]
	}
	return ""
}

// inferQuestionTypeFromBlock 从 HTML 内容推断题型
func (p *HTMLOptionParser) inferQuestionTypeFromBlock(block string) string {
	if block == "" {
		return "unknown"
	}
	if regexp.MustCompile(`(?i)<input[^>]*type=["']?radio["']?`).MatchString(block) {
		return "single"
	}
	if regexp.MustCompile(`(?i)<input[^>]*type=["']?checkbox["']?`).MatchString(block) {
		return "multi"
	}
	if regexp.MustCompile(`(?i)<select`).MatchString(block) {
		return "dropdown"
	}
	if regexp.MustCompile(`(?i)<textarea`).MatchString(block) {
		return "text"
	}
	if regexp.MustCompile(`(?i)<input[^>]*type=["']?number["']?`).MatchString(block) {
		return "text"
	}
	return "unknown"
}

// normalizeQuestionType 标准化题型名称
func (p *HTMLOptionParser) normalizeQuestionType(qType string) string {
	switch strings.ToLower(qType) {
	case "single", "single_choice", "radio":
		return "single"
	case "multi", "multi_choice", "checkbox", "multiple", "multi_select":
		return "multi"
	case "text", "open", "textarea", "number", "input":
		return "text"
	case "matrix_radio", "matrix_single":
		return "matrix_radio"
	case "matrix_checkbox", "matrix_multi":
		return "matrix_checkbox"
	case "dropdown", "select":
		return "dropdown"
	case "ranking", "ranking_drag", "ranking_select":
		return "ranking"
	default:
		return "unknown"
	}
}

// mapRecommendedAnswer 将 AI 推荐答案映射到选项索引
func (p *HTMLOptionParser) mapRecommendedAnswer(q *SimQuestion, recommended interface{}) interface{} {
	if recommended == nil {
		return nil
	}
	// 文字题（无选项）直接透传推荐文本
	if len(q.Options) == 0 {
		return recommended
	}

	switch v := recommended.(type) {
	case string:
		// 单选题：label → index
		for i, opt := range q.Options {
			if strings.EqualFold(opt.Label, v) {
				return i + 1 // 1-based
			}
		}
		return v // 未匹配，返回原始文本

	case []interface{}:
		// 多选题：[]label → []index
		var indices []int
		for _, item := range v {
			if label, ok := item.(string); ok {
				for i, opt := range q.Options {
					if strings.EqualFold(opt.Label, label) {
						indices = append(indices, i+1)
						break
					}
				}
			}
		}
		return indices

	default:
		return nil
	}
}

// truncateStr 安全截取字符串
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// generateInputXPath 为 input 元素生成稳定 XPath
// 优先级：id > name > data-* > class > 标签+属性
func (p *HTMLOptionParser) generateInputXPath(inputTag string, start, end int) string {
	// 1. id 属性（最稳定）
	if id := p.extractAttr(inputTag, "id"); id != "" {
		return `//input[@id="` + id + `"]`
	}

	// 2. name 属性
	if name := p.extractAttr(inputTag, "name"); name != "" {
		inputType := p.extractAttr(inputTag, "type")
		if inputType != "" {
			return `//input[@name="` + name + `" and @type="` + inputType + `"]`
		}
		return `//input[@name="` + name + `"]`
	}

	// 3. data-* 属性
	for _, attr := range []string{"data-id", "data-key", "data-option", "data-value"} {
		if val := p.extractAttr(inputTag, attr); val != "" {
			return `//input[@` + attr + `="` + val + `"]`
		}
	}

	// 4. value 属性（有一定区分度）
	if val := p.extractAttr(inputTag, "value"); val != "" {
		return `//input[@value="` + val + `"]`
	}

	// 5. 回退：用索引在同级中定位（尽量往后取范围提高准确度）
	searchStart := start - 1000
	if searchStart < 0 {
		searchStart = 0
	}
	searchEnd := end + 1000
	if searchEnd > len(p.rawHTML) {
		searchEnd = len(p.rawHTML)
	}
	contextBlock := p.rawHTML[searchStart:searchEnd]
	inputType := p.extractAttr(inputTag, "type")
	typeFilter := ""
	if inputType != "" {
		typeFilter = `[@type="` + inputType + `"]`
	}
	// 在当前问题块内找所有同类型 input，定位当前是第几个
	re := regexp.MustCompile(`(?i)<input[^>]*type=["']?` + regexp.QuoteMeta(inputType) + `["']?[^>]*>`)
	allMatches := re.FindAllStringSubmatchIndex(contextBlock, -1)
	for i, m := range allMatches {
		matchTag := contextBlock[m[0]:m[1]]
		// 判断是否就是当前 input（用 value/id/name 辅助判断）
		matchID := p.extractAttr(matchTag, "id")
		matchName := p.extractAttr(matchTag, "name")
		matchValue := p.extractAttr(matchTag, "value")
		currID := p.extractAttr(inputTag, "id")
		currName := p.extractAttr(inputTag, "name")
		currValue := p.extractAttr(inputTag, "value")
		if matchID == currID && matchName == currName && matchValue == currValue {
			return `//input` + typeFilter + `[` + strconv.Itoa(i+1) + `]`
		}
	}
	// 兜底：直接用标签+type
	return `//input` + typeFilter
}

// generateMatrixRowXPath 为矩阵题的每一行生成 XPath
func (p *HTMLOptionParser) generateMatrixRowXPath(rowStart int, rowLabel string) string {
	// 优先用 row 的 id 或 data-* 属性
	searchStart := rowStart - 200
	if searchStart < 0 {
		searchStart = 0
	}
	searchEnd := rowStart + 300
	if searchEnd > len(p.rawHTML) {
		searchEnd = len(p.rawHTML)
	}
	contextBlock := p.rawHTML[searchStart:searchEnd]

	// 查找最近的 tr
	trRe := regexp.MustCompile(`(?i)<tr[^>]*>`)
	if trMatch := trRe.FindStringSubmatchIndex(contextBlock); len(trMatch) >= 2 {
		trTag := contextBlock[trMatch[0]:trMatch[1]]
		if id := p.extractAttr(trTag, "id"); id != "" {
			return `//tr[@id="` + id + `"]`
		}
		for _, attr := range []string{"data-row", "data-key", "data-id"} {
			if val := p.extractAttr(trTag, attr); val != "" {
				return `//tr[@` + attr + `="` + val + `"]`
			}
		}
	}

	// 兜底：用 rowLabel 文本定位
	escapedLabel := regexp.MustCompile(`['"\\]`).ReplaceAllString(rowLabel, "")
	if escapedLabel != "" {
		return `//tr[td[contains(text(),"` + escapedLabel + `")]]`
	}

	// 最终兜底
	return `//tr`
}
