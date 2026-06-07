-- ============================================================
-- page_results 表新增 OCR 字段
-- 用于存储截图 OCR 识别结果
-- ============================================================

-- 新增 ocr_text 字段：原始 OCR 识别文字（全文）
ALTER TABLE page_results ADD COLUMN ocr_text TEXT;

-- 新增 ocr_json 字段：结构化 OCR 结果（JSON 格式，含题目/选项/用户选择）
ALTER TABLE page_results ADD COLUMN ocr_json TEXT;

-- 新增 ocr_model 字段：OCR 使用的视觉模型名称
ALTER TABLE page_results ADD COLUMN ocr_model VARCHAR(32);

-- 新增 ocr_verified 字段：OCR 结果与 AI 推荐答案的一致性比对结果
--   NULL   = 未比对
--   'match'    = OCR用户答案 与 recommend_answer 一致
--   'mismatch' = 不一致
--   'unclear'  = OCR 无法识别
ALTER TABLE page_results ADD COLUMN ocr_verified VARCHAR(16);

-- 新增 ocr_verified_at 字段：比对时间
ALTER TABLE page_results ADD COLUMN ocr_verified_at DATETIME;

-- 新增 ocr_error 字段：OCR 调用失败时的错误信息
ALTER TABLE page_results ADD COLUMN ocr_error TEXT;
