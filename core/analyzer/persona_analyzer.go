package analyzer

// persona_analyzer.go — 人设冲突分析核心逻辑
//
// 主入口：Analyze(surveyID int) (reportPath string, error)
//   1. 查询 survey 表获取会话信息
//   2. 从 page_results 加载所有答题记录（含 OCR 字段）
//   3. 从 survey_persona 加载人设（无则取 __default__）
//   4. 解析 recommend_answer 与 ocr_json，提取每题交互数据
//   5. 根据 OCR 数据构建 user_persona 快照
//   6. 从 prompts 表加载 session.persona_analysis 提示词
//   7. 拼装 YAML 数据，调用 Claude 生成报告
//   8. 保存报告到 storage/analysis_logs/{session_name}_report.md

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"survey.cli/core/database"
)

// personaRelevanceKeywords 人设相关性关键词：subject 含以下词则为"关键"，否则"无关"
var personaRelevanceKeywords = []string{
	"age", "gender", "country", "zip", "hispanic",
	"racial", "ethnic", "race", "children", "child",
	"income", "marital", "education", "occupation", "employment",
}

// containsPersonaKeyword 判断 subject 是否含人设相关关键词
func containsPersonaKeyword(subject string) bool {
	lower := strings.ToLower(subject)
	for _, kw := range personaRelevanceKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// =============================================================================
// 数据结构：解析 recommend_answer JSON
// =============================================================================

type recAnswerJSON struct {
	Data struct {
		Interactions []struct {
			QuestionType       string      `json:"question_type"`
			Subject            string      `json:"subject"`
			RecommendedAnswers interface{} `json:"recommended_answers"`
		} `json:"interactions"`
	} `json:"data"`
}

// =============================================================================
// 数据结构：解析 ocr_json
// =============================================================================

type ocrResultJSON struct {
	AnalysisStep struct {
		Interactions []struct {
			Subject                string      `json:"subject"`
			QuestionType           string      `json:"question_type"`
			RecommendedAnswers     interface{} `json:"recommended_answers"`
			UserSelected           interface{} `json:"user_selected"`
			FollowedRecommendation *bool       `json:"followed_recommendation"`
			MatchMethod            string      `json:"match_method"`
		} `json:"interactions"`
		Overall string `json:"overall"`
	} `json:"analysis_step"`
}

// =============================================================================
// 应用层数据结构
// =============================================================================

// ParsedInteraction 单题解析结果
type ParsedInteraction struct {
	Subject                string
	QuestionType           string
	RecommendedAnswer      string
	UserSelected           string
	FollowedRecommendation *bool
	MatchMethod            string
	TechnicalNote          string
}

// ParsedPage 单页解析结果
type ParsedPage struct {
	PageResultID   int64 // page_results.id，作为 px 字段在报告中展示
	PageID         string
	OCRVerified    string
	ScreenshotPath string
	Interactions   []ParsedInteraction
}

// UserPersona 从 OCR 数据构建的用户人设快照
type UserPersona struct {
	Age               string
	Gender            string
	Country           string
	ZipCode           string
	Hispanic          string
	Race              []string
	HasChildren       string
	WeekendActivities []string
	MediaFrequency    map[string]string
	SelfDescription   map[string]string
}

// =============================================================================
// YAML 输出结构（用于 yaml.Marshal 序列化）
// =============================================================================

type sessionInfoOut struct {
	SessionName         string  `yaml:"session_name"`
	SurveyName         string  `yaml:"survey_name"`
	URL                string  `yaml:"url"`
	CreatedAt          string  `yaml:"created_at"`
	PageCount          int     `yaml:"page_count"`
	SessionID          int64   `yaml:"session_id"`
	TotalQuestions     int     `yaml:"total_questions"`
	IgnoredQuestions   int     `yaml:"ignored_questions"`
	OCRMissingQuestions int    `yaml:"ocr_missing_questions"`
	ValidAnsweredCount int     `yaml:"valid_answered_count"`
	StartTime          string  `yaml:"start_time"`
	EndTime            string  `yaml:"end_time"`
	TotalDuration      string  `yaml:"total_duration"`
	OverallScore       float64 `yaml:"overall_score"`
	PureScore          float64 `yaml:"pure_score"`
	Conclusion         string  `yaml:"conclusion"`
}

type systemPersonaOut struct {
	AgeRange      string            `yaml:"age_range,omitempty"`
	Gender        string            `yaml:"gender,omitempty"`
	Race          string            `yaml:"race,omitempty"`
	Occupation    string            `yaml:"occupation,omitempty"`
	Income        int64             `yaml:"income,omitempty"`
	Education     string            `yaml:"education,omitempty"`
	HasChildren   bool              `yaml:"has_children"`
	MaritalStatus string            `yaml:"marital_status,omitempty"`
	Description   string            `yaml:"description,omitempty"`
	OptionMapping map[string]string `yaml:"option_mapping,omitempty"`
}

type pageResultItemOut struct {
	PageResultID int64  `yaml:"page_result_id"` // page_results.id，对应 px
	PX           int64  `yaml:"px"`             // 等于 pageResultID，供报告使用
	PageID       string `yaml:"page_id"`
	Subject      string `yaml:"subject"`
	QuestionType string `yaml:"question_type"`
	RecommendedAnswer string `yaml:"recommended_answer"`
	UserActualAnswer   string `yaml:"user_actual_answer"`
	FollowedRecommendation *bool `yaml:"followed_recommendation"`
	MatchMethod        string `yaml:"match_method"`
	OCRVerified        string `yaml:"ocr_verified"`
	ScreenshotPath     string `yaml:"screenshot_path"`
	TechnicalNote      string `yaml:"technical_note,omitempty"`
	PersonaRelevance   string `yaml:"persona_relevance"`
	FollowedLogic      string `yaml:"followed_logic"`
	ConflictType       string `yaml:"conflict_type"`
	ProblemDescription string `yaml:"problem_description"`
}

type userPersonaOut struct {
	Age               string            `yaml:"age,omitempty"`
	Gender            string            `yaml:"gender,omitempty"`
	Country           string            `yaml:"country,omitempty"`
	ZipCode           string            `yaml:"zip_code,omitempty"`
	Hispanic          string            `yaml:"hispanic,omitempty"`
	Race              []string          `yaml:"race,omitempty"`
	HasChildren       string            `yaml:"has_children,omitempty"`
	WeekendActivities []string          `yaml:"weekend_activities,omitempty"`
	MediaFrequency    map[string]string `yaml:"media_frequency,omitempty"`
	SelfDescription   map[string]string `yaml:"self_description,omitempty"`
}

type personaDimensionOut struct {
	Dimension    string `yaml:"dimension"`
	SystemValue string `yaml:"system_value"`
	UserValue   string `yaml:"user_value"`
	ConflictType string `yaml:"conflict_type"`
	ConflictNote string `yaml:"conflict_note"`
}

type analysisSummaryOut struct {
	PersonaDimensions      []personaDimensionOut `yaml:"persona_dimensions"`
	HardConflictSummary   string                `yaml:"hard_conflict_summary"`
	SoftConflictSummary   string                `yaml:"soft_conflict_summary"`
	MissingConflictSummary string                `yaml:"missing_conflict_summary"`
}

type analysisDataOut struct {
	SessionInfo     sessionInfoOut       `yaml:"session_info"`
	SystemPersona   systemPersonaOut     `yaml:"system_persona"`
	PageResults     []pageResultItemOut  `yaml:"page_results"`
	UserPersona     userPersonaOut        `yaml:"user_persona"`
	AnalysisSummary analysisSummaryOut  `yaml:"analysis_summary"`
}

// =============================================================================
// 主入口
// =============================================================================

// Analyze 执行人设冲突分析，返回报告文件路径
// @param surveyID  survey 表主键
// @returns         报告文件路径，或错误
func Analyze(surveyID int) (string, error) {
	fmt.Printf("[3/5] 加载会话信息 (survey_id=%d)...\n", surveyID)
	survey, err := loadSessionInfo(int64(surveyID))
	if err != nil {
		return "", err
	}

	pages, err := loadPageResults(int64(surveyID))
	if err != nil {
		return "", err
	}
	fmt.Printf("      session_name=%s  survey_name=%s  page_count=%d\n",
		survey.SessionName, survey.SurveyName, len(pages))

	persona, err := loadPersona(survey.SurveyName)
	if err != nil {
		return "", err
	}
	if persona != nil && persona.Description != "" {
		fmt.Printf("      加载人设: %s\n", survey.SurveyName)
	} else {
		fmt.Printf("      加载人设: (default)\n")
	}

	parsedPages, n1, n2, n3 := parseAllPages(pages)
	userPersona := buildUserPersona(parsedPages)

	fmt.Printf("[4/5] 加载提示词 session.persona_analysis...\n")
	promptTemplate, err := loadPrompt()
	if err != nil {
		return "", err
	}
	printPrompt := promptTemplate
	if len(printPrompt) > 200 {
		printPrompt = printPrompt[:200] + "\n      ... [省略 " + fmt.Sprintf("%d", len(printPrompt)-200) + " 字符]"
	}
	fmt.Printf("      提示词长度: %d 字符\n", len(promptTemplate))
	fmt.Println("      ─── 提示词内容 ──────────────────────────────────────────")
	for _, line := range splitLines(printPrompt) {
		fmt.Printf("      %s\n", line)
	}
	fmt.Println("      ─────────────────────────────────────────────────────────")

	fmt.Printf("[5/5] 组装 YAML 数据并调用 Claude...\n")
	yamlData, err := assembleYAMLData(survey, persona, parsedPages, userPersona, pages, n1, n2, n3)
	if err != nil {
		return "", fmt.Errorf("组装 YAML 数据失败: %w", err)
	}
	fmt.Printf("      YAML 数据大小: %d 字符\n", len(yamlData))
	if len(yamlData) > 400 {
		fmt.Printf("      YAML 预览（前 400 字符）:\n%s\n      ...\n", yamlData[:400])
	} else {
		fmt.Printf("      YAML 数据:\n%s\n", yamlData)
	}

	surveyIDStr := fmt.Sprintf("%d", surveyID)
	fmt.Printf("      正在调用 Claude API (survey_id=%s)...\n", surveyIDStr)
	report, err := CallClaude(surveyIDStr, promptTemplate, yamlData)
	if err != nil {
		return "", fmt.Errorf("Claude API 调用失败: %w", err)
	}
	fmt.Printf("      Claude 返回报告长度: %d 字符\n", len(report))

	reportPath, err := saveReport(survey.SessionName, report)
	if err != nil {
		return "", fmt.Errorf("保存报告失败: %w", err)
	}

	writeAnalysisEventLog(int64(surveyID), survey.SessionName, reportPath)
	return reportPath, nil
}

// =============================================================================
// 子函数：数据加载
// =============================================================================

func loadSessionInfo(surveyID int64) (*database.Survey, error) {
	repo := database.NewSurveyRepo()
	s, err := repo.FindByID(surveyID)
	if err != nil {
		return nil, fmt.Errorf("loadSessionInfo: %w", err)
	}
	if s == nil {
		return nil, fmt.Errorf("survey_id=%d 不存在", surveyID)
	}
	return s, nil
}

func loadPageResults(surveyID int64) ([]database.PageResult, error) {
	repo := database.NewPageResultRepo()
	pages, err := repo.FindAllFieldsBySurveyID(surveyID)
	if err != nil {
		return nil, fmt.Errorf("loadPageResults: %w", err)
	}
	if len(pages) == 0 {
		return nil, fmt.Errorf("survey_id=%d 无答题记录", surveyID)
	}
	return pages, nil
}

// loadPersona 加载人设，不存在时取 __default__，始终不返回 nil
func loadPersona(surveyName string) (*database.PersonaRecord, error) {
	persona, err := database.LoadSurveyPersona(surveyName)
	if err != nil || persona == nil {
		return &database.PersonaRecord{}, nil
	}
	return persona, nil
}

func loadPrompt() (string, error) {
	allPrompts, err := database.LoadAllPrompts()
	if err != nil {
		return "", fmt.Errorf("loadPrompt: %w", err)
	}
	content := allPrompts["session.persona_analysis"]
	if content == "" {
		return "", fmt.Errorf("prompts 表中未找到 prompt_key='session.persona_analysis'")
	}
	return content, nil
}

// =============================================================================
// 子函数：JSON 解析
// =============================================================================

// normalizeAnswer 将 interface{}（string 或 []interface{}）统一为字符串
func normalizeAnswer(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []interface{}:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	default:
		return fmt.Sprintf("%v", val)
	}
}

