package extractor

// prompt_builder.go — 读取 configs/prompts.yaml，按 assembly_order.analyze 顺序组装 AI Prompt

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AssembledPrompt 包含组装完成的提示词
//   - System: 系统提示词
//   - User: 用户提示词（日志用，含文件路径）
//   - UserForAI: 用户提示词（AI 用，含完整 HTML）
type AssembledPrompt struct {
	System    string
	User      string      // 日志用：含文件路径
	UserForAI string      // AI 用：含完整 HTML
}

const promptsYAMLPath = "configs/prompts.yaml"

// promptsConfig 对应 configs/prompts.yaml 的相关字段
type promptsConfig struct {
	System        systemPrompts    `yaml:"system"`
	CoreRules     string           `yaml:"core_rules"`
	QuestionTypes questionTypes    `yaml:"question_types"`
	Structure     structurePrompts `yaml:"structure"`
	AssemblyOrder assemblyOrder    `yaml:"assembly_order"`
}

type systemPrompts struct {
	Analyze string `yaml:"analyze"`
	Role    string `yaml:"role"`
}

type questionTypes struct {
	Choice       choiceTypes   `yaml:"choice"`
	Matrix       matrixTypes   `yaml:"matrix"`
	Email        string        `yaml:"email"`
	Phone        string        `yaml:"phone"`
	Number       string        `yaml:"number"`
	Date         string        `yaml:"date"`
	Time         string        `yaml:"time"`
	OpenEnd      string        `yaml:"open_end"`
	OpenText     string        `yaml:"open_text"`
	SemanticDiff string        `yaml:"semantic_diff"`
	Slider       string        `yaml:"slider"`
	NPS          string        `yaml:"nps"`
	Likert       string        `yaml:"likert"`
	LikertCard   string        `yaml:"likert_card"`
	MultiSelect  string        `yaml:"multi_select"`
	Ranking      rankingTypes  `yaml:"ranking"`
	MaxDiff      string        `yaml:"max_diff"`
	Conjoint     string        `yaml:"conjoint"`
	Cardsort     cardsortTypes `yaml:"cardsort"`
	ImageSelect  string        `yaml:"image_select"`
	Heatmap      string        `yaml:"heatmap"`
}

type choiceTypes struct {
	Single   string `yaml:"single"`
	Multiple string `yaml:"multiple"`
	Select   string `yaml:"select"`
}

type matrixTypes struct {
	MatrixRadio    string `yaml:"matrix_radio"`
	MatrixCheckbox string `yaml:"matrix_checkbox"`
}

type rankingTypes struct {
	RankingDrag   string `yaml:"ranking_drag"`
	RankingSelect string `yaml:"ranking_select"`
}

type cardsortTypes struct {
	CardsortDrag   string `yaml:"cardsort_drag"`
	CardsortSelect string `yaml:"cardsort_select"`
}

type structurePrompts struct {
	Grouping     string `yaml:"grouping"`
	NavButtons   string `yaml:"nav_buttons"`
	OutputFormat string `yaml:"output_format"`
}

type assemblyOrder struct {
	Analyze []string `yaml:"analyze"`
}

