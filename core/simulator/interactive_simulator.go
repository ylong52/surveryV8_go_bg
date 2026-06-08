package simulator

// interactive_simulator.go — 答题展示引擎
//
// 功能：
//   1. 展示所有题目 + 选项（opt_id 带前缀：q1→11,12,13 / q2→21,22,23...）
//   2. 用户直接输入 opt_id 进行选择
//   3. multi 题：输入 opt_id 切换选择，输入 s 提交整页
//   4. 返回 SimResult 供持久化

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// InteractiveSimulator 答题展示器
type InteractiveSimulator struct {
	SurveyID   string
	SessionID  string
	Questions  []SimQuestion
	PageID     string
	reader     *bufio.Reader
	pageAnswers             []SimAnswer // 当前页的所有答案
	isMultiMode            bool        // 是否为多选模式
	currentMultiSelections []int       // 多选题当前选中的 opt_id 列表
	debugMode              bool
}

// NewInteractiveSimulator 创建模拟器
func NewInteractiveSimulator(surveyID, sessionID string, questions []SimQuestion, pageID string) *InteractiveSimulator {
	return &InteractiveSimulator{
		SurveyID:  surveyID,
		SessionID: sessionID,
		Questions: questions,
		PageID:    pageID,
		reader:    bufio.NewReader(os.Stdin),
	}
}

// SetDebugMode 设置调试模式
func (s *InteractiveSimulator) SetDebugMode(enabled bool) {
	s.debugMode = enabled
}

// Run 展示所有题目，用户交互选择，最终提交整页答案
// @returns SimResult 答题结果
func (s *InteractiveSimulator) Run() (*SimResult, error) {
	s.pageAnswers = make([]SimAnswer, len(s.Questions))

	s.printHeader()
	s.printAll()

	// 提交整页答案
	s.submitPage()

	return &SimResult{
		SurveyID:  s.SurveyID,
		SessionID: s.SessionID,
		PageID:    s.PageID,
		Answers:   s.pageAnswers,
		AllFilled: true,
		Timestamp: time.Now(),
	}, nil
}

// printHeader 打印页面头部
func (s *InteractiveSimulator) printHeader() {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  调试信息: [开启]")
	fmt.Println("========================================")
	fmt.Println()
}

// printAll 展示所有题目并收集用户输入
func (s *InteractiveSimulator) printAll() {
	total := len(s.Questions)
	for i, q := range s.Questions {
		qID := "q" + strconv.Itoa(i+1)
		optPrefix := GetOptIDPrefix(qID)

		fmt.Println("────────────────────────────────────────")
		fmt.Printf("[题目 %d/%d] %s\n", i+1, total, q.Subject)
		fmt.Printf("  类型: %s\n", questionTypeCN(q.QuestionType))

		// 单选/矩阵单选：在此直接打印选项；多选题由 handleMultiChoice→printMultiSelections 打印
		if len(q.Options) > 0 && q.QuestionType != "multi" && q.QuestionType != "matrix_checkbox" {
			fmt.Println("  ─────────────────────")
			for j, opt := range q.Options {
				optID := optPrefix + j + 1
				fmt.Printf("  %d) %s\n", optID, opt.Label)
			}
			fmt.Println("  ─────────────────────")
		}

		// 根据题型处理交互
		switch q.QuestionType {
		case "single", "dropdown", "matrix_radio":
			s.handleSingleChoice(i, q, optPrefix)
		case "multi", "matrix_checkbox":
			s.handleMultiChoice(i, q, optPrefix)
		default:
			s.handleTextInput(i, q)
		}

		fmt.Println()
	}
}

// handleSingleChoice 处理单选题用户输入
func (s *InteractiveSimulator) handleSingleChoice(qIndex int, q SimQuestion, optPrefix int) {
	// 显示推荐答案
	recOptID := s.getRecommendedOptID(q, optPrefix)
	if recOptID > 0 {
		fmt.Printf("  推荐答案: %d\n", recOptID)
	}

	fmt.Printf("  ▶ 请输入 opt_id: ")

	for {
		input, _ := s.reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			// 空输入，采纳推荐答案
			if recOptID > 0 {
				s.submitSingleAnswer(qIndex, q, optPrefix, recOptID)
			}
			break
		}

		optID, err := strconv.Atoi(input)
		if err != nil || optID < optPrefix+1 || optID > optPrefix+len(q.Options) {
			fmt.Printf("  无效输入，请输入 %d-%d 之间的 opt_id: ", optPrefix+1, optPrefix+len(q.Options))
			continue
		}

		s.submitSingleAnswer(qIndex, q, optPrefix, optID)
		break
	}
}

