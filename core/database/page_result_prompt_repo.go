package database

import (
	"database/sql"
	"fmt"
)

// PageResultPromptRepo 提供 page_result_prompt 表的数据库操作（只新增不修改）
type PageResultPromptRepo struct {
	db *sql.DB
}

// NewPageResultPromptRepo 创建 PageResultPromptRepo 实例
func NewPageResultPromptRepo() *PageResultPromptRepo {
	return &PageResultPromptRepo{db: GetDB()}
}

// Create 插入新的 prompt 记录
// @param pageResultID  page_results.id
// @param prompt        system + user prompt 拼接文本（HTML 已截断）
// @param pageSummary   AI 返回的页面摘要
// @returns 新记录 ID 和 error
func (r *PageResultPromptRepo) Create(pageResultID int64, prompt, pageSummary string) (int64, error) {
	result, err := r.db.Exec(`
		INSERT INTO page_result_prompt (page_result_id, prompt, page_summary)
		VALUES (?, ?, ?)
	`, pageResultID, prompt, pageSummary)
	if err != nil {
		return 0, fmt.Errorf("Create page_result_prompt: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("Create page_result_prompt LastInsertId: %w", err)
	}
	return id, nil
}