// blockByKey 将 assembly_order 中的 camelCase key 映射到对应的提示词文本
// @param key  assembly_order.analyze 中的 key 名
// @returns    对应提示词文本，未匹配则返回 ""
func (p *promptsConfig) blockByKey(key string) string {
	switch key {
	case "role":
		return p.System.Role
	case "structureGrouping":
		return p.Structure.Grouping
	case "coreRules":
		return p.CoreRules
	case "single":
		return p.QuestionTypes.Choice.Single
	case "multiple":
		return p.QuestionTypes.Choice.Multiple
	case "select":
		return p.QuestionTypes.Choice.Select
	case "openText":
		return p.QuestionTypes.OpenText
	case "emailInput":
		return p.QuestionTypes.Email
	case "phoneInput":
		return p.QuestionTypes.Phone
	case "numberInput":
		return p.QuestionTypes.Number
	case "dateInput":
		return p.QuestionTypes.Date
	case "timeInput":
		return p.QuestionTypes.Time
	case "openEnd":
		return p.QuestionTypes.OpenEnd
	case "matrixRadio":
		return p.QuestionTypes.Matrix.MatrixRadio
	case "matrixCheckbox":
		return p.QuestionTypes.Matrix.MatrixCheckbox
	case "likert":
		return p.QuestionTypes.Likert
	case "semanticDiff":
		return p.QuestionTypes.SemanticDiff
	case "slider":
		return p.QuestionTypes.Slider
	case "nps":
		return p.QuestionTypes.NPS
	case "likertCard":
		return p.QuestionTypes.LikertCard
	case "multiSelect":
		return p.QuestionTypes.MultiSelect
	case "rankingDrag":
		return p.QuestionTypes.Ranking.RankingDrag
	case "rankingSelect":
		return p.QuestionTypes.Ranking.RankingSelect
	case "maxDiff":
		return p.QuestionTypes.MaxDiff
	case "conjoint":
		return p.QuestionTypes.Conjoint
	case "cardsortDrag":
		return p.QuestionTypes.Cardsort.CardsortDrag
	case "cardsortSelect":
		return p.QuestionTypes.Cardsort.CardsortSelect
	case "imageSelect":
		return p.QuestionTypes.ImageSelect
	case "heatmap":
		return p.QuestionTypes.Heatmap
	case "navButtons":
		return p.Structure.NavButtons
	case "outputFormat":
		return p.Structure.OutputFormat
	default:
		return ""
	}
}

// buildPrompt 读取 configs/prompts.yaml，按 assembly_order.analyze 顺序组装完整提示词
// @param surveyID  问卷 ID
// @param pageID    页面 ID（已清洗）
// @param cleanHTML 清洗后的 HTML（用于提取 JS 元数据）
// @returns         AssembledPrompt（含两个版本的 User Prompt），或错误
func buildPrompt(surveyID, pageID, cleanHTML string) (AssembledPrompt, error) {
	data, err := os.ReadFile(promptsYAMLPath)
	if err != nil {
		return AssembledPrompt{}, fmt.Errorf("buildPrompt: read %s: %w", promptsYAMLPath, err)
	}

	var cfg promptsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return AssembledPrompt{}, fmt.Errorf("buildPrompt: parse yaml: %w", err)
	}

	// system prompt: system.analyze 作为开头，然后按 assembly_order.analyze 顺序追加各 block
	sysBlocks := []string{cfg.System.Analyze}
	for _, key := range cfg.AssemblyOrder.Analyze {
		if block := cfg.blockByKey(key); block != "" {
			sysBlocks = append(sysBlocks, block)
		}
	}
	systemPrompt := strings.Join(sysBlocks, "\n")

	// user prompt for log: HTML 文件路径（可读性好）
	logUserPrompt := buildUserPrompt(surveyID, pageID, cleanHTML, true)
	// user prompt for AI: HTML 完整内容（AI 可处理）
	aiUserPrompt := buildUserPrompt(surveyID, pageID, cleanHTML, false)

	return AssembledPrompt{System: systemPrompt, User: logUserPrompt, UserForAI: aiUserPrompt}, nil
}

// buildUserPrompt 组装 User Prompt
// @param forLog  true=日志用（含文件路径），false=AI用（含完整HTML）
func buildUserPrompt(surveyID, pageID, cleanHTML string, forLog bool) string {
	var htmlContent string
	if forLog {
		// 日志用：显示文件路径，方便阅读
		htmlContent = fmt.Sprintf("cache/%s/%s.html", surveyID, pageID)
	} else {
		// AI 用：显示完整 HTML 内容
		htmlContent = cleanHTML
	}

	userLines := []string{
		"【题目结构（从下面提取成YAML格式）】",
		htmlContent,
	}
	return strings.Join(userLines, "\n")
}