// submitSingleAnswer 提交单选题答案
func (s *InteractiveSimulator) submitSingleAnswer(qIndex int, q SimQuestion, optPrefix int, optID int) {
	selectedIndex := optID - optPrefix // optID 转换为 1-based 索引

	if s.debugMode {
		fmt.Printf("\n[调试] 点击 xpath: %s\n", s.getXPathByOptID(q, optID))
		fmt.Printf("[调试] 当前答案: { \"question_id\": \"q%d\", \"selected\": [%d] }\n", qIndex+1, optID)
	}

	// 保存答案
	s.pageAnswers[qIndex] = SimAnswer{
		QuestionIndex: qIndex,
		SelectedIndex: selectedIndex,
		SelectedOptID: optID, // 记录原始 opt_id
	}

	if s.debugMode {
		fmt.Printf("[调试] 提交到后端: { \"question_id\": \"q%d\", \"selected\": [%d] }\n", qIndex+1, optID)
		fmt.Println("✅ 前端已提交")
	}
}

// handleMultiChoice 处理多选题用户输入
func (s *InteractiveSimulator) handleMultiChoice(qIndex int, q SimQuestion, optPrefix int) {
	s.currentMultiSelections = []int{}
	s.isMultiMode = true

	// 收集推荐 opt_id 列表（RecommendedIndex 为 []int，元素为 1-based 选项位置）
	var recOptIDs []int
	if q.RecommendedIndex != nil {
		if idxs, ok := q.RecommendedIndex.([]int); ok {
			for _, idx := range idxs {
				if idx >= 1 && idx <= len(q.Options) {
					recOptIDs = append(recOptIDs, optPrefix+idx)
				}
			}
		}
	}
	if len(recOptIDs) > 0 {
		parts := make([]string, len(recOptIDs))
		for i, id := range recOptIDs {
			parts[i] = strconv.Itoa(id)
		}
		fmt.Printf("  推荐答案: %s\n", strings.Join(parts, ", "))
	}

	for {
		if len(s.currentMultiSelections) > 0 {
			fmt.Printf("  ▶ 请输入 opt_id (输入 s 提交): ")
		} else {
			fmt.Printf("  ▶ 请输入 opt_id (选完输入 s 提交，回车采纳推荐): ")
		}

		input, _ := s.reader.ReadString('\n')
		input = strings.TrimSpace(input)

		// 空回车且无手动选择 → 采纳推荐答案自动提交
		if input == "" && len(s.currentMultiSelections) == 0 {
			if len(recOptIDs) > 0 {
				s.currentMultiSelections = recOptIDs
			}
			s.submitMultiAnswer(qIndex, q, optPrefix)
			break
		}

		if strings.ToLower(input) == "s" {
			s.submitMultiAnswer(qIndex, q, optPrefix)
			break
		}

		optID, err := strconv.Atoi(input)
		if err != nil || optID < optPrefix+1 || optID > optPrefix+len(q.Options) {
			fmt.Printf("  无效输入，请输入 %d-%d 之间的 opt_id，或输入 s 提交\n", optPrefix+1, optPrefix+len(q.Options))
			continue
		}

		// 切换选择状态
		s.toggleMultiSelection(optID)
		s.printMultiSelections(q, optPrefix)
	}
}

// printMultiSelections 打印多选题当前选中状态
func (s *InteractiveSimulator) printMultiSelections(q SimQuestion, optPrefix int) {
	fmt.Println("  ─────────────────────")
	for j, opt := range q.Options {
		optID := optPrefix + j + 1
		selected := ""
		for _, sel := range s.currentMultiSelections {
			if sel == optID {
				selected = " [x]"
				break
			}
		}
		fmt.Printf("  %d) %s%s\n", optID, opt.Label, selected)
	}
	fmt.Println("  ─────────────────────")
}

