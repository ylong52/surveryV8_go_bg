package database

// prompt_repo.go — 提示词相关数据库操作
//   提供 prompts / survey_prompt_overrides / survey_persona / prompt_assembly_orders 的查询方法

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// PersonaRecord 人设记录，对应 survey_persona 表
type PersonaRecord struct {
	SurveyID      string
	PersonaMode   string
	AgeRange      string
	Gender        string
	Race          string
	Occupation    string
	Income        int64
	Education     string
	HasChildren   bool
	MaritalStatus string
	Description   string
	OptionMapping map[string]string
}

// LoadAllPrompts 加载所有活跃提示词，返回 prompt_key→content 的 map
// @returns  map[prompt_key]content，error
func LoadAllPrompts() (map[string]string, error) {
	rows, err := GetDB().Query(`SELECT prompt_key, content FROM prompts WHERE is_active=1`)
	if err != nil {
		return nil, fmt.Errorf("LoadAllPrompts: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, content string
		if err := rows.Scan(&key, &content); err != nil {
			return nil, fmt.Errorf("LoadAllPrompts scan: %w", err)
		}
		result[key] = content
	}
	return result, rows.Err()
}

// LoadAssemblyOrder 按 step_order 加载指定 assembly_name 的 prompt_key 有序列表
// @param assemblyName  组装名（如 "analyze"）
// @returns             有序 prompt_key 切片，error
func LoadAssemblyOrder(assemblyName string) ([]string, error) {
	rows, err := GetDB().Query(
		`SELECT prompt_key FROM prompt_assembly_orders WHERE assembly_name=? AND is_active=1 ORDER BY step_order ASC`,
		assemblyName,
	)
	if err != nil {
		return nil, fmt.Errorf("LoadAssemblyOrder: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("LoadAssemblyOrder scan: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// LoadSurveyOverrides 加载指定 survey_id 的提示词覆盖，返回 prompt_key→content 的 map
// @param surveyID  问卷 ID
// @returns         map[prompt_key]content，error
func LoadSurveyOverrides(surveyID string) (map[string]string, error) {
	rows, err := GetDB().Query(
		`SELECT prompt_key, content FROM survey_prompt_overrides WHERE survey_id=? AND is_active=1`,
		surveyID,
	)
	if err != nil {
		return nil, fmt.Errorf("LoadSurveyOverrides: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, content string
		if err := rows.Scan(&key, &content); err != nil {
			return nil, fmt.Errorf("LoadSurveyOverrides scan: %w", err)
		}
		result[key] = content
	}
	return result, rows.Err()
}

// LoadDefaultPersona 加载默认人设（survey_id='__default__'）
// @returns  PersonaRecord，error
func LoadDefaultPersona() (*PersonaRecord, error) {
	return loadPersonaByID("__default__")
}

// LoadSurveyPersona 加载指定 survey_id 的人设，不存在时回退到 __default__
// @param surveyID  问卷 ID
// @returns         PersonaRecord，error
func LoadSurveyPersona(surveyID string) (*PersonaRecord, error) {
	rec, err := loadPersonaByID(surveyID)
	if err == sql.ErrNoRows || rec == nil {
		return loadPersonaByID("__default__")
	}
	return rec, err
}

// GetFixedTemplateByType 根据 question_type 查询 yaml_template 内容
// @param questionType  题型字符串（如 "single"、"mixed"）
// @returns             yaml_template 内容，未找到返回 ""，error
func GetFixedTemplateByType(questionType string) (string, error) {
	var tmpl sql.NullString
	err := GetDB().QueryRow(
		`SELECT yaml_template FROM prompts WHERE question_type=? AND is_active=1 LIMIT 1`,
		questionType,
	).Scan(&tmpl)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("GetFixedTemplateByType: %w", err)
	}
	return tmpl.String, nil
}

// LoadQuestionTypes 加载所有有效题型列表（yaml_template 和 prompt_type 非空的活跃记录）
// @returns  题型字符串切片，error
func LoadQuestionTypes() ([]string, error) {
	rows, err := GetDB().Query(
		`SELECT DISTINCT question_type FROM prompts
		 WHERE yaml_template != '' AND yaml_template IS NOT NULL
		   AND prompt_type != '' AND prompt_type IS NOT NULL
		   AND is_active=1 AND question_type != '' AND question_type IS NOT NULL`,
	)
	if err != nil {
		return nil, fmt.Errorf("LoadQuestionTypes: %w", err)
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var qt string
		if err := rows.Scan(&qt); err != nil {
			return nil, fmt.Errorf("LoadQuestionTypes scan: %w", err)
		}
		if qt != "" {
			types = append(types, qt)
		}
	}
	return types, rows.Err()
}

// LoadPromptByQuestionType 根据 question_type 查询 content 和 yaml_template
// @param questionType  题型字符串
// @returns             content, yamlTemplate, error
func LoadPromptByQuestionType(questionType string) (content, yamlTemplate string, err error) {
	var c, t sql.NullString
	err = GetDB().QueryRow(
		`SELECT content, yaml_template FROM prompts
		 WHERE question_type=? AND is_active=1 LIMIT 1`,
		questionType,
	).Scan(&c, &t)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("LoadPromptByQuestionType: %w", err)
	}
	return c.String, t.String, nil
}

// LoadTypeDetectorPrompt 加载 system.type_detector 提示词内容
// @returns  提示词字符串（含 {question_types} 占位符），error
func LoadTypeDetectorPrompt() (string, error) {
	var content sql.NullString
	err := GetDB().QueryRow(
		`SELECT content FROM prompts WHERE prompt_key='system.type_detector' AND is_active=1 LIMIT 1`,
	).Scan(&content)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("LoadTypeDetectorPrompt: %w", err)
	}
	return content.String, nil
}

// loadPersonaByID 从 survey_persona 表加载一条人设记录
func loadPersonaByID(surveyID string) (*PersonaRecord, error) {
	var r PersonaRecord
	var income sql.NullInt64
	var ageRange, gender, race, occupation, education, maritalStatus, description, optMapping sql.NullString
	var hasChildren sql.NullInt64

	err := GetDB().QueryRow(
		`SELECT survey_id, persona_mode, age_range, gender, race, occupation, income, education,
		        has_children, marital_status, description, option_mapping
		 FROM survey_persona WHERE survey_id=? AND is_active=1`,
		surveyID,
	).Scan(
		&r.SurveyID, &r.PersonaMode,
		&ageRange, &gender, &race, &occupation, &income, &education,
		&hasChildren, &maritalStatus, &description, &optMapping,
	)
	if err != nil {
		return nil, err
	}

	r.AgeRange = ageRange.String
	r.Gender = gender.String
	r.Race = race.String
	r.Occupation = occupation.String
	r.Income = income.Int64
	r.Education = education.String
	r.HasChildren = hasChildren.Int64 == 1
	r.MaritalStatus = maritalStatus.String
	r.Description = description.String

	if optMapping.Valid && optMapping.String != "" {
		if err := json.Unmarshal([]byte(optMapping.String), &r.OptionMapping); err != nil {
			r.OptionMapping = nil
		}
	}

	return &r, nil
}