// parseRecommendAnswer 从 recommend_answer JSON 提取推荐答案列表
func parseRecommendAnswer(jsonStr string) []ParsedInteraction {
	if jsonStr == "" {
		return nil
	}
	var rec recAnswerJSON
	if err := json.Unmarshal([]byte(jsonStr), &rec); err != nil {
		return nil
	}
	result := make([]ParsedInteraction, 0, len(rec.Data.Interactions))
	for _, item := range rec.Data.Interactions {
		result = append(result, ParsedInteraction{
			Subject:           item.Subject,
			QuestionType:      item.QuestionType,
			RecommendedAnswer: normalizeAnswer(item.RecommendedAnswers),
		})
	}
	return result
}

// parseOCRJSON 从 ocr_json 提取 analysis_step 交互列表（含用户实际选择）
func parseOCRJSON(jsonStr string) []ParsedInteraction {
	if jsonStr == "" {
		return nil
	}
	var ocr ocrResultJSON
	if err := json.Unmarshal([]byte(jsonStr), &ocr); err != nil {
		return nil
	}
	result := make([]ParsedInteraction, 0, len(ocr.AnalysisStep.Interactions))
	for _, item := range ocr.AnalysisStep.Interactions {
		result = append(result, ParsedInteraction{
			Subject:                item.Subject,
			QuestionType:           item.QuestionType,
			RecommendedAnswer:      normalizeAnswer(item.RecommendedAnswers),
			UserSelected:           normalizeAnswer(item.UserSelected),
			FollowedRecommendation: item.FollowedRecommendation,
			MatchMethod:            item.MatchMethod,
		})
	}
	return result
}

