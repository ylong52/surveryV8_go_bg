package database

import (
	"database/sql"
	"fmt"
)

// scanPageResult 统一扫描 page_results 行，处理 screenshot_path 的 NULL 值
func scanPageResult(row interface {
	Scan(dest ...any) error
}, p *PageResult) error {
	var screenshotPath sql.NullString
	err := row.Scan(
		&p.ID, &p.SurveyID, &p.PageID,
		&p.YamlTemplate, &p.RecommendAnswer,
		&p.PageFingerprint, &p.PageFingerprintMD5,
		&screenshotPath, &p.CreatedAt,
	)
	if err != nil {
		return err
	}
	p.ScreenshotPath = screenshotPath.String
	return nil
}

// PageResultRepo 提供 page_results 表的数据库操作
type PageResultRepo struct {
	db *sql.DB
}

// NewPageResultRepo 创建 PageResultRepo 实例
func NewPageResultRepo() *PageResultRepo {
	return &PageResultRepo{db: GetDB()}
}

// FindBySurveyID 根据 survey_id 查询所有页面结果
// @param surveyID survey 表主键
// @returns PageResult 列表和 error
func (r *PageResultRepo) FindBySurveyID(surveyID int64) ([]PageResult, error) {
	rows, err := r.db.Query(`
		SELECT id, survey_id, page_id, yaml_template, recommend_answer, page_fingerprint, page_fingerprint_md5, screenshot_path, created_at
		FROM page_results WHERE survey_id = ?
		ORDER BY created_at ASC
	`, surveyID)
	if err != nil {
		return nil, fmt.Errorf("FindBySurveyID query: %w", err)
	}
	defer rows.Close()

	var results []PageResult
	for rows.Next() {
		var p PageResult
		if err := scanPageResult(rows, &p); err != nil {
			return nil, fmt.Errorf("FindBySurveyID scan: %w", err)
		}
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("FindBySurveyID rows: %w", err)
	}
	return results, nil
}

// FindByPageID 根据 page_id 查询页面结果
// @param pageID 页面 ID
// @returns PageResult 记录和 error
func (r *PageResultRepo) FindByPageID(pageID string) (*PageResult, error) {
	row := r.db.QueryRow(`
		SELECT id, survey_id, page_id, yaml_template, recommend_answer, page_fingerprint, page_fingerprint_md5, screenshot_path, created_at
		FROM page_results WHERE page_id = ?
	`, pageID)

	var p PageResult
	if err := scanPageResult(row, &p); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("FindByPageID scan: %w", err)
	}
	return &p, nil
}

// Create 创建新记录
// @param surveyID survey 表主键
// @param pageID 页面 ID
// @param yamlTemplate YAML 模板（已清空）
// @param recommendAnswer AI 推荐答案（JSON 格式）
// @param pageFingerprint interactions 去除答案后的 JSON
// @param pageFingerprintMD5 PageFingerprint 的 MD5
// @returns 新记录 ID 和 error
func (r *PageResultRepo) Create(surveyID int64, pageID, yamlTemplate, recommendAnswer, pageFingerprint, pageFingerprintMD5 string) (int64, error) {
	result, err := r.db.Exec(`
		INSERT INTO page_results (survey_id, page_id, yaml_template, recommend_answer, page_fingerprint, page_fingerprint_md5) VALUES (?, ?, ?, ?, ?, ?)
	`, surveyID, pageID, yamlTemplate, recommendAnswer, pageFingerprint, pageFingerprintMD5)
	if err != nil {
		return 0, fmt.Errorf("Create page_result: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("Create LastInsertId: %w", err)
	}
	return id, nil
}

// UpdateRecommendAnswer 更新 AI 推荐答案
// @param id page_results 主键
// @param recommendAnswer 新的推荐答案（JSON 格式）
// @returns error
func (r *PageResultRepo) UpdateRecommendAnswer(id int64, recommendAnswer string) error {
	_, err := r.db.Exec(`
		UPDATE page_results SET recommend_answer = ? WHERE id = ?
	`, recommendAnswer, id)
	if err != nil {
		return fmt.Errorf("UpdateRecommendAnswer: %w", err)
	}
	return nil
}

// UpdateScreenshotPath 更新截图路径
// @param id page_results 主键
// @param screenshotPath 截图文件路径
// @returns error
func (r *PageResultRepo) UpdateScreenshotPath(id int64, screenshotPath string) error {
	_, err := r.db.Exec(`
		UPDATE page_results SET screenshot_path = ? WHERE id = ?
	`, screenshotPath, id)
	if err != nil {
		return fmt.Errorf("UpdateScreenshotPath: %w", err)
	}
	return nil
}

// FindByPageFingerprintMD5 根据 page_fingerprint_md5 查询页面结果
// @param pageFingerprintMD5 PageFingerprint 的 MD5
// @returns PageResult 记录和 error
func (r *PageResultRepo) FindByPageFingerprintMD5(pageFingerprintMD5 string) (*PageResult, error) {
	row := r.db.QueryRow(`
		SELECT id, survey_id, page_id, yaml_template, recommend_answer, page_fingerprint, page_fingerprint_md5, screenshot_path, created_at
		FROM page_results WHERE page_fingerprint_md5 = ?
	`, pageFingerprintMD5)

	var p PageResult
	if err := scanPageResult(row, &p); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("FindByPageFingerprintMD5 scan: %w", err)
	}
	return &p, nil
}

