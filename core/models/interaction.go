package models

import (
	"fmt"
	"strconv"
)

// QID 题目序号，支持从 JSON 整数或字符串反序列化
type QID struct {
	Value string
}

func (q *QID) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		// 字符串格式：直接去掉引号
		s = s[1 : len(s)-1]
	} else {
		// 整数格式：任意正整数转为 q{n}
		n, err := strconv.Atoi(string(data))
		if err == nil && n > 0 {
			s = fmt.Sprintf("q%d", n)
		} else {
			s = string(data)
		}
	}
	q.Value = s
	return nil
}

// Interaction 单个题目的推荐答案
type Interaction struct {
	QID                 QID         `yaml:"q_id" json:"question_id"`               // 题目序号（"q1","q2"...）
	Subject             string      `yaml:"subject" json:"subject"`
	RecommendedAnswer   interface{} `yaml:"recommended_answer" json:"recommended_answer"` // 推荐答案（string 或 []string）
	ShowAnswer          interface{} `yaml:"showAnswer" json:"showAnswer"`                   // 旧版 AI 兼容
	QuestionType        string      `yaml:"questionType" json:"questionType"`               // 题型（camelCase）
	QuestionTypeAlt     string      `yaml:"question_type" json:"question_type"`             // 题型（snake_case）
	WordLimit           WordLimit   `yaml:"wordLimit" json:"wordLimit"`
	HumansPass          *bool       `yaml:"-" json:"-"`
	PromptSuggestionNote string     `yaml:"-" json:"-"`
}

// WordLimit 开放题字数限制
type WordLimit struct {
	Min int `yaml:"min" json:"min"`
	Max int `yaml:"max" json:"max"`
}
