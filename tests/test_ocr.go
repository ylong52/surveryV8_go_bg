//go:build ignore

package main

// tests/test_ocr.go — 截图 OCR 识别测试（多模型投票机制）
//
// 使用方式：
//   go run tests/test_ocr.go image.png
//   go run tests/test_ocr.go image.png recommend_answer.json
//   go run tests/test_ocr.go img1.png img2.png recommend_answer.json
//
// 投票机制：Qwen + Claude 并行识别，逐题投票裁决；平票最多重试 2 次。
// recommend_answer.json 仅提取 subject + question_type，不向模型传 recommended_answers。

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"
)

// =============================================================================
// 配置
// =============================================================================

var volcesCfg = struct {
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
	Timeout     int
}{
	APIKey:      "sk-7fb1444ef4d64a3bb38ba71aa2c26860",
	BaseURL:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
	Model:       "qwen3-vl-flash",
	Temperature: 0.7,
	Timeout:     120,
}

var anthropicCfg = struct {
	APIKey      string
	BaseURL     string
	Model       string
	MaxTokens   int
	Temperature float64
	Timeout     int
}{
	APIKey:      "",
	BaseURL:     "https://api.anthropic.com",
	Model:       "claude-sonnet-4-6",
	MaxTokens:   4096,
	Temperature: 0.0,
	Timeout:     180,
}

// =============================================================================
// 类型定义
// =============================================================================

type interactionItem struct {
	Subject      string `json:"subject"`
	QuestionType string `json:"question_type"`
}

type OCRInteraction struct {
	Subject      string      `json:"subject"`
	QuestionType string      `json:"question_type"`
	UserSelected interface{} `json:"user_selected"`
	Note         string      `json:"note,omitempty"`
}

type MatrixRow struct {
	RowIndex    int    `json:"row_index"`
	RowText     string `json:"row_text"`
	SelectedCol any    `json:"selected_col"`
	Note        string `json:"note,omitempty"`
}

type ParsedResult struct {
	PageType  string
	Items     []OCRInteraction
	Rows      []MatrixRow
	ColLabels []string
	TotalRows int
	TotalCols int
	PageNote  string
}

type ModelResult struct {
	ModelName string
	Parsed    ParsedResult
	RawJSON   string
	Elapsed   time.Duration
	Error     error
}

type VoteStatus string

const (
	VoteConfirmed   VoteStatus = "confirmed"
	VoteMajority    VoteStatus = "majority"
	VoteUncertain   VoteStatus = "uncertain"
	VoteHumanReview VoteStatus = "human_review_required"
)

type VotedItem struct {
	Index        int
	Subject      string
	QuestionType string
	FinalAnswer  interface{}
	Status       VoteStatus
	ModelAnswers map[string]interface{}
	Note         string
	ColLabels    []string // matrix mode only
}

// =============================================================================
// 数据库提示词加载
// =============================================================================

func loadOCRSystemPrompt() (string, error) {
	db, err := sql.Open("sqlite", "data/survey202605.db")
	if err != nil {
		return "", err
	}
	defer db.Close()

	var content string
	err = db.QueryRow(
		`SELECT content FROM prompts WHERE prompt_key='ocr.system' AND is_active=1 LIMIT 1`,
	).Scan(&content)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("数据库中未找到 ocr.system")
	}
	return content, err
}

func loadOCRSystemPromptMatrix() (string, string, error) {
	db, err := sql.Open("sqlite", "data/survey202605.db")
	if err != nil {
		return "", "", err
	}
	defer db.Close()

	var content string
	err = db.QueryRow(
		`SELECT content FROM prompts WHERE prompt_key='ocr.system.matrix' AND is_active=1 LIMIT 1`,
	).Scan(&content)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("数据库中未找到 ocr.system.matrix")
	}
	return content, "", err
}

// =============================================================================
// HTTP 工具
// =============================================================================

func postJSON(url string, headers map[string]string, body interface{}, timeoutSec int) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP: %w", err)
	}
	defer resp.Body.Close()
	body2, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body2))
	}
	return body2, nil
}

// cleanJSON 从模型输出中提取 JSON，兼容以下情况：
//   - Qwen <think>...</think> 前缀
//   - Claude 在 JSON 前输出 prose（如 "I'll analyze..."）
//   - ```json ... ``` 代码块出现在任意位置
//   - JSON 裸露，前后有多余文字
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)

	// 剥离 Qwen <think> 块
	if idx := strings.Index(s, "</think>"); idx != -1 {
		s = strings.TrimSpace(s[idx+len("</think>"):])
	}

	// 优先从 ```json ... ``` 或 ``` ... ``` 代码块中提取（允许出现在任意位置）
	if start := strings.Index(s, "```json"); start != -1 {
		after := s[start+7:]
		if end := strings.Index(after, "```"); end != -1 {
			return strings.TrimSpace(after[:end])
		}
	}
	if start := strings.Index(s, "```"); start != -1 {
		after := strings.TrimLeft(s[start+3:], "\r\n")
		if end := strings.Index(after, "```"); end != -1 {
			return strings.TrimSpace(after[:end])
		}
	}

	// 兜底：定位第一个 { 或 [，截取到最后一个匹配的 } 或 ]
	if start := strings.IndexAny(s, "{["); start != -1 {
		s = s[start:]
		for i := len(s); i > 0; i-- {
			if s[i-1] == '}' || s[i-1] == ']' {
				return strings.TrimSpace(s[:i])
			}
		}
	}

	return strings.TrimSpace(s)
}