// parseAllPages 遍历所有 PageResult，解析 recommend_answer 和 ocr_json，返回解析结果及统计计数
//
//  统计口径（按页面级别）：
//   - N1（ignoredQuestions）：有推荐答案但 ocr 解析为空（OCR 完全失败）的页面数
//   - N2（ocrMissingQuestions）：有推荐答案但 OCR 数据不完整的页面数（仅推荐答案，无用户选择）
//   - N3（validAnsweredCount）：推荐答案与用户实际选择均存在的页面数
//   - N4（totalQuestions）：页面总数（= page_count，始终 = 24）
func parseAllPages(pages []database.PageResult) ([]ParsedPage, int, int, int) {
	result := make([]ParsedPage, 0, len(pages))
	ignoredCount := 0   // N1：推荐答案存在但 OCR 完全缺失
	ocrMissingCount := 0 // N2：推荐答案存在，OCR 数据不完整
	validCount := 0     // N3：推荐答案 + 用户实际选择均存在

	for _, p := range pages {
		recInteractions := parseRecommendAnswer(p.RecommendAnswer)
		ocrInteractions := parseOCRJSON(p.OCRJSON)

		var interactions []ParsedInteraction
		if len(recInteractions) == 0 {
			// 页面无推荐答案（未进入答题），直接跳过，不计入任何计数
			continue
		}

		if len(ocrInteractions) == 0 {
			// 推荐答案存在但无 OCR → 标记为 OCR 缺失
			for i := range recInteractions {
				recInteractions[i].TechnicalNote = "OCR数据缺失，仅有推荐答案"
			}
			interactions = recInteractions
			ocrMissingCount++
		} else {
			// 优先用 OCR 交互数据（包含用户实际选择）
			interactions = ocrInteractions
			validCount++
		}

		result = append(result, ParsedPage{
			PageResultID:   p.ID,
			PageID:         p.PageID,
			OCRVerified:    p.OCRVerified,
			ScreenshotPath: p.ScreenshotPath,
			Interactions:   interactions,
		})
	}
	return result, ignoredCount, ocrMissingCount, validCount
}

