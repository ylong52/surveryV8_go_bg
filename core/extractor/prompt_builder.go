package extractor

// prompt_builder.go — 从数据库查询提示词，按 assembly_order.analyze 顺序组装 AI Prompt

import (
	"fmt"
	"strings"

	"survey.cli/core/database"
)

// AssembledPrompt 包含组装完成的提示词
//   - System: 系统提示词
//   - User: 用户提示词（日志用，含文件路径）
//   - UserForAI: 用户提示词（AI 用，含完整 HTML）
type AssembledPrompt struct {
	System    string
	User      string    // 日志用：含文件路径
	UserForAI string    // AI 用：含完整 HTML
}

// buildPrompt 从数据库加载提示词，按 assembly_order.analyze 顺序组装完整提示词
// @param surveyID  问卷 ID（用于加载 per-survey 覆盖提示词）
// @param pageID    页面 ID
// @param cleanHTML 清洗后的 HTML
// @returns         AssembledPrompt，error
func buildPrompt(surveyID, pageID, cleanHTML string) (AssembledPrompt, error) {
	// 加载所有活跃提示词
	prompts, err := database.LoadAllPrompts()
	if err != nil {
		return AssembledPrompt{}, fmt.Errorf("buildPrompt: load prompts: %w", err)
	}

	// 加载 survey 覆盖并合并（覆盖优先）
	overrides, err := database.LoadSurveyOverrides(surveyID)
	if err != nil {
		return AssembledPrompt{}, fmt.Errorf("buildPrompt: load overrides: %w", err)
	}
	for k, v := range overrides {
		prompts[k] = v
	}

	// 加载组装顺序
	assemblyKeys, err := database.LoadAssemblyOrder("analyze")
	if err != nil {
		return AssembledPrompt{}, fmt.Errorf("buildPrompt: load assembly order: %w", err)
	}

	// system prompt：system.analyze 作为开头，然后按顺序追加各 block
	sysBlocks := []string{prompts["system.analyze"]}
	for _, key := range assemblyKeys {
		if block := prompts[key]; block != "" {
			sysBlocks = append(sysBlocks, block)
		}
	}
	systemPrompt := strings.Join(sysBlocks, "\n")

	logUserPrompt := buildUserPrompt(surveyID, pageID, cleanHTML, true)
	aiUserPrompt := buildUserPrompt(surveyID, pageID, cleanHTML, false)

	return AssembledPrompt{System: systemPrompt, User: logUserPrompt, UserForAI: aiUserPrompt}, nil
}

// buildUserPrompt 组装 User Prompt
// @param forLog  true=日志用（含文件路径），false=AI用（含完整HTML）
func buildUserPrompt(surveyID, pageID, cleanHTML string, forLog bool) string {
	var htmlContent string
	if forLog {
		htmlContent = fmt.Sprintf("cache/%s/%s.html", surveyID, pageID)
	} else {
		htmlContent = cleanHTML
	}

	userLines := []string{
		"【题目结构（从下面提取成YAML格式）】",
		htmlContent,
	}
	return strings.Join(userLines, "\n")
}