// =============================================================================
// API 调用 — Volces / Qwen（OpenAI-compatible）
// =============================================================================

func callVolcesVL(systemPrompt, userPrompt, imageBase64, mimeType string) (string, error) {
	imageURL := fmt.Sprintf("data:%s;base64,%s", mimeType, imageBase64)

	type imgSrc struct {
		URL string `json:"url"`
	}
	type part struct {
		Type string  `json:"type"`
		Text string  `json:"text,omitempty"`
		Img  *imgSrc `json:"image_url,omitempty"`
	}

	body := map[string]interface{}{
		"model": volcesCfg.Model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": []part{
				{Type: "text", Text: userPrompt},
				{Type: "image_url", Img: &imgSrc{URL: imageURL}},
			}},
		},
		"temperature": volcesCfg.Temperature,
		"max_tokens":  4096,
	}

	raw, err := postJSON(volcesCfg.BaseURL+"/chat/completions",
		map[string]string{"Authorization": "Bearer " + volcesCfg.APIKey},
		body, volcesCfg.Timeout)
	if err != nil {
		return "", err
	}

	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("Qwen 返回为空")
	}
	return cleanJSON(resp.Choices[0].Message.Content), nil
}

// =============================================================================
// API 调用 — Claude（Anthropic native API）
// =============================================================================

func callClaudeVL(systemPrompt, userPrompt, imageBase64, mimeType string) (string, error) {
	if anthropicCfg.APIKey == "" {
		return "", fmt.Errorf("anthropic API key 未配置")
	}
	mediaType := strings.SplitN(mimeType, ";", 2)[0]

	body := map[string]interface{}{
		"model":      anthropicCfg.Model,
		"max_tokens": anthropicCfg.MaxTokens,
		"system":     systemPrompt,
		"messages": []map[string]interface{}{
			{"role": "user", "content": []map[string]interface{}{
				{"type": "text", "text": userPrompt},
				{"type": "image", "source": map[string]interface{}{
					"type":       "base64",
					"media_type": mediaType,
					"data":       imageBase64,
				}},
			}},
		},
		"temperature": anthropicCfg.Temperature,
	}

	raw, err := postJSON(anthropicCfg.BaseURL+"/v1/messages",
		map[string]string{
			"x-api-key":         anthropicCfg.APIKey,
			"anthropic-version": "2023-06-01",
		},
		body, anthropicCfg.Timeout)
	if err != nil {
		return "", err
	}

	var resp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	for _, block := range resp.Content {
		if block.Type == "text" && block.Text != "" {
			return cleanJSON(block.Text), nil
		}
	}
	return "", fmt.Errorf("Claude 返回为空")
}

// =============================================================================
// 多模型并行调用
// =============================================================================

type modelCaller struct {
	name string
	fn   func(string, string, string, string) (string, error)
}

func getActiveModels() []modelCaller {
	models := []modelCaller{{name: "Qwen", fn: callVolcesVL}}
	if anthropicCfg.APIKey != "" {
		models = append(models, modelCaller{name: "Claude", fn: callClaudeVL})
	}
	return models
}

func callAllModels(sys, user, img64, mime string) []ModelResult {
	callers := getActiveModels()
	results := make([]ModelResult, len(callers))
	var wg sync.WaitGroup
	for i, c := range callers {
		wg.Add(1)
		go func(idx int, caller modelCaller) {
			defer wg.Done()
			start := time.Now()
			raw, err := caller.fn(sys, user, img64, mime)
			r := ModelResult{
				ModelName: caller.name,
				RawJSON:   raw,
				Elapsed:   time.Since(start),
				Error:     err,
			}
			if err == nil {
				if p, pErr := parseResult(raw); pErr == nil {
					r.Parsed = p
				} else {
					r.Error = pErr
				}
			}
			results[idx] = r
		}(i, c)
	}
	wg.Wait()
	return results
}

// =============================================================================
// 结果解析（统一矩阵 / 普通双格式）
// =============================================================================

func parseResult(raw string) (ParsedResult, error) {
	// 尝试矩阵格式
	var m struct {
		PageType  string      `json:"page_type"`
		TotalRows int         `json:"total_rows"`
		TotalCols int         `json:"total_cols"`
		ColLabels []string    `json:"col_labels"`
		Rows      []MatrixRow `json:"rows"`
		PageNote  string      `json:"page_note,omitempty"`
	}
	if err := json.Unmarshal([]byte(raw), &m); err == nil && m.PageType == "matrix" {
		return ParsedResult{
			PageType:  "matrix",
			Rows:      m.Rows,
			ColLabels: m.ColLabels,
			TotalRows: m.TotalRows,
			TotalCols: m.TotalCols,
			PageNote:  m.PageNote,
		}, nil
	}

	// 普通格式
	var n struct {
		Interactions []OCRInteraction `json:"interactions"`
		PageNote     string           `json:"page_note,omitempty"`
	}
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		return ParsedResult{}, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return ParsedResult{PageType: "normal", Items: n.Interactions, PageNote: n.PageNote}, nil
}

