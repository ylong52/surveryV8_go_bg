package extractor

// persona_loader.go — 从数据库加载人设信息
//   支持 persona_mode: "generic" 和 "fixed" 两种模式
//   输出格式：组装好的人设文本字符串，供 BuildYamlAnswerPrompt 使用

import (
	"fmt"
	"strings"

	"survey.cli/core/database"
)

// LoadPersona 从数据库加载默认人设，返回组装好的人设文本
// @returns  组装好的人设文本字符串，error
func LoadPersona() (string, error) {
	rec, err := database.LoadDefaultPersona()
	if err != nil {
		return "", fmt.Errorf("LoadPersona: %w", err)
	}
	return assemblePersona(rec), nil
}

// GetPersonaMode 返回当前默认 persona_mode
// @returns  persona_mode 字符串（fixed/generic/random）
func GetPersonaMode() (string, error) {
	rec, err := database.LoadDefaultPersona()
	if err != nil {
		return "", fmt.Errorf("GetPersonaMode: %w", err)
	}
	return rec.PersonaMode, nil
}

// assemblePersona 将 PersonaRecord 组装成可读的人设文本
// @param r  人设记录
// @returns  组装好的人设字符串
func assemblePersona(r *database.PersonaRecord) string {
	var sb strings.Builder

	sb.WriteString("【基本信息】\n")
	if r.AgeRange != "" {
		sb.WriteString(fmt.Sprintf("- 年龄: %s\n", r.AgeRange))
	}
	if r.Gender != "" {
		sb.WriteString(fmt.Sprintf("- 性别: %s\n", r.Gender))
	}
	if r.Race != "" {
		sb.WriteString(fmt.Sprintf("- 种族: %s\n", r.Race))
	}
	if r.Occupation != "" {
		sb.WriteString(fmt.Sprintf("- 职业: %s\n", r.Occupation))
	}
	if r.Income > 0 {
		sb.WriteString(fmt.Sprintf("- 年收入: %d\n", r.Income))
	}
	if r.Education != "" {
		sb.WriteString(fmt.Sprintf("- 学历: %s\n", r.Education))
	}
	if r.MaritalStatus != "" {
		sb.WriteString(fmt.Sprintf("- 婚姻状态: %s\n", r.MaritalStatus))
	}
	if r.HasChildren {
		sb.WriteString("- 子女: 有正在申请大学的孩子（16-18岁）\n")
	}

	if r.Description != "" {
		sb.WriteString("\n【背景描述】\n")
		sb.WriteString(r.Description)
		sb.WriteString("\n")
	}

	if len(r.OptionMapping) > 0 {
		sb.WriteString("\n【选项映射】（直接指定题目关键词 → 应选选项文本）\n")
		for key, value := range r.OptionMapping {
			sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
	}

	return sb.String()
}
