package database

import (
	"database/sql"
	"fmt"
)

// SurveyRepo 提供 survey 表的数据库操作
type SurveyRepo struct {
	db *sql.DB
}

// NewSurveyRepo 创建 SurveyRepo 实例
func NewSurveyRepo() *SurveyRepo {
	return &SurveyRepo{db: GetDB()}
}

// FindBySessionName 根据 session_name 查询记录
// @param sessionName 会话名称
// @returns Survey 记录和 error
func (r *SurveyRepo) FindBySessionName(sessionName string) (*Survey, error) {
	row := r.db.QueryRow(`
		SELECT id, session_name, survey_name, url, created_at
		FROM survey WHERE session_name = ?
	`, sessionName)

	var s Survey
	err := row.Scan(&s.ID, &s.SessionName, &s.SurveyName, &s.URL, &s.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // 不存在返回 nil
		}
		return nil, fmt.Errorf("FindBySessionName scan: %w", err)
	}
	return &s, nil
}

// FindByID 根据 id 查询记录
// @param id 主键
// @returns Survey 记录和 error
func (r *SurveyRepo) FindByID(id int64) (*Survey, error) {
	row := r.db.QueryRow(`
		SELECT id, session_name, survey_name, url, created_at
		FROM survey WHERE id = ?
	`, id)

	var s Survey
	err := row.Scan(&s.ID, &s.SessionName, &s.SurveyName, &s.URL, &s.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("FindByID scan: %w", err)
	}
	return &s, nil
}

// Create 创建新记录
// @param sessionID 会话 ID
// @param surveyID 问卷标识
// @param url 问卷 URL
// @returns 新记录 ID 和 error
func (r *SurveyRepo) Create(sessionID, surveyID, url string) (int64, error) {
	result, err := r.db.Exec(`
		INSERT INTO survey (session_name, survey_name, url) VALUES (?, ?, ?)
	`, sessionID, surveyID, url)
	if err != nil {
		return 0, fmt.Errorf("Create survey: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("Create LastInsertId: %w", err)
	}
	return id, nil
}

// GetOrCreate 查询记录，存在则返回，不存在则创建
// @param sessionID 会话 ID
// @param surveyID 问卷标识
// @param url 问卷 URL
// @returns Survey 记录（包含 ID）和 error
func (r *SurveyRepo) GetOrCreate(sessionID, surveyID, url string) (*Survey, error) {
	// 先查询
	existing, err := r.FindBySessionName(sessionID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	// 不存在则创建
	id, err := r.Create(sessionID, surveyID, url)
	if err != nil {
		return nil, err
	}

	return &Survey{
		ID:          id,
		SessionName: sessionID,
		SurveyName:  surveyID,
		URL:         url,
	}, nil
}
