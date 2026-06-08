package simulator

// text_generator.go — 填空题文本生成器
//
// 用于：
//   - 开放题文本生成（人名、地址、评论等）
//   - 按类型随机生成符合格式的文本

import (
	"fmt"
	"math/rand"
	"time"
)

// TextGenerator 文本生成器
type TextGenerator struct {
	rand *rand.Rand
}

// NewTextGenerator 创建生成器
func NewTextGenerator() *TextGenerator {
	return &TextGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateByType 根据题型生成文本
// @param qType 题型：text / number / email / phone / date 等
// @returns 生成的文本
func (g *TextGenerator) GenerateByType(qType string) string {
	switch qType {
	case "number":
		return g.generateNumber()
	case "email":
		return g.generateEmail()
	case "phone":
		return g.generatePhone()
	case "date":
		return g.generateDate()
	default:
		return g.generateGeneric()
	}
}

// generateNumber 生成随机数字
func (g *TextGenerator) generateNumber() string {
	digits := g.rand.Intn(5) + 1
	result := ""
	for i := 0; i < digits; i++ {
		result += fmt.Sprintf("%d", g.rand.Intn(10))
	}
	return result
}

// generateEmail 生成随机邮箱
func (g *TextGenerator) generateEmail() string {
	names := []string{"user", "test", "sample", "demo", "example"}
	domains := []string{"gmail.com", "outlook.com", "yahoo.com", "test.com"}
	name := names[g.rand.Intn(len(names))]
	num := g.rand.Intn(1000)
	domain := domains[g.rand.Intn(len(domains))]
	return fmt.Sprintf("%s%d@%s", name, num, domain)
}

// generatePhone 生成随机电话号码
func (g *TextGenerator) generatePhone() string {
	return fmt.Sprintf("1%d%09d", g.rand.Intn(9)+1, g.rand.Intn(1000000000))
}

// generateDate 生成随机日期
func (g *TextGenerator) generateDate() string {
	year := 1980 + g.rand.Intn(40)
	month := g.rand.Intn(12) + 1
	day := g.rand.Intn(28) + 1
	return fmt.Sprintf("%04d-%02d-%02d", year, month, day)
}

// generateGeneric 生成通用文本（短句）
func (g *TextGenerator) generateGeneric() string {
	sentences := []string{
		"Yes, I agree.",
		"This is a good point.",
		"I have no comments.",
		"Interesting survey.",
		"Thank you for your time.",
	}
	return sentences[g.rand.Intn(len(sentences))]
}