// =============================================================================
// 子函数：构建 UserPersona
// =============================================================================

// buildUserPersona 根据 OCR 交互数据推断用户人设
func buildUserPersona(pages []ParsedPage) UserPersona {
	persona := UserPersona{
		MediaFrequency:  make(map[string]string),
		SelfDescription: make(map[string]string),
	}

	matrixTypes := map[string]bool{
		"matrix_radio":    true,
		"matrix_checkbox": true,
	}

	for _, page := range pages {
		for _, item := range page.Interactions {
			subjectLow := strings.ToLower(item.Subject)
			// 优先取用户实际选择，无则取推荐答案
			answer := item.UserSelected
			if answer == "" {
				answer = item.RecommendedAnswer
			}
			if answer == "" {
				continue
			}

			switch {
			case strings.Contains(subjectLow, "age"):
				persona.Age = answer
			case strings.Contains(subjectLow, "gender"):
				persona.Gender = answer
			case strings.Contains(subjectLow, "country"):
				persona.Country = answer
			case strings.Contains(subjectLow, "zip"):
				persona.ZipCode = answer
			case strings.Contains(subjectLow, "hispanic"):
				persona.Hispanic = answer
			case strings.Contains(subjectLow, "racial") || strings.Contains(subjectLow, "ethnic") || subjectLow == "race":
				persona.Race = append(persona.Race, answer)
			case strings.Contains(subjectLow, "children") || strings.Contains(subjectLow, "child"):
				persona.HasChildren = answer
			case strings.Contains(subjectLow, "weekend") || strings.Contains(subjectLow, "leisure") || strings.Contains(subjectLow, "activity"):
				persona.WeekendActivities = append(persona.WeekendActivities, answer)
			case matrixTypes[item.QuestionType] && (strings.Contains(subjectLow, "media") || strings.Contains(subjectLow, "watch") || strings.Contains(subjectLow, "read") || strings.Contains(subjectLow, "listen") || strings.Contains(subjectLow, "frequency")):
				persona.MediaFrequency[item.Subject] = answer
			case strings.Contains(subjectLow, "descri") || strings.Contains(subjectLow, "yourself") || strings.Contains(subjectLow, "personality"):
				persona.SelfDescription[item.Subject] = answer
			}
		}
	}
	return persona
}

// =============================================================================
// 子函数：组装 YAML 数据
// =============================================================================

