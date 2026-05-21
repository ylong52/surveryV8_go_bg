package storage

// 统一存取引擎，提供 Storage.Save() 调用

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Storage 封装文件写入路径与命名规则
type Storage struct {
	basePath string
}

// New 构造 Storage，basePath 为外挂存储根目录
func New(basePath string) *Storage {
	return &Storage{basePath: basePath}
}

// Save 将内容写入 basePath/<surveyID>/<timestamp>_<name>.txt
// @param surveyID  问卷标识
// @param name      文件用途标签（如 "html" / "yaml" / "log"）
// @param content   待写入内容
// @returns         实际写入路径，或错误
func (s *Storage) Save(surveyID, name, content string) (string, error) {
	dir := filepath.Join(s.basePath, surveyID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	ts := time.Now().Format("20060102_150405")
	path := filepath.Join(dir, fmt.Sprintf("%s_%s.txt", ts, name))
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

// SaveHTMLPage 将清洗后的 HTML 保存到 cache/<surveyID>/<pageID>.html
// @param surveyID  问卷 ID（如 "macalester_2026_prospective_families"）
// @param pageID    清洗后的页面标识（不含扩展名）
// @param html      HTML 内容
// @returns         实际写入路径，或错误
func (s *Storage) SaveHTMLPage(surveyID, pageID, html string) (string, error) {
	if pageID == "" {
		return "", fmt.Errorf("SaveHTMLPage: pageID cannot be empty")
	}
	// 保存到 cache 子目录
	dir := filepath.Join(s.basePath, "cache", surveyID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, pageID+".html")
	if err := os.WriteFile(path, []byte(html), 0644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}