// =============================================================================
// 投票工具函数
// =============================================================================

var attentionKeywords = []string{
	"for data quality purposes", "please select", "attention check",
	"quality check", "select strongly agree", "select somewhat agree",
	"select agree", "choose agree", "must select", "make sure you select",
}

func isAttentionCheck(subject string) bool {
	low := strings.ToLower(subject)
	for _, kw := range attentionKeywords {
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}

// normalizeAns 将答案归一化为小写字符串，用于比较
func normalizeAns(v interface{}) string {
	switch val := v.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(val))
	case []interface{}:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				parts = append(parts, strings.ToLower(strings.TrimSpace(s)))
			}
		}
		return strings.Join(parts, ",")
	case float64:
		return fmt.Sprintf("%.0f", val)
	case int:
		return fmt.Sprintf("%d", val)
	case nil:
		return "null"
	default:
		return strings.ToLower(fmt.Sprintf("%v", val))
	}
}

// displayAns 将答案格式化为可读字符串（保留大小写）
func displayAns(v interface{}) string {
	switch val := v.(type) {
	case string:
		if val == "" {
			return "null（未识别）"
		}
		return val
	case []interface{}:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case float64:
		return fmt.Sprintf("col %.0f", val)
	case int:
		return fmt.Sprintf("col %d", val)
	case nil:
		return "null（未识别）"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// voteAnswers 投票裁决，返回 (最终答案, 状态, 备注)
func voteAnswers(answers map[string]interface{}) (interface{}, VoteStatus, string) {
	if len(answers) == 0 {
		return nil, VoteUncertain, "无模型结果"
	}
	// 单模型直接 confirmed
	if len(answers) == 1 {
		for _, v := range answers {
			return v, VoteConfirmed, ""
		}
	}

	// 统计归一化票数
	counts := map[string]int{}
	rawOf := map[string]interface{}{}
	for _, v := range answers {
		norm := normalizeAns(v)
		counts[norm]++
		rawOf[norm] = v
	}

	total := len(answers)

	// 全票一致
	if len(counts) == 1 {
		for norm := range counts {
			return rawOf[norm], VoteConfirmed, ""
		}
	}

	// 找最高票
	best, bestCount := "", 0
	for norm, cnt := range counts {
		if cnt > bestCount {
			bestCount = cnt
			best = norm
		}
	}

	// 各模型详情（用于 note）
	parts := make([]string, 0, len(answers))
	for name, v := range answers {
		parts = append(parts, fmt.Sprintf("%s=%s", name, displayAns(v)))
	}
	detail := strings.Join(parts, " / ")

	if bestCount*2 > total {
		return rawOf[best], VoteMajority, "多数裁决: " + detail
	}
	return nil, VoteUncertain, "需人工复核: " + detail
}

// =============================================================================
// 核心投票（单轮）
// =============================================================================

func runVoting(results []ModelResult) []VotedItem {
	var ok []ModelResult
	for _, r := range results {
		if r.Error == nil {
			ok = append(ok, r)
		}
	}
	if len(ok) == 0 {
		return nil
	}

	isMatrix := ok[0].Parsed.PageType == "matrix"
	colLabels := ok[0].Parsed.ColLabels

	// 取各模型中最大题目数
	total := 0
	for _, r := range ok {
		n := len(r.Parsed.Items)
		if isMatrix {
			n = len(r.Parsed.Rows)
		}
		if n > total {
			total = n
		}
	}

	items := make([]VotedItem, total)
	for i := 0; i < total; i++ {
		answers := map[string]interface{}{}
		subject, qType := "", ""

		if isMatrix {
			for _, r := range ok {
				if i < len(r.Parsed.Rows) {
					row := r.Parsed.Rows[i]
					answers[r.ModelName] = row.SelectedCol
					if subject == "" {
						subject = row.RowText
					}
				}
			}
			qType = "matrix_row"
		} else {
			for _, r := range ok {
				if i < len(r.Parsed.Items) {
					it := r.Parsed.Items[i]
					answers[r.ModelName] = it.UserSelected
					if subject == "" {
						subject = it.Subject
						qType = it.QuestionType
					}
				}
			}
		}

		// 陷阱题：跳过投票，忠实记录各模型原始结果
		if isAttentionCheck(subject) {
			noteParts := []string{"attention_check_detected"}
			for name, v := range answers {
				noteParts = append(noteParts, fmt.Sprintf("%s=%s", name, displayAns(v)))
			}
			items[i] = VotedItem{
				Index:        i + 1,
				Subject:      subject,
				QuestionType: qType,
				FinalAnswer:  answers[ok[0].ModelName],
				Status:       VoteConfirmed,
				ModelAnswers: answers,
				Note:         strings.Join(noteParts, " | "),
				ColLabels:    colLabels,
			}
			continue
		}

		finalAns, status, note := voteAnswers(answers)
		items[i] = VotedItem{
			Index:        i + 1,
			Subject:      subject,
			QuestionType: qType,
			FinalAnswer:  finalAns,
			Status:       status,
			ModelAnswers: answers,
			Note:         note,
			ColLabels:    colLabels,
		}
	}

	return items
}

// getAnswerAtIdx 从 ModelResult 中提取第 idx 题答案（0-based）
func getAnswerAtIdx(r ModelResult, idx int) interface{} {
	if r.Error != nil {
		return nil
	}
	if r.Parsed.PageType == "matrix" {
		if idx < len(r.Parsed.Rows) {
			return r.Parsed.Rows[idx].SelectedCol
		}
	} else {
		if idx < len(r.Parsed.Items) {
			return r.Parsed.Items[idx].UserSelected
		}
	}
	return nil
}

// =============================================================================
// 重试机制（对 uncertain 题目重新调用所有模型，最多 maxRetry 轮）
// =============================================================================

func runVotingWithRetry(sys, user, img64, mime string, initial []ModelResult, maxRetry int) []VotedItem {
	voted := runVoting(initial)
	if voted == nil {
		return nil
	}

	for retry := 0; retry < maxRetry; retry++ {
		var uncertain []int
		for i, item := range voted {
			if item.Status == VoteUncertain {
				uncertain = append(uncertain, i)
			}
		}
		if len(uncertain) == 0 {
			break
		}

		fmt.Printf("\n[重试 %d/%d] %d 题存疑，重新调用所有模型...\n",
			retry+1, maxRetry, len(uncertain))

		newResults := callAllModels(sys, user, img64, mime)
		resolved := 0

		for _, idx := range uncertain {
			newAns := map[string]interface{}{}
			for _, r := range newResults {
				if r.Error == nil {
					newAns[r.ModelName] = getAnswerAtIdx(r, idx)
				}
			}
			finalAns, status, note := voteAnswers(newAns)
			if status != VoteUncertain {
				voted[idx].FinalAnswer = finalAns
				voted[idx].Status = status
				voted[idx].Note = joinNote(voted[idx].Note,
					fmt.Sprintf("重试%d解决 | %s", retry+1, note))
				for k, v := range newAns {
					voted[idx].ModelAnswers[k] = v
				}
				resolved++
			}
		}
		fmt.Printf("[重试 %d] 解决 %d/%d 题\n", retry+1, resolved, len(uncertain))
	}

	// 仍 uncertain → human_review_required
	for i := range voted {
		if voted[i].Status == VoteUncertain {
			voted[i].Status = VoteHumanReview
		}
	}
	return voted
}

func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	return a + " | " + b
}

// =============================================================================
// 输出
// =============================================================================

func statusIcon(s VoteStatus) string {
	switch s {
	case VoteConfirmed:
		return "✅"
	case VoteMajority:
		return "🔶"
	case VoteHumanReview:
		return "🔴"
	default:
		return "❓"
	}
}

func printVotedResults(items []VotedItem) {
	sep := strings.Repeat("─", 64)
	fmt.Println()
	fmt.Println("┌─ 投票识别结果 ─────────────────────────────────────────────")
	for _, item := range items {
		subject := item.Subject
		if len(subject) > 50 {
			subject = subject[:47] + "..."
		}

		// 矩阵模式：将列号转换为列标签
		ansDisplay := displayAns(item.FinalAnswer)
		if item.QuestionType == "matrix_row" && item.FinalAnswer != nil {
			colNum := 0
			switch v := item.FinalAnswer.(type) {
			case float64:
				colNum = int(v)
			case int:
				colNum = v
			}
			if colNum > 0 && colNum <= len(item.ColLabels) {
				ansDisplay = fmt.Sprintf("col%d: %s", colNum, item.ColLabels[colNum-1])
			}
		}

		fmt.Printf("  %s [%02d] (%s) %s\n",
			statusIcon(item.Status), item.Index, item.QuestionType, subject)
		fmt.Printf("         → %s  [%s]\n", ansDisplay, string(item.Status))
		for name, v := range item.ModelAnswers {
			fmt.Printf("         · %-8s %s\n", name+":", displayAns(v))
		}
		if item.Note != "" {
			fmt.Printf("         📝 %s\n", item.Note)
		}
		fmt.Printf("  %s\n", sep)
	}
	fmt.Println("└──────────────────────────────────────────────────────────────")
}

func printSummary(items []VotedItem) {
	counts := map[VoteStatus]int{}
	for _, item := range items {
		counts[item.Status]++
	}

	fmt.Println()
	fmt.Println("┌─ 统计汇总 ──────────────────────────────────────────────────")
	fmt.Printf("  总题数:                      %d\n", len(items))
	fmt.Printf("  ✅ confirmed  (全部一致):    %d 题\n", counts[VoteConfirmed])
	fmt.Printf("  🔶 majority   (多数通过):    %d 题\n", counts[VoteMajority])
	fmt.Printf("  🔴 human_review (需人工复核): %d 题\n", counts[VoteHumanReview])

	var reviewList []VotedItem
	for _, item := range items {
		if item.Status == VoteHumanReview {
			reviewList = append(reviewList, item)
		}
	}
	if len(reviewList) > 0 {
		fmt.Println()
		fmt.Println("  存疑题目列表:")
		for _, item := range reviewList {
			subject := item.Subject
			if len(subject) > 40 {
				subject = subject[:37] + "..."
			}
			parts := make([]string, 0, len(item.ModelAnswers))
			for name, v := range item.ModelAnswers {
				parts = append(parts, fmt.Sprintf("%s:%s", name, displayAns(v)))
			}
			fmt.Printf("    [%02d] %s\n          → %s\n",
				item.Index, subject, strings.Join(parts, " / "))
		}
	}
	fmt.Println("└──────────────────────────────────────────────────────────────")
}

// =============================================================================
// 主处理函数
// =============================================================================

func processImage(imagePath, systemPrompt, systemPromptMatrix string,
	interactions []interactionItem, isMatrix bool, recSummary string) {

	// ── 1. 读取图片 ──────────────────────────────────────────────
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		for _, ext := range []string{".jpg", ".jpeg", ".png", ".PNG", ".JPG", ".JPEG"} {
			alt := strings.TrimSuffix(strings.TrimSuffix(imagePath, ".png"), ".PNG") + ext
			imgData, err = os.ReadFile(alt)
			if err == nil {
				imagePath = alt
				fmt.Printf("[提示] 自动切换为: %s\n", alt)
				break
			}
		}
		if err != nil {
			fmt.Printf("[错误] 读取图片失败: %v\n", err)
			return
		}
	}

	imageBase64 := base64.StdEncoding.EncodeToString(imgData)
	mimeType := http.DetectContentType(imgData)
	fmt.Printf("[图片] 大小=%d bytes | MIME=%s\n", len(imgData), mimeType)
	fmt.Printf("[题目] %s\n", recSummary)

	// ── 2. 选择提示词 ────────────────────────────────────────────
	var activeSystem, userPrompt string
	if isMatrix {
		activeSystem = systemPromptMatrix
		userPrompt = buildMatrixUserPrompt(interactions)
	} else {
		activeSystem = systemPrompt
		userPrompt = buildOCRUserPrompt(interactions)
	}

	// ── 3. 打印活动模型 ──────────────────────────────────────────
	callers := getActiveModels()
	names := make([]string, len(callers))
	for i, c := range callers {
		names[i] = c.name
	}
	mode := "普通"
	if isMatrix {
		mode = "矩阵"
	}
	fmt.Printf("[模型] %s模式 | 并行调用: %s\n", mode, strings.Join(names, " + "))

	// ── 4. 并行调用所有模型 ──────────────────────────────────────
	fmt.Printf("[调用] 正在并行调用 %d 个模型...\n", len(callers))
	t0 := time.Now()
	initial := callAllModels(activeSystem, userPrompt, imageBase64, mimeType)
	fmt.Printf("[完成] 总耗时=%.1fs（并行）\n", time.Since(t0).Seconds())

	for _, r := range initial {
		if r.Error != nil {
			fmt.Printf("  ✗ %-8s 失败: %v\n", r.ModelName, r.Error)
		} else {
			qCount := len(r.Parsed.Items)
			if r.Parsed.PageType == "matrix" {
				qCount = len(r.Parsed.Rows)
			}
			fmt.Printf("  ✓ %-8s %.1fs | %d 题\n", r.ModelName, r.Elapsed.Seconds(), qCount)
		}
	}

	// ── 5. 打印各模型原始 JSON ───────────────────────────────────
	for _, r := range initial {
		if r.Error != nil {
			continue
		}
		fmt.Println()
		fmt.Printf("┌─ %s 原始结果 ─────────────────────────────────────────────\n", r.ModelName)
		var buf bytes.Buffer
		if json.Valid([]byte(r.RawJSON)) && json.Indent(&buf, []byte(r.RawJSON), "  ", "  ") == nil {
			for _, line := range strings.Split(buf.String(), "\n") {
				fmt.Printf("  %s\n", line)
			}
		} else {
			fmt.Printf("  %s\n", r.RawJSON)
		}
		fmt.Println("└──────────────────────────────────────────────────────────────")
	}

	// ── 6. 投票裁决（含重试）────────────────────────────────────
	fmt.Println()
	fmt.Println("[投票] 开始多模型投票裁决（最多重试 5 次）...")
	var maxRetry int = 5
	voted := runVotingWithRetry(activeSystem, userPrompt, imageBase64, mimeType, initial, maxRetry)
	if voted == nil {
		fmt.Println("[错误] 所有模型均调用失败，无法进行投票")
		return
	}

	// ── 7. 输出结果 ─────────────────────────────────────────────
	printVotedResults(voted)
	printSummary(voted)
}