// assembleYAMLData 将所有分析数据序列化为 YAML 字符串
func assembleYAMLData(
	survey *database.Survey,
	persona *database.PersonaRecord,
	pages []ParsedPage,
	userPersona UserPersona,
	rawPages []database.PageResult,
	n1, n2, n3 int,
) (string, error) {
	// 统计：totalQuestions = 页面总数（一个 SPA 页面算一道题）
	pageCount := len(pages)
	overallScore := 0.0
	pureScore := 0.0
	conclusion := ""
	if pageCount-n1 > 0 {
		overallScore = float64(n3*10) / float64(pageCount-n1)
	}
	if pageCount-n1-n2 > 0 {
		pureScore = float64(n3*10) / float64(pageCount-n1-n2)
	}
	if overallScore >= 7 {
		conclusion = "用户遵循度高，整体执行力强"
	} else if overallScore >= 4 {
		conclusion = "用户执行力中等，存在部分偏离"
	} else {
		conclusion = "用户执行力较低，需关注人设匹配度"
	}

	var startTime, endTime string
	if len(rawPages) > 0 {
		startTime = rawPages[0].CreatedAt.Format("2006-01-02 15:04:05")
		endTime = rawPages[len(rawPages)-1].CreatedAt.Format("2006-01-02 15:04:05")
	}
	totalDuration := ""
	if len(rawPages) >= 2 {
		dur := rawPages[len(rawPages)-1].CreatedAt.Sub(rawPages[0].CreatedAt)
		h := int(dur.Hours())
		m := int(dur.Minutes()) % 60
		s := int(dur.Seconds()) % 60
		if h > 0 {
			totalDuration = fmt.Sprintf("%dh %dm %ds", h, m, s)
		} else if m > 0 {
			totalDuration = fmt.Sprintf("%dm %ds", m, s)
		} else {
			totalDuration = fmt.Sprintf("%ds", s)
		}
	}

	// 展开所有 page（每页一行），将同页所有交互的推荐答案和用户答案聚合展示
	var pageItems []pageResultItemOut
	for _, page := range pages {
		followedLogic := "是"
		conflictType := "自动"
		problemDesc := "-"

		if len(page.Interactions) == 0 {
			// 无推荐答案的页面
			pageItems = append(pageItems, pageResultItemOut{
				PageResultID:    page.PageResultID,
				PX:            page.PageResultID,
				PageID:        page.PageID,
				Subject:       "",
				QuestionType:  "",
				OCRVerified:   page.OCRVerified,
				ScreenshotPath: page.ScreenshotPath,
				PersonaRelevance: "无关",
				FollowedLogic:  "跳过",
				ConflictType:   "缺失",
				ProblemDescription: "推荐答案缺失，未进入答题流程",
			})
			continue
		}

		// 以第一个 interaction 的状态代表整页
		item := page.Interactions[0]
		if item.FollowedRecommendation == nil {
			followedLogic = "跳过"
			if item.RecommendedAnswer != "" {
				conflictType = "缺失"
				problemDesc = "截图路径存在但OCR数据不可用，无法验证"
			}
		} else if !*item.FollowedRecommendation {
			followedLogic = "否"
			problemDesc = "用户实际选择与推荐答案不一致，Claude分析偏离原因"
		}

		personaRel := "无关"
		if containsPersonaKeyword(item.Subject) {
			personaRel = "关键"
		}

		pageItems = append(pageItems, pageResultItemOut{
			PageResultID:          page.PageResultID,
			PX:                  page.PageResultID,
			PageID:              page.PageID,
			Subject:             item.Subject,
			QuestionType:        item.QuestionType,
			RecommendedAnswer:    item.RecommendedAnswer,
			UserActualAnswer:    item.UserSelected,
			FollowedRecommendation: item.FollowedRecommendation,
			MatchMethod:         item.MatchMethod,
			OCRVerified:        page.OCRVerified,
			ScreenshotPath:     page.ScreenshotPath,
			TechnicalNote:       item.TechnicalNote,
			PersonaRelevance:    personaRel,
			FollowedLogic:       followedLogic,
			ConflictType:       conflictType,
			ProblemDescription: problemDesc,
		})
	}

	out := analysisDataOut{
		SessionInfo: sessionInfoOut{
			SessionName:          survey.SessionName,
			SurveyName:          survey.SurveyName,
			URL:                 survey.URL,
			CreatedAt:           survey.CreatedAt.Format("2006-01-02 15:04:05"),
			PageCount:           pageCount, // 页面总数 = 总题数（每页算一道题）
			SessionID:           survey.ID,
			TotalQuestions:      pageCount, // 每页一行，页面总数即总题数
			IgnoredQuestions:    n1,
			OCRMissingQuestions: n2,
			ValidAnsweredCount: n3,
			StartTime:           startTime,
			EndTime:             endTime,
			TotalDuration:       totalDuration,
			OverallScore:        overallScore,
			PureScore:           pureScore,
			Conclusion:          conclusion,
		},
		SystemPersona: systemPersonaOut{
			AgeRange:      persona.AgeRange,
			Gender:        persona.Gender,
			Race:          persona.Race,
			Occupation:    persona.Occupation,
			Income:        persona.Income,
			Education:     persona.Education,
			HasChildren:   persona.HasChildren,
			MaritalStatus: persona.MaritalStatus,
			Description:   persona.Description,
			OptionMapping: persona.OptionMapping,
		},
		PageResults: pageItems,
		UserPersona: userPersonaOut{
			Age:               userPersona.Age,
			Gender:            userPersona.Gender,
			Country:           userPersona.Country,
			ZipCode:           userPersona.ZipCode,
			Hispanic:          userPersona.Hispanic,
			Race:              userPersona.Race,
			HasChildren:       userPersona.HasChildren,
			WeekendActivities: userPersona.WeekendActivities,
			MediaFrequency:    userPersona.MediaFrequency,
			SelfDescription:   userPersona.SelfDescription,
		},
		AnalysisSummary: analysisSummaryOut{
			PersonaDimensions:      []personaDimensionOut{},
			HardConflictSummary:   "",
			SoftConflictSummary:   "",
			MissingConflictSummary: "",
		},
	}

	data, err := yaml.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("yaml.Marshal: %w", err)
	}
	return string(data), nil
}

