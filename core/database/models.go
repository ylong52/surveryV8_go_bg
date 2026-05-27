package database

import "time"

// Survey 会话记录，对应 survey 表
type Survey struct {
	ID          int64     `json:"id"`
	SessionName string    `json:"session_name"`
	SurveyName  string    `json:"survey_name"` // 问卷标识
	URL         string    `json:"url"`
	CreatedAt   time.Time `json:"created_at"`
}

// PageResult 页面结果，对应 page_results 表
type PageResult struct {
	ID                 int64     `json:"id"`
	SurveyID           int64     `json:"survey_id"`            // FK → survey.id
	PageID             string    `json:"page_id"`
	YamlTemplate       string    `json:"yaml_template"`        // YAML 模板（已清空）
	RecommendAnswer    string    `json:"recommend_answer"`     // AI 推荐答案（JSON 格式）
	PageFingerprint    string    `json:"page_fingerprint"`     // interactions 去除答案后的 JSON
	PageFingerprintMD5 string    `json:"page_fingerprint_md5"` // PageFingerprint 的 MD5
	ScreenshotPath     string    `json:"screenshot_path"`      // 截图文件路径
	CreatedAt          time.Time `json:"created_at"`
}

// PageResultPrompt AI Prompt 记录，对应 page_result_prompt 表（只新增不修改）
type PageResultPrompt struct {
	ID           int64     `json:"id"`
	PageResultID int64     `json:"page_result_id"` // FK → page_results.id
	Prompt       string    `json:"prompt"`       // system + user prompt（HTML已截断）
	PageSummary  string    `json:"page_summary"` // AI 返回的页面摘要
	CreatedAt    time.Time `json:"created_at"`
}

// PageHTMLSource HTML 源码记录，对应 page_html_source 表
type PageHTMLSource struct {
	ID           int64     `json:"id"`
	PageResultID int64     `json:"page_results_id"` // FK → page_results.id
	RawHTMLMD5   string    `json:"raw_html_md5"`   // RawHTML MD5 值，用于去重
	RawHTML      string    `json:"raw_html"`        // 原始 HTML（未清洗）
	CleanHTML    string    `json:"clean_html"`      // 清洗后 HTML（可选）
	CreatedAt    time.Time `json:"created_at"`
}

// Site 站点配置，对应 sites 表
type Site struct {
	ID         int64  `json:"id"`
	SiteName   string `json:"site_name"`
	SurveyName string `json:"survey_name"`
	URL        string `json:"url"`
	IsActive   bool   `json:"is_active"`
}

// SiteFingerprint 页面指纹配置，对应 site_page_fingerprints 表
type SiteFingerprint struct {
	ID                   int64    `json:"id"`
	SiteID               int64    `json:"site_id"`
	PageIDPatternRegexes []string `json:"page_id_pattern_regexes"` // 从 JSON 数组解析
	NextPattern          string   `json:"next_pattern"`
	WaitAfterClick       int      `json:"wait_after_click"`
}