// FindBySurveyAndPage 根据 survey_id、page_id 和 page_fingerprint_md5 查询页面结果
// @param surveyID survey 表主键
// @param pageID 页面 ID
// @param pageFingerprintMD5 PageFingerprint 的 MD5
// @returns PageResult 记录和 error
func (r *PageResultRepo) FindBySurveyAndPage(surveyID int64, pageID, pageFingerprintMD5 string) (*PageResult, error) {
	row := r.db.QueryRow(`
		SELECT id, survey_id, page_id, yaml_template, recommend_answer, page_fingerprint, page_fingerprint_md5, screenshot_path, created_at
		FROM page_results WHERE survey_id = ? AND page_id = ? AND page_fingerprint_md5 = ?
	`, surveyID, pageID, pageFingerprintMD5)

	var p PageResult
	if err := scanPageResult(row, &p); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("FindBySurveyAndPage scan: %w", err)
	}
	return &p, nil
}

// GetOrCreate 查询记录，存在则返回，不存在则创建
// @param surveyID survey 表主键
// @param pageID 页面 ID
// @param yamlTemplate YAML 模板（已清空）
// @param recommendAnswer AI 推荐答案（JSON 格式）
// @param pageFingerprint interactions 去除答案后的 JSON
// @param pageFingerprintMD5 PageFingerprint 的 MD5
// @returns PageResult 记录（包含 ID）和 error
func (r *PageResultRepo) GetOrCreate(surveyID int64, pageID, yamlTemplate, recommendAnswer, pageFingerprint, pageFingerprintMD5 string) (*PageResult, error) {
	// 基于 survey_id + page_id + page_fingerprint_md5 判断是否已存在
	existing, err := r.FindBySurveyAndPage(surveyID, pageID, pageFingerprintMD5)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	// 不存在则创建
	id, err := r.Create(surveyID, pageID, yamlTemplate, recommendAnswer, pageFingerprint, pageFingerprintMD5)
	if err != nil {
		return nil, err
	}

	return &PageResult{
		ID:                 id,
		SurveyID:           surveyID,
		PageID:             pageID,
		YamlTemplate:       yamlTemplate,
		RecommendAnswer:    recommendAnswer,
		PageFingerprint:    pageFingerprint,
		PageFingerprintMD5: pageFingerprintMD5,
	}, nil
}

// FindYamlTemplate 查询 yaml_template（根据 survey_id 和 page_id）
// @param surveyID survey 表主键
// @param pageID 页面 ID
// @returns yaml_template 字符串（空表示未找到）和 error
func (r *PageResultRepo) FindYamlTemplate(surveyID int64, pageID string) (string, error) {
	row := r.db.QueryRow(`
		SELECT yaml_template FROM page_results
		WHERE survey_id = ? AND page_id = ? AND yaml_template IS NOT NULL AND yaml_template != ''
		LIMIT 1
	`, surveyID, pageID)

	var yamlTemplate string
	err := row.Scan(&yamlTemplate)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil // 未找到，返回空字符串
		}
		return "", fmt.Errorf("FindYamlTemplate scan: %w", err)
	}
	return yamlTemplate, nil
}