// =============================================================================
// 子函数：保存报告与日志
// =============================================================================

// saveReport 将 Markdown 报告转换为带 CSS 样式的 HTML 并保存
func saveReport(sessionName, report string) (string, error) {
	logDir := filepath.Join("storage", "analysis_logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("创建目录 %s: %w", logDir, err)
	}
	path := filepath.Join(logDir, fmt.Sprintf("%s_report.html", sessionName))

	html := markdownToHTML(report)
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		return "", fmt.Errorf("写入报告文件: %w", err)
	}
	return path, nil
}

const reportCSS = `
:root {
  --bg:       #1e1e2e;
  --surface:  #262637;
  --border:   #3d3d5c;
  --accent:   #89b4fa;
  --green:    #a6e3a1;
  --red:      #f38ba8;
  --yellow:   #f9e2af;
  --purple:   #cba6f7;
  --cyan:     #94e2d5;
  --text:     #cdd6f4;
  --muted:    #6c7086;
  --font:     'Segoe UI', system-ui, -apple-system, sans-serif;
}
* { box-sizing: border-box; margin: 0; padding: 0; }
body {
  background: var(--bg);
  color: var(--text);
  font-family: var(--font);
  font-size: 15px;
  line-height: 1.7;
  padding: 32px 24px;
  max-width: 900px;
  margin: 0 auto;
}
h1 { font-size: 1.8em; color: var(--accent); border-bottom: 2px solid var(--accent); padding-bottom: 10px; margin-bottom: 24px; }
h2 { font-size: 1.35em; color: var(--purple); margin: 28px 0 14px; padding-left: 10px; border-left: 4px solid var(--purple); }
h3 { font-size: 1.1em;  color: var(--cyan);  margin: 20px 0 10px; }
p  { margin-bottom: 12px; }
ul, ol { padding-left: 28px; margin-bottom: 14px; }
li { margin-bottom: 6px; }
strong { color: var(--yellow); font-weight: 600; }
em   { color: var(--cyan); font-style: normal; font-weight: 500; }
blockquote {
  border-left: 4px solid var(--border);
  padding: 10px 16px;
  margin: 14px 0;
  background: var(--surface);
  border-radius: 0 6px 6px 0;
  color: var(--muted);
}
table {
  width: 100%;
  border-collapse: collapse;
  margin: 16px 0;
  font-size: 0.92em;
}
th {
  background: var(--surface);
  color: var(--accent);
  padding: 10px 14px;
  text-align: left;
  border: 1px solid var(--border);
  font-weight: 600;
}
td {
  padding: 8px 14px;
  border: 1px solid var(--border);
  vertical-align: top;
}
tr:nth-child(even) td { background: rgba(38,38,55,0.5); }
tr.conflict-row td { background: rgba(243,139,168,0.15) !important; color: #f38ba8 !important; }
tr.conflict-row td:first-child { color: #f38ba8 !important; font-weight: 600; }
hr { border: none; border-top: 1px solid var(--border); margin: 24px 0; }
code {
  background: var(--surface);
  color: var(--green);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Cascadia Code', 'Fira Code', Consolas, monospace;
  font-size: 0.88em;
}
pre {
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 16px;
  overflow-x: auto;
  margin: 16px 0;
}
pre code { background: none; padding: 0; color: var(--text); }
.math-block { text-align: center; margin: 18px 0; overflow-x: auto; }
.math-inline { padding: 0 2px; }
`

const reportHTMLTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>人设冲突分析报告</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/katex.min.css">
<script defer src="https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/katex.min.js"></script>
<script defer src="https://cdn.jsdelivr.net/npm/katex@0.16.9/dist/contrib/auto-render.min.js"
        onload="renderMathInElement(document.body,{delimiters:[{left:'$$$$',right:'$$$$',display:true},{left:'$',right:'$',display:false}],trust:true});"></script>