// =============================================================================
// User Prompt 构建
// =============================================================================

func buildOCRUserPrompt(interactions []interactionItem) string {
	var sb strings.Builder

	if len(interactions) > 0 {
		sb.WriteString(fmt.Sprintf("【参考题目清单】共 %d 题，仅供参考，如截图中的题目与此不同，优先以截图为准。\n\n", len(interactions)))
		for i, item := range interactions {
			sb.WriteString(fmt.Sprintf("题目%d:\n  题干: %s\n  题型: %s\n\n", i+1, item.Subject, item.QuestionType))
		}
	}

	sb.WriteString(`【识别要求】
扫描截图中的所有题目，逐题识别用户选中的答案。不要因为题目不在清单中就跳过。

【Likert 量表识别规则】（严禁违反）
- Likert 量表（如 Strongly Disagree / Disagree / Agree / Strongly Agree，或 Never / Rarely / Sometimes / Often / Always 等）属于普通单选题，只要截图中能看到选项文字且能判断哪个被选中，就必须输出 user_selected 为该选项的标签文字（如 "Strongly Agree"），而不是 null。
- 严禁在 Likert 量表上输出 null，除非截图中该题选项完全不可见。
- "likert_card" 特指下拉选择或卡片翻转题，其选项在截图中完全不可见，这种情况下才返回 null。

【陷阱题识别规则】（严禁违反）
- 识别包含以下关键词的题目为陷阱题：
  "For data quality purposes", "please select", "attention check", "quality check",
  "select strongly agree", "select somewhat agree", "select agree", "choose agree",
  "must select", "make sure you select"
- 陷阱题与普通题相同，必须以视觉圆点为准，忠实记录用户的实际勾选。
- 严禁用题干语义覆盖用户的真实选择。
- 在 note 中标记 "attention_check_detected"，并额外记录题干要求的选项，
  例如：note = "attention_check_detected | expected: Somewhat Agree"
- 视觉圆点优先级：视觉圆点 > 语义指令。此为强制规则，不可绕过。

【极性反转题识别规则】（严禁违反）
- 如果题干是负向感受陈述（如 creeps me out / bores me / annoys me / disgusts me / embarrasses me / upsets me / irritates me / makes me uncomfortable 等），选 "Agree" / "Strongly Agree" 表示用户认同该负面感受（即：我确实有此感受）；选 "Disagree" / "Strongly Disagree" 表示用户否认该负面感受（即：我没有此感受）。
- 严禁将负向题中的 "Agree" 理解为"不认可题干"，必须结合语义和圆点位置综合判断。
- 严禁在负向题上因语义歧义而输出 null 或跳题。

【相邻列识别规则】（严禁违反）
- Likert 四列场景（Strongly Disagree / Somewhat Disagree / Somewhat Agree / Strongly Agree），Somewhat Agree 和 Somewhat Disagree 为左右相邻列，最容易误判。
- 严禁在相邻列之间模棱两可就随便选一个，必须明确判断圆点位于中左（Somewhat Disagree）还是中右（Somewhat Agree）。
- 如果无法明确判断，必须输出 note 标记 "adjacent_col_uncertain"，严禁不写备注就随意填写。

【输出示例】
假设截图中有以下题目：
  题干: "I love watching action movies"
  选项: [Never] [Rarely] [Sometimes] [Often] [Always]
  用户选中了 "Often"

正确输出：
{
  "interactions": [
    {
      "subject": "I love watching action movies",
      "question_type": "single",
      "user_selected": "Often",
      "note": ""
    }
  ],
  "page_note": ""
}

请输出 JSON：
{
  "interactions": [
    {
      "subject": "截图中的题干原文",
      "question_type": "single/multi/text 等",
      "user_selected": "用户选择（单选string/多选array/文本string/null）",
      "note": "备注"
    }
  ],
  "page_note": "全页备注"
}`)
	return sb.String()
}

