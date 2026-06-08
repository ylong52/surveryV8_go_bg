package simulator

// types.go — 答题模拟器核心数据结构

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// SimOption 单个选项
type SimOption struct {
	Index int    // 1-based 编号
	Label string // 选项文本
	Value string // 表单值（如 "0", "1", "male"）
	XPath string // 元素 XPath（用于前端点击定位）
}

// SimQuestion 单题
type SimQuestion struct {
	Index            int           // 题号（1-based）
	QuestionType     string        // single / multi / text / matrix_* / dropdown
	Subject          string        // 题目文本
	RecommendedIndex interface{}   // 推荐索引：int (单选题) / []int (多选题) / string (填空题)
	Options          []SimOption   // 选项列表（单选题/多选题有值）
	RawHTML          string        // 原始 HTML 片段
}

// SimAnswer 用户输入的答案
type SimAnswer struct {
	QuestionIndex   int      // 题号（0-based）
	SelectedIndex   int      // 单选题选中索引（1-based，基于当前题目的选项）
	SelectedIndices []int    // 多选题选中索引列表（1-based）
	TextValue       string   // 填空题文本
	SelectedOptID   int      // 单选题原始 opt_id（带前缀）
	SelectedOptIDs  []int    // 多选题原始 opt_id 列表（带前缀）
}

// SimResult 整次答题结果
type SimResult struct {
	SurveyID  string
	SessionID string
	PageID    string
	Answers   []SimAnswer
	AllFilled bool
	Timestamp time.Time
}

// FormatAnswer 将答案格式化为字符串
func FormatAnswer(q SimQuestion, a SimAnswer) string {
	switch q.QuestionType {
	case "single", "dropdown":
		if a.SelectedIndex >= 1 && a.SelectedIndex <= len(q.Options) {
			opt := q.Options[a.SelectedIndex-1]
			return opt.Label
		}
		return ""

	case "multi", "matrix_radio", "matrix_checkbox":
		var parts []string
		for _, idx := range a.SelectedIndices {
			if idx >= 1 && idx <= len(q.Options) {
				opt := q.Options[idx-1]
				parts = append(parts, opt.Label)
			}
		}
		return joinOr(parts)

	default:
		return a.TextValue
	}
}

// joinOr 用 " / " 连接
func joinOr(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " / " + parts[i]
	}
	return result
}

// =============================================================================
// opt_id 前缀计算（基于 question_id）
// q1 → 前缀 10，opt_id: 11, 12, 13...
// q2 → 前缀 20，opt_id: 21, 22, 23...
// q10 → 前缀 100，opt_id: 101, 102, 103...
// =============================================================================

// GetQuestionNumber 从 question_id 提取数字
// "q1" → 1, "q2" → 2, "q10" → 10, "q99" → 99
func GetQuestionNumber(questionID string) int {
	re := regexp.MustCompile(`^q(\d+)$`)
	m := re.FindStringSubmatch(strings.ToLower(questionID))
	if len(m) >= 2 {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// GetOptIDPrefix 计算 opt_id 前缀
// q1 → 10, q2 → 20, q10 → 100, q99 → 990
func GetOptIDPrefix(questionID string) int {
	n := GetQuestionNumber(questionID)
	return n * 10
}

// BuildOptID 构建 opt_id
// questionID="q1", index=1 → 11
// questionID="q2", index=2 → 22
func BuildOptID(questionID string, index int) int {
	return GetOptIDPrefix(questionID) + index
}

// ParseOptID 从 opt_id 提取索引
// opt_id=12 → index=2（假设前缀为10）
// opt_id=22 → index=2（假设前缀为20）
func ParseOptID(optID int) int {
	// 去掉十位和百位，只保留个位
	// 12 → 2, 22 → 2, 111 → 1
	return optID % 10
}

// OptIDPrefixMap opt_id 前缀映射表（用于展示）
var OptIDPrefixMap = map[string]int{
	"q1":  10,
	"q2":  20,
	"q3":  30,
	"q4":  40,
	"q5":  50,
	"q6":  60,
	"q7":  70,
	"q8":  80,
	"q9":  90,
	"q10": 100,
}