// UpdateOCRResult 更新 OCR 分析结果
// @param id page_results 主键
// @param ocrText 原始 OCR 文字
// @param ocrJSON 结构化 OCR JSON
// @param ocrModel 使用的视觉模型名称
// @param ocrVerified 比对结果：match/mismatch/unclear
// @param ocrError OCR 调用失败错误信息（无错误传空字符串）
// @returns error
func (r *PageResultRepo) UpdateOCRResult(id int64, ocrText, ocrJSON, ocrModel, ocrVerified, ocrError string) error {
	_, err := r.db.Exec(`
		UPDATE page_results
		   SET ocr_text = ?, ocr_json = ?, ocr_model = ?,
		       ocr_verified = ?, ocr_verified_at = datetime('now', 'localtime'),
		       ocr_error = ?
		 WHERE id = ?
	`, ocrText, ocrJSON, ocrModel, ocrVerified, ocrError, id)
	if err != nil {
		return fmt.Errorf("UpdateOCRResult: %w", err)
	}
	return nil
}

// FindAllFieldsBySurveyID 查询指定 survey 的所有页面结果，包含全部 OCR 字段
// @param surveyID survey 表主键
// @returns PageResult 列表（含 OCR 字段）和 error
func (r *PageResultRepo) FindAllFieldsBySurveyID(surveyID int64) ([]PageResult, error) {
	rows, err := r.db.Query(`
		SELECT id, survey_id, page_id, yaml_template, recommend_answer,
		       page_fingerprint, page_fingerprint_md5, screenshot_path,
		       ocr_text, ocr_json, ocr_model, ocr_verified, ocr_verified_at, ocr_error,
		       created_at
		  FROM page_results WHERE survey_id = ?
		 ORDER BY id ASC
	`, surveyID)
	if err != nil {
		return nil, fmt.Errorf("FindAllFieldsBySurveyID query: %w", err)
	}
	defer rows.Close()

	var results []PageResult
	for rows.Next() {
		var p PageResult
		var screenshotPath, ocrText, ocrJSON, ocrModel, ocrVerified, ocrVerifiedAt, ocrError sql.NullString
		err := rows.Scan(
			&p.ID, &p.SurveyID, &p.PageID,
			&p.YamlTemplate, &p.RecommendAnswer,
			&p.PageFingerprint, &p.PageFingerprintMD5, &screenshotPath,
			&ocrText, &ocrJSON, &ocrModel, &ocrVerified, &ocrVerifiedAt, &ocrError,
			&p.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("FindAllFieldsBySurveyID scan: %w", err)
		}
		p.ScreenshotPath = screenshotPath.String
		p.OCRText = ocrText.String
		p.OCRJSON = ocrJSON.String
		p.OCRModel = ocrModel.String
		p.OCRVerified = ocrVerified.String
		p.OCRVerifiedAt = ocrVerifiedAt.String
		p.OCRError = ocrError.String
		results = append(results, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("FindAllFieldsBySurveyID rows: %w", err)
	}
	return results, nil
}

// FindByID 根据 page_results 主键查询完整记录（含 OCR 字段）
// @param id page_results 主键
// @returns PageResult 记录和 error
func (r *PageResultRepo) FindByID(id int64) (*PageResult, error) {
	row := r.db.QueryRow(`
		SELECT id, survey_id, page_id, yaml_template, recommend_answer,
		       page_fingerprint, page_fingerprint_md5, screenshot_path,
		       ocr_text, ocr_json, ocr_model, ocr_verified, ocr_verified_at, ocr_error,
		       created_at
		  FROM page_results WHERE id = ?
	`, id)

	var p PageResult
	var ocrText, ocrJSON, ocrModel, ocrVerified, ocrVerifiedAt, ocrError sql.NullString
	err := row.Scan(
		&p.ID, &p.SurveyID, &p.PageID,
		&p.YamlTemplate, &p.RecommendAnswer,
		&p.PageFingerprint, &p.PageFingerprintMD5, &p.ScreenshotPath,
		&ocrText, &ocrJSON, &ocrModel, &ocrVerified, &ocrVerifiedAt, &ocrError,
		&p.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("FindByID scan: %w", err)
	}
	p.OCRText = ocrText.String
	p.OCRJSON = ocrJSON.String
	p.OCRModel = ocrModel.String
	p.OCRVerified = ocrVerified.String
	p.OCRVerifiedAt = ocrVerifiedAt.String
	p.OCRError = ocrError.String
	return &p, nil
}