func buildMatrixUserPrompt(interactions []interactionItem) string {
	var sb strings.Builder

	if len(interactions) > 0 {
		sb.WriteString(fmt.Sprintf("【参考题目清单】共 %d 项，优先识别为矩阵题。\n", len(interactions)))
		for i, item := range interactions {
			sb.WriteString(fmt.Sprintf("行%d: %s\n", i+1, item.Subject))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`【识别任务】
请先判断截图是哪种结构：

A. 矩阵题（表格式）：表格左侧是行（子题题干），上方有统一列标签（如 Yes/No，或 Likert 选项）
B. 普通题（非矩阵）：各题目独立，无统一列标签，可能是单选、多选、文本等

请输出纯 JSON：
- 如果是矩阵题（判断依据：能识别出 2 列及以上且列标签适用于所有行）：
{
  "page_type": "matrix",
  "total_rows": <总行数>,
  "total_cols": <总列数>,
  "col_labels": ["列1标签", "列2标签", ...],
  "rows": [
    {"row_index": 1, "row_text": "子题题干原文", "selected_col": <列号整数或null>, "note": "备注"},
    {"row_index": 2, "row_text": "子题题干原文", "selected_col": <列号整数或null>, "note": "备注"}
  ]
}

- 如果不是矩阵题（无法识别出统一列标签，或列标签不适用于所有行）：
{
  "page_type": "normal",
  "interactions": [
    {"subject": "题干原文", "question_type": "single/multi/text", "user_selected": "用户选择", "note": "备注"},
    ...
  ],
  "page_note": ""
}

【陷阱题识别规则】（矩阵题适用）（严禁违反）
- 识别包含以下关键词的题目为陷阱题：
  "For data quality purposes", "please select", "attention check", "quality check",
  "select strongly agree", "select somewhat agree", "select agree", "choose agree",
  "must select", "make sure you select"
- 陷阱题与普通题相同，必须以视觉圆点为准，忠实记录用户的实际勾选。
- 严禁用题干语义覆盖用户的真实选择。
- 在 note 中标记 "attention_check_detected"，并额外记录题干要求的选项，
  例如：note = "attention_check_detected | expected: Somewhat Agree"
- 视觉圆点优先级：视觉圆点 > 语义指令。此为强制规则，不可绕过。

【极性反转题识别规则】（矩阵题适用）（严禁违反）
- 如果某行题干是负向感受陈述（如 creeps me out / bores me / annoys me / disgusts me / embarrasses me / upsets me / irritates me / makes me uncomfortable 等），选 "Agree" / "Strongly Agree" 表示用户认同该负面感受（即：我确实有此感受）；选 "Disagree" / "Strongly Disagree" 表示用户否认该负面感受（即：我没有此感受）。
- 严禁将负向题中的 "Agree" 理解为"不认可题干"，必须结合语义和圆点位置综合判断。
- 严禁在负向题上因语义歧义而输出 null 或跳题。

【相邻列识别规则】（矩阵题适用）（严禁违反）
- Likert 四列场景（Strongly Disagree / Somewhat Disagree / Somewhat Agree / Strongly Agree），Somewhat Agree 和 Somewhat Disagree 为左右相邻列，最容易误判。
- 严禁在相邻列之间模棱两可就随便选一个，必须明确判断圆点位于中左（Somewhat Disagree）还是中右（Somewhat Agree）。
- 如果无法明确判断，必须输出 note 标记 "adjacent_col_uncertain"，严禁不写备注就随意填写。

【输出要求】
- 必须输出纯 JSON，禁止包含 Markdown 代码块。
- 仔细扫描每一行，selected_col / user_selected 严禁留空或跳过任何一行。
`)
	return sb.String()
}

// =============================================================================
// 主程序 & 辅助函数
// =============================================================================

func main() {
	banner()
	loadConfig()

	systemPrompt, systemPromptMatrix := loadPrompts()

	args := os.Args[1:]
	if len(args) == 0 {
		showUsage()
		return
	}

	var recommendAnswerPath string
	var imagePaths []string
	for _, arg := range args {
		if strings.HasSuffix(strings.ToLower(arg), ".json") {
			recommendAnswerPath = arg
		} else {
			imagePaths = append(imagePaths, arg)
		}
	}
	if len(imagePaths) == 0 {
		showUsage()
		return
	}

	// 展开目录，收集文件
	var files []string
	for _, path := range imagePaths {
		info, err := os.Stat(path)
		if err != nil {
			fmt.Printf("  ⚠️  文件不存在，跳过: %s\n", path)
			continue
		}
		if info.IsDir() {
			entries, _ := os.ReadDir(path)
			for _, e := range entries {
				if !e.IsDir() {
					files = append(files, path+string(os.PathSeparator)+e.Name())
				}
			}
		} else {
			files = append(files, path)
		}
	}

	// 过滤图片格式
	var validFiles []string
	for _, path := range files {
		low := strings.ToLower(path)
		if strings.HasSuffix(low, ".png") || strings.HasSuffix(low, ".jpg") || strings.HasSuffix(low, ".jpeg") {
			validFiles = append(validFiles, path)
		} else {
			fmt.Printf("  ⚠️  非图片格式，跳过: %s\n", path)
		}
	}
	if len(validFiles) == 0 {
		fmt.Println("  ⚠️  没有找到任何有效图片文件")
		return
	}

	interactions, isMatrix, recSummary := loadRecommendAnswer(recommendAnswerPath)
	fmt.Printf("  📋 待处理文件数: %d\n", len(validFiles))
	sep := strings.Repeat("─", 64)

	for i, path := range validFiles {
		fmt.Printf("\n%s\n", sep)
		fmt.Printf("  [%d/%d] 📷 %s\n", i+1, len(validFiles), path)
		fmt.Printf("%s\n", sep)
		processImage(path, systemPrompt, systemPromptMatrix, interactions, isMatrix, recSummary)
	}

	fmt.Printf("\n%s\n", sep)
	fmt.Printf("  ✅ 全部完成，共处理 %d 个文件\n", len(validFiles))
	fmt.Printf("%s\n", sep)
}

func banner() {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║    截图 OCR 识别  —  多模型投票机制  tests/test_ocr.go    ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
}

func showUsage() {
	fmt.Println()
	fmt.Println("══════════════════════════════════════════════════════════════")
	fmt.Println("  💡 使用方式：")
	fmt.Println()
	fmt.Println("    go run tests/test_ocr.go image.png")
	fmt.Println("    go run tests/test_ocr.go image.png recommend.json")
	fmt.Println("    go run tests/test_ocr.go img1.png img2.png recommend.json")
	fmt.Println()
	fmt.Println("  recommend_answer.json 仅提取 subject + question_type")
	fmt.Println("══════════════════════════════════════════════════════════════")
}

func loadConfig() {
	data, err := os.ReadFile("configs/env.yaml")
	if err != nil {
		fmt.Printf("[警告] 读取 configs/env.yaml 失败: %v\n", err)
		return
	}
	var yml map[string]interface{}
	if err := yaml.Unmarshal(data, &yml); err != nil {
		return
	}

	if vv, ok := yml["volces_vision"].(map[string]interface{}); ok {
		if m, ok := vv["model"].(string); ok && m != "" {
			volcesCfg.Model = m
		}
		if key, ok := vv["api_key"].(string); ok && key != "" {
			volcesCfg.APIKey = key
		}
		if url, ok := vv["base_url"].(string); ok && url != "" {
			volcesCfg.BaseURL = url
		}
		if t, ok := vv["temperature"].(float64); ok {
			volcesCfg.Temperature = t
		}
		if to, ok := vv["timeout"].(int); ok && to > 0 {
			volcesCfg.Timeout = to
		}
	}
	if av, ok := yml["anthropic"].(map[string]interface{}); ok {
		if m, ok := av["model"].(string); ok && m != "" {
			anthropicCfg.Model = m
		}
		if key, ok := av["api_key"].(string); ok && key != "" {
			anthropicCfg.APIKey = key
		}
		if url, ok := av["base_url"].(string); ok && url != "" {
			anthropicCfg.BaseURL = url
		}
		if mt, ok := av["max_tokens"].(int); ok && mt > 0 {
			anthropicCfg.MaxTokens = mt
		}
		if t, ok := av["temperature"].(float64); ok {
			anthropicCfg.Temperature = t
		}
		if to, ok := av["timeout"].(int); ok && to > 0 {
			anthropicCfg.Timeout = to
		}
	}

	claudeStatus := "未配置（仅 Qwen 单模型）"
	if anthropicCfg.APIKey != "" {
		claudeStatus = "已配置，双模型投票"
	}
	fmt.Printf("\n[Qwen]   模型=%-20s temp=%.1f | timeout=%ds\n",
		volcesCfg.Model, volcesCfg.Temperature, volcesCfg.Timeout)
	fmt.Printf("[Claude] 模型=%-20s max_tokens=%d | %s\n",
		anthropicCfg.Model, anthropicCfg.MaxTokens, claudeStatus)
}

func loadPrompts() (string, string) {
	prompt, err := loadOCRSystemPrompt()
	if err != nil {
		fmt.Printf("[错误] 加载 ocr.system 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[提示词] ocr.system 已加载（%d 字符）\n", len(prompt))

	promptMatrix, _, _ := loadOCRSystemPromptMatrix()
	if promptMatrix != "" {
		fmt.Printf("[提示词] ocr.system.matrix 已加载（%d 字符）\n", len(promptMatrix))
	} else {
		fmt.Printf("[警告] ocr.system.matrix 未找到，使用通用提示词\n")
		promptMatrix = prompt
	}
	return prompt, promptMatrix
}

func loadRecommendAnswer(jsonPath string) ([]interactionItem, bool, string) {
	defaultItems := []interactionItem{
		{Subject: "What is your gender?", QuestionType: "single"},
		{Subject: "What is your age?", QuestionType: "single"},
		{Subject: "Are you Hispanic or Latino?", QuestionType: "single"},
		{Subject: "What is your racial background?", QuestionType: "single"},
		{Subject: "What is your current country of residence?", QuestionType: "single"},
		{Subject: "What is your zip code?", QuestionType: "text"},
	}

	if jsonPath == "" {
		return defaultItems, false, "(默认题目清单)"
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		fmt.Printf("[警告] 读取 %s 失败: %v，使用默认题目清单\n", jsonPath, err)
		return defaultItems[:2], false, "(默认题目清单)"
	}

	var wrapper struct {
		Data struct {
			PageMetadata struct {
				PageID       string `json:"page_id"`
				IsMatrixPage bool   `json:"is_matrix_page"`
			} `json:"page_metadata"`
			Interactions []struct {
				QuestionType string `json:"question_type"`
				Subject      string `json:"subject"`
			} `json:"interactions"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &wrapper); err != nil {
		fmt.Printf("[警告] 解析 %s 失败: %v，使用默认题目清单\n", jsonPath, err)
		return defaultItems[:2], false, "(默认题目清单)"
	}

	isMatrix := wrapper.Data.PageMetadata.IsMatrixPage
	for _, it := range wrapper.Data.Interactions {
		if strings.HasPrefix(it.QuestionType, "matrix") {
			isMatrix = true
			break
		}
	}

	items := make([]interactionItem, 0, len(wrapper.Data.Interactions))
	for _, it := range wrapper.Data.Interactions {
		items = append(items, interactionItem{Subject: it.Subject, QuestionType: it.QuestionType})
	}

	tag := "普通"
	if isMatrix {
		tag = "矩阵"
	}
	summary := fmt.Sprintf("%s (%s, %d 题)", jsonPath, tag, len(items))
	fmt.Printf("[题目] 从 %s 加载 %d 道题目\n", jsonPath, len(items))
	if pid := wrapper.Data.PageMetadata.PageID; pid != "" {
		fmt.Printf("[页面] page_id=%s\n", pid)
	}
	return items, isMatrix, summary
}
