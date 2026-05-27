package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// SiteWithFingerprint 站点配置与指纹的内存缓存条目
type SiteWithFingerprint struct {
	Site
	Fingerprint *SiteFingerprint
}

var siteCache []SiteWithFingerprint

// InitSiteCache 从 DB 加载所有激活站点到内存（启动时调用一次）
// @returns error
func InitSiteCache() error {
	rows, err := GetDB().Query(`
		SELECT s.id, s.site_name, s.survey_name, s.url, s.is_active,
		       f.id, f.page_id_pattern_regex, f.next_pattern, f.wait_after_click
		FROM sites s
		LEFT JOIN site_page_fingerprints f ON f.site_id = s.id
		WHERE s.is_active = 1
		ORDER BY s.id
	`)
	if err != nil {
		return fmt.Errorf("InitSiteCache query: %w", err)
	}
	defer rows.Close()

	var cache []SiteWithFingerprint
	for rows.Next() {
		var (
			sID                          int64
			sSiteName, sSurveyName, sURL string
			isActive                     int64
			fID                          sql.NullInt64
			fRegexJSON, fNextPattern     sql.NullString
			fWaitAfterClick              sql.NullInt64
		)
		if err := rows.Scan(
			&sID, &sSiteName, &sSurveyName, &sURL, &isActive,
			&fID, &fRegexJSON, &fNextPattern, &fWaitAfterClick,
		); err != nil {
			return fmt.Errorf("InitSiteCache scan: %w", err)
		}

		site := Site{
			ID:         sID,
			SiteName:   sSiteName,
			SurveyName: sSurveyName,
			URL:        sURL,
			IsActive:   isActive == 1,
		}

		var fp *SiteFingerprint
		if fID.Valid {
			fp = &SiteFingerprint{
				ID:             fID.Int64,
				SiteID:         sID,
				NextPattern:    fNextPattern.String,
				WaitAfterClick: int(fWaitAfterClick.Int64),
			}
			if fRegexJSON.Valid && fRegexJSON.String != "" {
				if err := json.Unmarshal([]byte(fRegexJSON.String), &fp.PageIDPatternRegexes); err != nil {
					return fmt.Errorf("InitSiteCache json unmarshal for site %s: %w", sSurveyName, err)
				}
			}
		}

		cache = append(cache, SiteWithFingerprint{Site: site, Fingerprint: fp})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("InitSiteCache rows: %w", err)
	}
	siteCache = cache
	return nil
}

// GetAllActiveSites 返回所有激活站点（用于 URL 匹配）
// @returns []SiteWithFingerprint
func GetAllActiveSites() []SiteWithFingerprint {
	return siteCache
}

// FindSiteBySurveyName 按 survey_name 查找站点（大小写不敏感）
// @param surveyName 问卷标识
// @returns *SiteWithFingerprint，未找到返回 nil
func FindSiteBySurveyName(surveyName string) *SiteWithFingerprint {
	for i := range siteCache {
		if strings.EqualFold(siteCache[i].SurveyName, surveyName) {
			return &siteCache[i]
		}
	}
	return nil
}
