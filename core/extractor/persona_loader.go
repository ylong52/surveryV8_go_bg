package extractor

// persona_loader.go — 从 configs/prompts.yaml 加载人设信息
//   支持 persona_mode: "generic" 和 "fixed" 两种模式
//   输出格式：组装好的人设文本字符串，供 BuildYamlAnswerPrompt 使用

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// promptsPersonaConfig 对应 configs/prompts.yaml 的人设相关字段
type promptsPersonaConfig struct {
	PersonaMode   string         `yaml:"persona_mode"`
	PersonaFixed  personaBlock   `yaml:"persona_fixed"`
	PersonaGeneric personaBlock  `yaml:"persona_generic"`
}

// personaBlock 人设节点（fixed 和 generic 共用同一结构）
type personaBlock struct {
	Age          interface{}       `yaml:"age"`
	Gender       string            `yaml:"gender"`
	Race        string            `yaml:"race"`
	Occupation  string            `yaml:"occupation"`
	Income      interface{}       `yaml:"income"`
	Education   string            `yaml:"education"`
	HasFamily   interface{}       `yaml:"has_family"`
	HasChildren interface{}       `yaml:"has_children"`
	MaritalStatus string          `yaml:"marital_status"`
	Description string            `yaml:"description"`
	OptionMapping map[string]string `yaml:"option_mapping"`
}

// LoadPersona 从 prompts.yaml 加载人设信息
//   根据 persona_mode 返回对应的人设文本
//   支持 "generic" 和 "fixed" 两种模式
// @returns  组装好的人设文本字符串，error
func LoadPersona() (string, error) {
	data, err := os.ReadFile(promptsYAMLPath)
	if err != nil {
		return "", fmt.Errorf("LoadPersona: read %s: %w", promptsYAMLPath, err)
	}

	var cfg promptsPersonaConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("LoadPersona: parse yaml: %w", err)
	}

	switch cfg.PersonaMode {
	case "fixed":
		return assemblePersona(cfg.PersonaFixed), nil
	case "generic":
		return assemblePersona(cfg.PersonaGeneric), nil
	default:
		return "", fmt.Errorf("LoadPersona: unsupported persona_mode: %s (supported: fixed, generic)", cfg.PersonaMode)
	}
}

// assemblePersona 将 personaBlock 组装成可读的人设文本
// @param block  人设数据块
// @returns      组装好的人设字符串
func assemblePersona(block personaBlock) string {
	var sb strings.Builder

	sb.WriteString("【基本信息】\n")
	if block.Age != nil {
		sb.WriteString(fmt.Sprintf("- 年龄: %v\n", block.Age))
	}
	if block.Gender != "" {
		sb.WriteString(fmt.Sprintf("- 性别: %s\n", block.Gender))
	}
	if block.Race != "" {
		sb.WriteString(fmt.Sprintf("- 种族: %s\n", block.Race))
	}
	if block.Occupation != "" {
		sb.WriteString(fmt.Sprintf("- 职业: %s\n", block.Occupation))
	}
	if block.Income != nil {
		sb.WriteString(fmt.Sprintf("- 年收入: %v\n", block.Income))
	}
	if block.Education != "" {
		sb.WriteString(fmt.Sprintf("- 学历: %s\n", block.Education))
	}
	if block.MaritalStatus != "" {
		sb.WriteString(fmt.Sprintf("- 婚姻状态: %s\n", block.MaritalStatus))
	}
	if block.HasFamily != nil {
		if b, ok := block.HasFamily.(bool); ok && b {
			sb.WriteString("- 家庭伴侣: 有\n")
		}
	}
	if block.HasChildren != nil {
		if b, ok := block.HasChildren.(bool); ok && b {
			sb.WriteString("- 子女: 有正在申请大学的孩子（16-18岁）\n")
		}
	}

	if block.Description != "" {
		sb.WriteString("\n【背景描述】\n")
		sb.WriteString(block.Description)
		sb.WriteString("\n")
	}

	if len(block.OptionMapping) > 0 {
		sb.WriteString("\n【选项映射】（直接指定题目关键词 → 应选选项文本）\n")
		for key, value := range block.OptionMapping {
			sb.WriteString(fmt.Sprintf("%s: %s\n", key, value))
		}
	}

	return sb.String()
}

// GetPersonaMode 返回当前 persona_mode
// @returns  persona_mode 字符串（fixed/generic/random）
func GetPersonaMode() (string, error) {
	data, err := os.ReadFile(promptsYAMLPath)
	if err != nil {
		return "", fmt.Errorf("GetPersonaMode: read %s: %w", promptsYAMLPath, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("GetPersonaMode: parse yaml: %w", err)
	}

	if mode, ok := raw["persona_mode"].(string); ok {
		return mode, nil
	}
	return "", fmt.Errorf("GetPersonaMode: persona_mode not found")
}