// toggleMultiSelection 切换多选选中状态
func (s *InteractiveSimulator) toggleMultiSelection(optID int) {
	// 检查是否已选中
	for i, sel := range s.currentMultiSelections {
		if sel == optID {
			// 取消选中
			s.currentMultiSelections = append(s.currentMultiSelections[:i], s.currentMultiSelections[i+1:]...)
			if s.debugMode {
				fmt.Printf("\n[调试] 取消选择 opt_id: %d\n", optID)
			}
			return
		}
	}
	// 添加选中
	s.currentMultiSelections = append(s.currentMultiSelections, optID)
	if s.debugMode {
		fmt.Printf("\n[调试] 选中 opt_id: %d\n", optID)
	}
}

// submitMultiAnswer 提交多选题答案
func (s *InteractiveSimulator) submitMultiAnswer(qIndex int, q SimQuestion, optPrefix int) {
	if len(s.currentMultiSelections) == 0 {
		fmt.Println("  ⚠️ 未选择任何选项")
	}

	if s.debugMode {
		selectedStr := ""
		for i, sel := range s.currentMultiSelections {
			if i > 0 {
				selectedStr += ", "
			}
			selectedStr += strconv.Itoa(sel)
		}
		fmt.Printf("\n[调试] 提交到后端: { \"question_id\": \"q%d\", \"selected\": [%s] }\n", qIndex+1, selectedStr)
		fmt.Println("✅ 前端已提交")
	}

	// 转换为 1-based 索引列表
	selectedIndices := make([]int, len(s.currentMultiSelections))
	for i, optID := range s.currentMultiSelections {
		selectedIndices[i] = optID - optPrefix
	}

	// 保存答案
	s.pageAnswers[qIndex] = SimAnswer{
		QuestionIndex:   qIndex,
		SelectedIndices: selectedIndices,
		SelectedOptIDs:  s.currentMultiSelections, // 记录原始 opt_id 列表
	}

	s.isMultiMode = false
	s.currentMultiSelections = nil
}

// handleTextInput 处理填空题用户输入
func (s *InteractiveSimulator) handleTextInput(qIndex int, q SimQuestion) {
	// 提取推荐文本（string 或 []interface{} 取第一项）
	recText := ""
	if q.RecommendedIndex != nil {
		switch v := q.RecommendedIndex.(type) {
		case string:
			recText = v
		case []interface{}:
			if len(v) > 0 {
				if str, ok := v[0].(string); ok {
					recText = str
				}
			}
		}
	}

	if recText != "" {
		fmt.Printf("  推荐答案: %s\n", recText)
	}
	fmt.Printf("  ▶ 请输入答案: ")

	input, _ := s.reader.ReadString('\n')
	input = strings.TrimSpace(input)

	// 空回车采纳推荐答案
	if input == "" && recText != "" {
		input = recText
	}

	if s.debugMode {
		fmt.Printf("\n[调试] 提交到后端: { \"question_id\": \"q%d\", \"text\": \"%s\" }\n", qIndex+1, input)
		fmt.Println("✅ 前端已提交")
	}

	s.pageAnswers[qIndex] = SimAnswer{
		QuestionIndex: qIndex,
		TextValue:     input,
	}
}

// submitPage 提交整页答案到后端
func (s *InteractiveSimulator) submitPage() {
	if !s.debugMode {
		return
	}

	fmt.Println()
	fmt.Println("────────────────────────────────────────")
	fmt.Println("[调试] 前端提交本页全部答案:")

	for i, ans := range s.pageAnswers {
		q := s.Questions[i]
		qID := "q" + strconv.Itoa(i+1)

		switch q.QuestionType {
		case "single", "dropdown", "matrix_radio":
			if ans.SelectedOptID > 0 {
				fmt.Printf("{ \"question_id\": \"%s\", \"selected\": [%d] }\n", qID, ans.SelectedOptID)
			}
		case "multi", "matrix_checkbox":
			if len(ans.SelectedOptIDs) > 0 {
				parts := make([]string, len(ans.SelectedOptIDs))
				for j, optID := range ans.SelectedOptIDs {
					parts[j] = strconv.Itoa(optID)
				}
				fmt.Printf("{ \"question_id\": \"%s\", \"selected\": [%s] }\n", qID, strings.Join(parts, ", "))
			}
		default:
			if ans.TextValue != "" {
				fmt.Printf("{ \"question_id\": \"%s\", \"text\": \"%s\" }\n", qID, ans.TextValue)
			}
		}
	}

	fmt.Println("────────────────────────────────────────")
	fmt.Println()
	fmt.Println("[调试] 后端确认（一页答案）:")
	s.printBackendConfirmation()
}