<style>%s</style>
</head>
<body>
%s
</body>
</html>`

// markdownToHTML 将 Markdown 文本转换为带样式的 HTML
func markdownToHTML(md string) string {
	lines := splitLines(md)
	var out strings.Builder
	// 简单状态机：检测当前块类型
	inCodeBlock := false
	codeBuf := strings.Builder{}
	inTable := false
	tableBuf := strings.Builder{}
	inList := false
	listTag := ""

	flushCode := func() {
		escaped := escapeHTML(codeBuf.String())
		out.WriteString("<pre><code>")
		out.WriteString(escaped)
		out.WriteString("</code></pre>\n")
		codeBuf.Reset()
	}

	flushTable := func() {
		raw := tableBuf.String()
		if raw != "" {
			out.WriteString(tableToHTML(raw))
		}
		tableBuf.Reset()
		inTable = false
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// 代码块
		if strings.HasPrefix(line, "```") {
			if !inCodeBlock {
				if inTable {
					flushTable()
				}
				inCodeBlock = true
				codeBuf.Reset()
			} else {
				flushCode()
				inCodeBlock = false
			}
			continue
		}
		if inCodeBlock {
			if codeBuf.Len() > 0 {
				codeBuf.WriteString("\n")
			}
			codeBuf.WriteString(line)
			continue
		}

		// 表格
		if strings.HasPrefix(line, "|") && !inTable {
			inTable = true
			tableBuf.Reset()
		}
		if inTable {
			tableBuf.WriteString(line + "\n")
			// 分隔行或结束表格
			if strings.TrimSpace(line) == "" || !strings.HasPrefix(line, "|") {
				flushTable()
			}
			continue
		}

		// 列表
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			if !inList || listTag != "ul" {
				if inList {
					out.WriteString("</" + listTag + ">\n")
				}
				out.WriteString("<ul>\n")
				listTag = "ul"
				inList = true
			}
			content := strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
			content = strings.TrimPrefix(content, "+ ")
			out.WriteString("<li>" + inlineMarkdown(content) + "</li>\n")
			continue
		}
		if matched := listNumberRe.FindStringSubmatch(trimmed); len(matched) > 0 {
			if !inList || listTag != "ol" {
				if inList {
					out.WriteString("</" + listTag + ">\n")
				}
				out.WriteString("<ol>\n")
				listTag = "ol"
				inList = true
			}
			content := strings.TrimPrefix(trimmed, matched[0])
			out.WriteString("<li>" + inlineMarkdown(content) + "</li>\n")
			continue
		}
		if inList && trimmed == "" {
			out.WriteString("</" + listTag + ">\n")
			inList = false
			listTag = ""
			continue
		}
		if inList && !strings.HasPrefix(trimmed, "- ") && !strings.HasPrefix(trimmed, "* ") &&
			!strings.HasPrefix(trimmed, "+ ") && !listNumberRe.MatchString(trimmed) {
			out.WriteString("</" + listTag + ">\n")
			inList = false
			listTag = ""
		}

		// 标题
		if strings.HasPrefix(trimmed, "#### ") {
			out.WriteString("<h4>" + inlineMarkdown(strings.TrimPrefix(trimmed, "#### ")) + "</h4>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "### ") {
			out.WriteString("<h3>" + inlineMarkdown(strings.TrimPrefix(trimmed, "### ")) + "</h3>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "## ") {
			out.WriteString("<h2>" + inlineMarkdown(strings.TrimPrefix(trimmed, "## ")) + "</h2>\n")
			continue
		}
		if strings.HasPrefix(trimmed, "# ") {
			out.WriteString("<h1>" + inlineMarkdown(strings.TrimPrefix(trimmed, "# ")) + "</h1>\n")
			continue
		}

		// 分隔线
		if strings.TrimSpace(trimmed) == "---" || strings.TrimSpace(trimmed) == "***" || strings.TrimSpace(trimmed) == "___" {
			out.WriteString("<hr>\n")
			continue
		}

		// 引用块
		if strings.HasPrefix(trimmed, "> ") {
			out.WriteString("<blockquote>" + inlineMarkdown(strings.TrimPrefix(trimmed, "> ")) + "</blockquote>\n")
			continue
		}

		// 空行
		if trimmed == "" {
			continue
		}

		// 普通段落
		out.WriteString("<p>" + inlineMarkdown(trimmed) + "</p>\n")
	}

	if inCodeBlock {
		flushCode()
	}
	if inTable {
		flushTable()
	}
	if inList {
		out.WriteString("</" + listTag + ">\n")
	}

	body := out.String()
	return fmt.Sprintf(reportHTMLTemplate, reportCSS, body)
}

var (
	listNumberRe = regexpMustCompile(`(?m)^(\d+)\.\s+`)
)

func regexpMustCompile(pattern string) *regexp.Regexp {
	return regexp.MustCompile(pattern)
}

