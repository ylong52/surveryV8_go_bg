package database

import (
	"database/sql"
	"fmt"
)

// UserSubmittedAnswerRepo 提供 user_submitted_answers 表的数据库操作
type UserSubmittedAnswerRepo struct {
	db *sql.DB
}

// NewUserSubmittedAnswerRepo 创建 UserSubmittedAnswerRepo 实例
func NewUserSubmittedAnswerRepo() *UserSubmittedAnswerRepo {
	return &UserSubmittedAnswerRepo{db: GetDB()}
}

// Upsert 插入或更新用户提交的答案
// 唯一约束: session_id + page_results_id + question + row_label + col_label
// 同一个 session 的同一页面同一问题，答案不同则覆盖更新
func (r *UserSubmittedAnswerRepo) Upsert(answer *UserSubmittedAnswer) error {
	query := `
		INSERT INTO user_submitted_answers
			(session_id, page_results_id, question, answer, interaction_type, row_label, col_label, answer_id, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, page_results_id, question, row_label, col_label)
		DO UPDATE SET
			answer = excluded.answer,
			interaction_type = excluded.interaction_type,
			answer_id = excluded.answer_id,
			timestamp = excluded.timestamp,
			submitted_at = datetime('now', 'localtime')
	`
	_, err := r.db.Exec(query,
		answer.SessionID,
		answer.PageResultsID,
		answer.Question,
		answer.Answer,
		answer.InteractionType,
		answer.RowLabel,
		answer.ColLabel,
		answer.AnswerID,
		answer.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("Upsert user_submitted_answer: %w", err)
	}
	return nil
}

// FindBySessionID 根据 session_id 查询所有提交的答案
// @param sessionID 会话 ID
// @returns UserSubmittedAnswer 列表和 error
func (r *UserSubmittedAnswerRepo) FindBySessionID(sessionID string) ([]UserSubmittedAnswer, error) {
	rows, err := r.db.Query(`
		SELECT id, session_id, page_results_id, question, answer, interaction_type, row_label, col_label, answer_id, timestamp, submitted_at
		FROM user_submitted_answers 
		WHERE session_id = ?
		ORDER BY page_results_id ASC, timestamp ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("FindBySessionID query: %w", err)
	}
	defer rows.Close()

	var results []UserSubmittedAnswer
	for rows.Next() {
		var a UserSubmittedAnswer
		var rowLabel, colLabel, answerID sql.NullString
		err := rows.Scan(
			&a.ID, &a.SessionID, &a.PageResultsID, &a.Question, &a.Answer,
			&a.InteractionType, &rowLabel, &colLabel, &answerID, &a.Timestamp, &a.SubmittedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("FindBySessionID scan: %w", err)
		}
		a.RowLabel = rowLabel.String
		a.ColLabel = colLabel.String
		a.AnswerID = answerID.String
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("FindBySessionID rows: %w", err)
	}
	return results, nil
}

// FindByPageResultsID 根据 page_results_id 查询所有提交的答案
// @param pageResultsID page_results 表主键
// @returns UserSubmittedAnswer 列表和 error
func (r *UserSubmittedAnswerRepo) FindByPageResultsID(pageResultsID int64) ([]UserSubmittedAnswer, error) {
	rows, err := r.db.Query(`
		SELECT id, session_id, page_results_id, question, answer, interaction_type, row_label, col_label, answer_id, timestamp, submitted_at
		FROM user_submitted_answers 
		WHERE page_results_id = ?
		ORDER BY timestamp ASC
	`, pageResultsID)
	if err != nil {
		return nil, fmt.Errorf("FindByPageResultsID query: %w", err)
	}
	defer rows.Close()

	var results []UserSubmittedAnswer
	for rows.Next() {
		var a UserSubmittedAnswer
		var rowLabel, colLabel, answerID sql.NullString
		err := rows.Scan(
			&a.ID, &a.SessionID, &a.PageResultsID, &a.Question, &a.Answer,
			&a.InteractionType, &rowLabel, &colLabel, &answerID, &a.Timestamp, &a.SubmittedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("FindByPageResultsID scan: %w", err)
		}
		a.RowLabel = rowLabel.String
		a.ColLabel = colLabel.String
		a.AnswerID = answerID.String
		results = append(results, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("FindByPageResultsID rows: %w", err)
	}
	return results, nil
}

// DeleteBySessionID 删除指定 session_id 的所有答案
// @param sessionID 会话 ID
// @returns error
func (r *UserSubmittedAnswerRepo) DeleteBySessionID(sessionID string) error {
	_, err := r.db.Exec(`DELETE FROM user_submitted_answers WHERE session_id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("DeleteBySessionID: %w", err)
	}
	return nil
}