// printBackendConfirmation 打印后端确认的答案
func (s *InteractiveSimulator) printBackendConfirmation() {
	fmt.Println("{")
	for i, ans := range s.pageAnswers {
		q := s.Questions[i]
		qID := "q" + strconv.Itoa(i+1)
		optPrefix := GetOptIDPrefix(qID)

		if i > 0 {
			fmt.Println(",")
		}

		switch q.QuestionType {
		case "single", "dropdown", "matrix_radio":
			if ans.SelectedOptID > 0 {
				optIndex := ans.SelectedOptID - optPrefix - 1
				if optIndex >= 0 && optIndex < len(q.Options) {
					label := q.Options[optIndex].Label
					xpath := q.Options[optIndex].XPath
					fmt.Printf("  \"question_id\": \"%s\",\n", qID)
					fmt.Printf("  \"question_type\": \"%s\",\n", q.QuestionType)
					fmt.Printf("  \"answer\": {\n")
					fmt.Printf("    \"label\": \"%s\",\n", label)
					fmt.Printf("    \"opt_id\": %d,\n", ans.SelectedOptID)
					fmt.Printf("    \"xpath\": \"%s\"\n", xpath)
					fmt.Printf("  }")
				}
			}
		case "multi", "matrix_checkbox":
			if len(ans.SelectedOptIDs) > 0 {
				fmt.Printf("  \"question_id\": \"%s\",\n", qID)
				fmt.Printf("  \"question_type\": \"%s\",\n", q.QuestionType)
				fmt.Printf("  \"answer\": [\n")
				for j, optID := range ans.SelectedOptIDs {
					optIndex := optID - optPrefix - 1
					if optIndex >= 0 && optIndex < len(q.Options) {
						label := q.Options[optIndex].Label
						xpath := q.Options[optIndex].XPath
						if j > 0 {
							fmt.Printf(",\n")
						}
						fmt.Printf("    { \"label\": \"%s\", \"opt_id\": %d, \"xpath\": \"%s\" }", label, optID, xpath)
					}
				}
				fmt.Printf("\n  ]")
			}
		default:
			fmt.Printf("  \"question_id\": \"%s\",\n", qID)
			fmt.Printf("  \"question_type\": \"%s\",\n", q.QuestionType)
			fmt.Printf("  \"answer\": \"%s\"", ans.TextValue)
		}
	}
	fmt.Println()
	fmt.Println("}")
}

// getRecommendedOptID 获取推荐答案的 opt_id
func (s *InteractiveSimulator) getRecommendedOptID(q SimQuestion, optPrefix int) int {
	if q.RecommendedIndex == nil {
		return 0
	}

	switch v := q.RecommendedIndex.(type) {
	case int:
		// 单选题推荐索引 → opt_id
		if v >= 1 && v <= len(q.Options) {
			return optPrefix + v
		}
	case []int:
		// 多选题推荐索引 → 第一个选项的 opt_id
		if len(v) > 0 && v[0] >= 1 && v[0] <= len(q.Options) {
			return optPrefix + v[0]
		}
	}
	return 0
}

// getXPathByOptID 根据 opt_id 获取 XPath
func (s *InteractiveSimulator) getXPathByOptID(q SimQuestion, optID int) string {
	qID := "q" + strconv.Itoa(q.Index)
	optPrefix := GetOptIDPrefix(qID)
	optIndex := optID - optPrefix - 1 // opt_id → 0-based 索引

	if optIndex >= 0 && optIndex < len(q.Options) {
		return q.Options[optIndex].XPath
	}
	return ""
}

// questionTypeCN 返回题型中文标签
func questionTypeCN(qType string) string {
	switch qType {
	case "single":
		return "单选题"
	case "multi":
		return "多选题"
	case "text":
		return "填空题"
	case "matrix_radio":
		return "矩阵单选题"
	case "matrix_checkbox":
		return "矩阵多选题"
	case "dropdown":
		return "下拉题"
	case "ranking":
		return "排序题"
	default:
		return "未知"
	}
}
