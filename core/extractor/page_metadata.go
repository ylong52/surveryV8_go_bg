package extractor

// PageMetadata 提取模块
//   extractPageMetadata - 从 cleanHTML 提取 page_id，组装 PageMetadata

import (
	"fmt"
	"time"

	"survey.cli/core/cleaner"
)

// PageMetadata 保存单次问卷页面的元信息
type PageMetadata struct {
	SurveyID       string
	PageID         string
	Timestamp      string
	QuestionsCount int
}

// ExtractPageMetadataFromRaw 从原始 HTML（保留 class）中提取 page_id，生成 PageMetadata
// 用于 page_id 提取需要保留 class 属性的场景
func ExtractPageMetadataFromRaw(surveyID, rawHTML string) (PageMetadata, error) {
	// 根据 surveyID 加载对应的正则配置
	patterns, err := cleaner.LoadPatternsForSurvey(surveyID)
	if err != nil {
		return PageMetadata{}, fmt.Errorf("extractPageMetadataFromRaw: %w", err)
	}

	// 使用配置的正则从原始 HTML 提取 page_id（不经过清洗）
	pageID, err := cleaner.ExtractPageIDWithPatterns(rawHTML, patterns)
	if err != nil {
		return PageMetadata{}, fmt.Errorf("extractPageMetadataFromRaw: %w", err)
	}
	return PageMetadata{
		SurveyID:       surveyID,
		PageID:         pageID,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		QuestionsCount: 0,
	}, nil
}

// extractPageMetadata 从 cleanHTML 中提取 page_id，生成 PageMetadata
// @param surveyID  问卷 ID（由调用方传入）
// @param cleanHTML 清洗后的 HTML 字符串
// @returns         PageMetadata, 或错误
func extractPageMetadata(surveyID, cleanHTML string) (PageMetadata, error) {
	// 根据 surveyID 加载对应的正则配置
	patterns, err := cleaner.LoadPatternsForSurvey(surveyID)
	if err != nil {
		return PageMetadata{}, fmt.Errorf("extractPageMetadata: %w", err)
	}

	// 使用配置的正则提取 page_id
	pageID, err := cleaner.ExtractPageIDWithPatterns(cleanHTML, patterns)
	if err != nil {
		return PageMetadata{}, fmt.Errorf("extractPageMetadata: %w", err)
	}
	return PageMetadata{
		SurveyID:       surveyID,
		PageID:         pageID,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		QuestionsCount: 0,
	}, nil
}
