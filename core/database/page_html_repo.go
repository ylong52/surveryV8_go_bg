package database

import (
	"database/sql"
	"fmt"
)

// PageHTMLRepo 提供 page_html_source 表的数据库操作
type PageHTMLRepo struct {
	db *sql.DB
}

// NewPageHTMLRepo 创建 PageHTMLRepo 实例
func NewPageHTMLRepo() *PageHTMLRepo {
	return &PageHTMLRepo{db: GetDB()}
}

// FindByPageResultID 根据 page_results_id 查询 HTML 源码记录
// @param pageResultID page_results 表主键
// @returns PageHTMLSource 记录和 error
func (r *PageHTMLRepo) FindByPageResultID(pageResultID int64) (*PageHTMLSource, error) {
	row := r.db.QueryRow(`
		SELECT id, page_results_id, raw_html_md5, raw_html, clean_html, created_at
		FROM page_html_source WHERE page_results_id = ?
	`, pageResultID)

	var p PageHTMLSource
	err := row.Scan(&p.ID, &p.PageResultID, &p.RawHTMLMD5, &p.RawHTML, &p.CleanHTML, &p.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("FindByPageResultID scan: %w", err)
	}
	return &p, nil
}

// FindByRawHTMLMD5 根据 raw_html_md5 查询记录
// @param rawHTMLMD5 RawHTML MD5 值
// @returns PageHTMLSource 记录和 error
func (r *PageHTMLRepo) FindByRawHTMLMD5(rawHTMLMD5 string) (*PageHTMLSource, error) {
	row := r.db.QueryRow(`
		SELECT id, page_results_id, raw_html_md5, raw_html, clean_html, created_at
		FROM page_html_source WHERE raw_html_md5 = ?
	`, rawHTMLMD5)

	var p PageHTMLSource
	err := row.Scan(&p.ID, &p.PageResultID, &p.RawHTMLMD5, &p.RawHTML, &p.CleanHTML, &p.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("FindByRawHTMLMD5 scan: %w", err)
	}
	return &p, nil
}

// FindByID 根据 id 查询记录
// @param id 主键
// @returns PageHTMLSource 记录和 error
func (r *PageHTMLRepo) FindByID(id int64) (*PageHTMLSource, error) {
	row := r.db.QueryRow(`
		SELECT id, page_results_id, raw_html_md5, raw_html, clean_html, created_at
		FROM page_html_source WHERE id = ?
	`, id)

	var p PageHTMLSource
	err := row.Scan(&p.ID, &p.PageResultID, &p.RawHTMLMD5, &p.RawHTML, &p.CleanHTML, &p.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("FindByID scan: %w", err)
	}
	return &p, nil
}

// Create 创建新记录
// @param pageResultID page_results 表主键
// @param rawHTML 原始 HTML（未清洗）
// @param cleanHTML 清洗后 HTML（可选）
// @returns 新记录 ID 和 error
func (r *PageHTMLRepo) Create(pageResultID int64, rawHTML, cleanHTML string) (int64, error) {
	rawHTMLMD5 := CalcMD5(rawHTML)
	result, err := r.db.Exec(`
		INSERT INTO page_html_source (page_results_id, raw_html_md5, raw_html, clean_html) VALUES (?, ?, ?, ?)
	`, pageResultID, rawHTMLMD5, rawHTML, cleanHTML)
	if err != nil {
		return 0, fmt.Errorf("Create page_html_source: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("Create LastInsertId: %w", err)
	}
	return id, nil
}

// GetOrCreate 查询记录，存在则返回，不存在则创建
// @param pageResultID page_results 表主键
// @param rawHTML 原始 HTML（未清洗）
// @param cleanHTML 清洗后 HTML（可选）
// @returns PageHTMLSource 记录（包含 ID）和 error
func (r *PageHTMLRepo) GetOrCreate(pageResultID int64, rawHTML, cleanHTML string) (*PageHTMLSource, error) {
	rawHTMLMD5 := CalcMD5(rawHTML)

	// 先根据 MD5 检查是否已存在
	existing, err := r.FindByRawHTMLMD5(rawHTMLMD5)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	// 不存在则创建
	id, err := r.Create(pageResultID, rawHTML, cleanHTML)
	if err != nil {
		return nil, err
	}

	return &PageHTMLSource{
		ID:           id,
		PageResultID: pageResultID,
		RawHTMLMD5:   rawHTMLMD5,
		RawHTML:      rawHTML,
		CleanHTML:    cleanHTML,
	}, nil
}

// UpdateCleanHTML 更新清洗后的 HTML
// @param id page_html_source 主键
// @param cleanHTML 新的清洗后 HTML
// @returns error
func (r *PageHTMLRepo) UpdateCleanHTML(id int64, cleanHTML string) error {
	_, err := r.db.Exec(`
		UPDATE page_html_source SET clean_html = ? WHERE id = ?
	`, cleanHTML, id)
	if err != nil {
		return fmt.Errorf("UpdateCleanHTML: %w", err)
	}
	return nil
}