// tableToHTML 将 Markdown 表格转换为 HTML
// 对第二章（系统人设 vs 用户人设对比表）包含"冲突"的行添加红色高亮
// 第三章（逐题分析）添加 page_result_id / px 列
func tableToHTML(raw string) string {
	lines := splitLines(strings.TrimSpace(raw))
	if len(lines) < 2 {
		return ""
	}

	// 判断是否为第二章：表头包含"维度"
	isChapter2 := false
	// 判断是否为第三章：表头包含"序号"
	isChapter3 := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") {
			if strings.Contains(trimmed, "维度") && strings.Contains(trimmed, "系统预设") {
				isChapter2 = true
			}
			if strings.Contains(trimmed, "序号") && strings.Contains(trimmed, "页面ID") {
				isChapter3 = true
			}
			break
		}
	}

	var sb strings.Builder
	sb.WriteString("<table>\n")

	for rowIdx, row := range lines {
		trimmed := strings.Trim(row, " \t")
		if trimmed == "" {
			continue
		}
		if strings.TrimSpace(strings.ReplaceAll(row, "-", "")) == "" ||
			strings.TrimSpace(strings.ReplaceAll(row, ":", "")) == "" {
			continue
		}
		cells := strings.Split(row, "|")
		if len(cells) > 0 && strings.TrimSpace(cells[0]) == "" {
			cells = cells[1:]
		}
		if len(cells) > 0 && strings.TrimSpace(cells[len(cells)-1]) == "" {
			cells = cells[:len(cells)-1]
		}

		tag := "td"
		rowClass := ""
		if rowIdx == 0 {
			tag = "th"
		} else if isChapter2 {
			// 第二章：冲突类型列（第4列）含"冲突"字样 → 整行标红
			for _, cell := range cells {
				if strings.Contains(strings.TrimSpace(cell), "冲突") &&
					!strings.Contains(strings.TrimSpace(cell), "无冲突") {
					rowClass = ` class="conflict-row"`
					break
				}
			}
		}

		sb.WriteString("  <tr" + rowClass + ">")
		for colIdx, cell := range cells {
			sb.WriteString("<" + tag + ">")
			if rowIdx > 0 && isChapter3 && colIdx == 0 {
				// 第三章：序号列加上 px 前缀
				cell = "px" + strings.TrimSpace(cell)
			}
			sb.WriteString(inlineMarkdown(strings.TrimSpace(cell)))
			sb.WriteString("</" + tag + ">")
		}
		sb.WriteString("</tr>\n")
	}

	sb.WriteString("</table>\n")
	return sb.String()
}

// escapeHTML 转义 HTML 特殊字符
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// inlineMarkdown 处理行内 Markdown 语法（加粗、斜体、代码、行内链接）
func inlineMarkdown(s string) string {
	// 优先处理行内代码块
	for {
		start := strings.Index(s, "`")
		if start == -1 {
			break
		}
		end := strings.Index(s[start+1:], "`")
		if end == -1 {
			break
		}
		end += start + 1
		inner := s[start+1 : end]
		s = s[:start] + "<code>" + escapeHTML(inner) + "</code>" + s[end+1:]
	}

	// 加粗 **text** 或 __text__
	s = inlineReplacer(s, `\*\*(.+?)\*\*`, "<strong>$1</strong>")
	s = inlineReplacer(s, `__(.+?)__`, "<strong>$1</strong>")

	// 斜体 *text* 或 _text_（bold 已处理完，剩余 * 均为单个）
	s = inlineReplacer(s, `\*([^*\n]+)\*`, "<em>$1</em>")
	s = inlineReplacer(s, `_([^_\n]+)_`, "<em>$1</em>")

	// LaTeX 数学公式（$$...$$ 块级）
	s = inlineReplacer(s, `\$\$(.+?)\$\$`, "<div class=\"math-block\">$$$$1$$</div>")

	// LaTeX 数学公式（$...$ 行内）—— Go RE2 不支持 lookbehind，用手动扫描跳过 \$
	s = inlineInlineMath(s)

	return s
}

// inlineInlineMath 替换未转义的 $...$ 为 <span class="math-inline">
func inlineInlineMath(s string) string {
	var buf strings.Builder
	i := 0
	for i < len(s) {
		// 跳过转义的 \$
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == '$' {
			buf.WriteByte('\\')
			buf.WriteByte('$')
			i += 2
			continue
		}
		if s[i] == '$' {
			// 找结束 $（同行内，非转义）
			j := i + 1
			for j < len(s) && s[j] != '$' && s[j] != '\n' {
				if s[j] == '\\' && j+1 < len(s) && s[j+1] == '$' {
					j += 2
				} else {
					j++
				}
			}
			if j < len(s) && s[j] == '$' && j > i+1 {
				buf.WriteString(`<span class="math-inline">$`)
				buf.WriteString(s[i+1 : j])
				buf.WriteString(`$</span>`)
				i = j + 1
				continue
			}
		}
		buf.WriteByte(s[i])
		i++
	}
	return buf.String()
}

func inlineReplacer(s, pattern, replacement string) string {
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(s, replacement)
}

// writeAnalysisEventLog 追加分析完成记录到事件日志
func writeAnalysisEventLog(surveyID int64, sessionName, reportPath string) {
	logDir := filepath.Join("storage", "logs", "events")
	_ = os.MkdirAll(logDir, 0755)

	date := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, fmt.Sprintf("sys_%s.log", date))

	line := fmt.Sprintf("%s|ANALYSIS|survey_id=%d|session=%s|报告=%s|完成\n",
		time.Now().Format("2006-01-02 15:04:05"),
		surveyID,
		sessionName,
		reportPath,
	)

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
