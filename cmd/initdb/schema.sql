-- 创建 survey 表
CREATE TABLE IF NOT EXISTS survey (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id      VARCHAR(64)  NOT NULL UNIQUE,
    survey_id       VARCHAR(64)  NOT NULL,
    url             VARCHAR(254) NOT NULL,
    created_at      DATETIME     DEFAULT (datetime('now', 'localtime'))
);
CREATE INDEX IF NOT EXISTS idx_survey_session_id ON survey(session_id);

-- 创建 page_results 表
CREATE TABLE IF NOT EXISTS page_results (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    survey_id           INTEGER     NOT NULL,
    page_id             VARCHAR(128) NOT NULL,
    yaml_template       TEXT,
    recommend_answer    TEXT,
    interactions        TEXT,
    interactions_md5   VARCHAR(32)  NOT NULL,
    screenshot_path     TEXT,
    created_at          DATETIME    DEFAULT (datetime('now', 'localtime')),
    FOREIGN KEY (survey_id) REFERENCES survey(id),
    UNIQUE(survey_id, page_id, interactions_md5)
);
CREATE INDEX IF NOT EXISTS idx_page_results_survey_id ON page_results(survey_id);
CREATE INDEX IF NOT EXISTS idx_page_results_page_id ON page_results(page_id);
CREATE INDEX IF NOT EXISTS idx_page_results_interactions_md5 ON page_results(interactions_md5);

-- 创建 page_html_source 表
CREATE TABLE IF NOT EXISTS page_html_source (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    page_results_id  INTEGER     NOT NULL,
    raw_html         TEXT        NOT NULL,
    clean_html       TEXT,
    created_at       DATETIME    DEFAULT (datetime('now', 'localtime')),
    FOREIGN KEY (page_results_id) REFERENCES page_results(id)
);
CREATE INDEX IF NOT EXISTS idx_html_source_page_results_id ON page_html_source(page_results_id);

-- 创建 prompts 表（AI提示词配置表）
CREATE TABLE IF NOT EXISTS prompts (
    id              INTEGER PRIMARY KEY,
    prompt_key      TEXT        NOT NULL,
    prompt_type     TEXT        NOT NULL,
    question_type   TEXT,
    content         TEXT        NOT NULL,
    description     TEXT,
    yaml_template   TEXT,
    source_file     TEXT,
    version         INTEGER     NOT NULL DEFAULT 1,
    is_active      INTEGER     NOT NULL DEFAULT 1,
    created_at      TEXT        NOT NULL DEFAULT (datetime('now','localtime')),
    updated_at      TEXT        NOT NULL DEFAULT (datetime('now','localtime')),
    UNIQUE ("prompt_key" ASC)
);
CREATE INDEX IF NOT EXISTS idx_prompts_prompt_key ON prompts(prompt_key);
CREATE INDEX IF NOT EXISTS idx_prompts_prompt_type ON prompts(prompt_type);
CREATE INDEX IF NOT EXISTS idx_prompts_question_type ON prompts(question_type);

-- 创建 page_summary_cache 表（页面摘要缓存）
CREATE TABLE IF NOT EXISTS page_summary_cache (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    survey_id       TEXT        NOT NULL,
    page_id         TEXT        NOT NULL,
    page_summary    TEXT        NOT NULL,
    created_at      TEXT        NOT NULL DEFAULT (datetime('now','localtime')),
    UNIQUE (survey_id, page_id)
);
CREATE INDEX IF NOT EXISTS idx_page_summary_cache_survey_id ON page_summary_cache(survey_id);
