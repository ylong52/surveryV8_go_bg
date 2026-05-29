package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"survey.cli/core/cleaner"
	"survey.cli/core/database"
	"survey.cli/core/extractor"
)

// isNormalClose 判断 WebSocket 关闭是否为正常退出（不打印日志）
func isNormalClose(err error) bool {
	if websocket.IsUnexpectedCloseError(err,
		websocket.CloseNormalClosure,    // 1000 - 正常关闭
		websocket.CloseGoingAway,        // 1001 - 客户端离开
		websocket.CloseAbnormalClosure,  // 1006 - 异常关闭（客户端网络中断等）
		websocket.CloseNoStatusReceived, // 1005 - 无状态码
	) {
		return false
	}
	return true
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Msg struct {
	SessionID     string      `json:"session_id"`
	RawHTML       string      `json:"rawhtml"`
	URL           string      `json:"url"`
	Type          string      `json:"type"`
	DebugMode     string      `json:"debugmode"`
	RequestID     string      `json:"requestId,omitempty"`
	ImageBase64   string      `json:"image_base64,omitempty"`
	PageID        string      `json:"page_id,omitempty"`
	PageResultsID interface{} `json:"page_results_id,omitempty"`
	SurveyID      string      `json:"survey_id,omitempty"` // 问卷ID（可选，优先使用）
}

type Response struct {
	Code               int         `json:"code"`
	Message            string      `json:"message"`
	RequestID          string      `json:"requestId,omitempty"`
	PageResultsID      int64       `json:"page_results_id,omitempty"`
	YamlTemplateSource string      `json:"yaml_template_source,omitempty"`
	ElapsedSec         float64     `json:"elapsed_sec,omitempty"`
	Data               interface{} `json:"data,omitempty"`
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// 确保工作目录正确
	setProjectRoot()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// 初始化 page_summary 缓存
	basePath := os.Getenv("STORAGE_PATH")
	if basePath == "" {
		basePath = "./storage"
	}
	extractor.InitPageSummaryCache(basePath)

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if !isNormalClose(err) {
				log.Printf("read error: %v", err)
			}
			break
		}

		var msg Msg
		if err := json.Unmarshal(message, &msg); err != nil {
			sendError(conn, "JSON parse error: "+err.Error(), msg.RequestID)
			continue
		}

		// 入口日志：打印收到的消息类型（所有类型都打，避免 screenshot 被静默丢失）
		log.Printf("[wsHandler] recv type=%q session=%s requestId=%s", msg.Type, msg.SessionID, msg.RequestID)

		// ========== 类型路由分发 ==========
		switch msg.Type {
		case "analyze_request":
			handleAnalyzeRequest(conn, msg, basePath)
		case "submit":
			handleSubmit(conn, msg, basePath)
		case "screenshot":
			handleScreenshot(conn, msg, basePath)
		default:
			log.Printf("[wsHandler] unknown type=%q, msg=%s", msg.Type, string(message))
			sendError(conn, "unknown type: "+msg.Type, msg.RequestID)
		}
	}
}

func handleAnalyzeRequest(conn *websocket.Conn, msg Msg, basePath string) {
	startTime := time.Now()

	// 优先使用消息中已有的 surveyID，否则从 URL 提取
	surveyID := msg.SurveyID
	if surveyID == "" {
		surveyID = extractSurveyID(msg.URL, msg.RawHTML)
	}
	if surveyID == "" || surveyID == "unknown" {
		sendError(conn, "extractSurveyID: invalid URL format, survey_id not found", msg.RequestID)
		return
	}

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("            接收到的原始数据")
	fmt.Println("========================================")
	fmt.Printf("SessionID: %s\n", msg.SessionID)
	fmt.Printf("RequestID: %s\n", msg.RequestID)
	fmt.Printf("URL:       %s\n", msg.URL)
	fmt.Printf("SurveyID:  %s\n", surveyID)
	fmt.Printf("RawHTML长度: %d 字符\n", len(msg.RawHTML))
	fmt.Println()

	// 步骤 0: 数据库 - 提取/创建 survey 记录
	fmt.Println("========================================")
	fmt.Println(">>> 步骤 0: 数据库 - 提取/创建 survey 记录")
	fmt.Println("========================================")
	surveyRepo := database.NewSurveyRepo()
	surveyRecord, err := surveyRepo.GetOrCreate(msg.SessionID, surveyID, msg.URL)
	if err != nil {
		sendError(conn, "survey.GetOrCreate error: "+err.Error(), msg.RequestID)
		return
	}
	fmt.Printf("Survey ID: %d (session_name=%s)\n", surveyRecord.ID, surveyRecord.SessionName)

	// 步骤 1: 修复编码
	fixedHTML := fixEncoding([]byte(msg.RawHTML))

	// 步骤 2: 先从原始 HTML 提取 page_id（保留 class 属性）
	fmt.Println("========================================")
	fmt.Println(">>> 步骤 2: 提取 page_id（使用原始 HTML）")
	fmt.Println("========================================")
	pageMeta, err := extractor.ExtractPageMetadataFromRaw(surveyID, fixedHTML)
	if err != nil {
		sendError(conn, "ExtractPageMetadataFromRaw error: "+err.Error(), msg.RequestID)
		return
	}
	fmt.Printf("PageID:    %s\n", pageMeta.PageID)
	fmt.Println()

	// 步骤 3: 清洗 HTML（用于 YAML 提取）
	cleanHTML := cleaner.StripForDirectSurvey(fixedHTML)

	fmt.Println("========================================")
	fmt.Println(">>> 步骤 3: 提取 YAML 结构")
	fmt.Println("========================================")
	yamlResult, err := extractor.ExtractYamlStructureWithPageID(surveyID, surveyRecord.ID, pageMeta.PageID, cleanHTML, basePath)
	if err != nil {
		sendError(conn, "ExtractYamlStructureWithPageID error: "+err.Error(), msg.RequestID)
		return
	}
	fmt.Printf("PageID: %s\n", yamlResult.Meta.PageID)
	fmt.Printf("Source: %s\n", yamlResult.Source)

	// 调试：打印生成的 YAML 内容
	fmt.Println()
	fmt.Println("=== [DEBUG] Generated YAML Template ===")
	if len(yamlResult.YamlContent) >= 1000 {
		fmt.Printf("Length: %d chars\n", len(yamlResult.YamlContent))
		fmt.Printf("First 1000 chars:\n%s...\n", yamlResult.YamlContent[:1000])
	} else {
		fmt.Printf("Length: %d chars\n", len(yamlResult.YamlContent))
		fmt.Printf("Content:\n%s\n", yamlResult.YamlContent)
	}
	fmt.Println("=== [DEBUG] End YAML Template ===")
	fmt.Println()

	var finalJson extractor.FinalAnswerJson

	// 用于存入数据库的 JSON 字符串
	var answerResultJSON string

	fmt.Println("========================================")
	fmt.Println(">>> 步骤 4: 生成推荐答案")
	fmt.Println("========================================")

	// 始终使用 GenerateRealtimeAnswerOnlyWithMeta 生成推荐答案
	// 该函数使用 HTML + YAML（占位符）生成 interactions
	fmt.Println("调用 AI: GenerateRealtimeAnswerOnlyWithMeta...")
	answerResult, err := extractor.GenerateRealtimeAnswerOnlyWithMeta(pageMeta, cleanHTML, yamlResult.YamlContent, surveyID, surveyRecord.ID, basePath, yamlResult.DetectedTypes)
	if err != nil {
		sendError(conn, "GenerateRealtimeAnswerOnly error: "+err.Error(), msg.RequestID)
		return
	}
	finalJson = answerResult.FinalJson
	// 生成 JSON 字符串用于数据库存储
	rawJSON, _ := json.Marshal(answerResult.FinalJson)
	answerResultJSON = string(rawJSON)
	fmt.Printf("AI 响应来源: %s\n", answerResult.Source)
	fmt.Printf("推荐答案数量: %d\n", len(answerResult.Interactions))

	_, _ = json.MarshalIndent(finalJson, "", "  ")
	elapsed := time.Since(startTime)

	// 步骤 5: 数据库 - 保存 page_results 记录
	fmt.Println("========================================")
	fmt.Println(">>> 步骤 5: 数据库 - 保存 page_results 记录")
	fmt.Println("========================================")

	// 从 finalJson 中提取 interactions JSON，移除 recommended_answers 后作为页面指纹
	interactionsJSON, _ := json.Marshal(finalJson.Data.Interactions)
	pageFingerprint := extractor.StripRecommendedAnswers(string(interactionsJSON))
	pageFingerprintMD5 := database.CalcMD5(pageFingerprint)

	fmt.Printf("SQL: SELECT id FROM page_results WHERE survey_id=%d AND page_id=%q AND page_fingerprint_md5=%q\n",
		surveyRecord.ID, yamlResult.Meta.PageID, pageFingerprintMD5)

	pageResultRepo := database.NewPageResultRepo()
	existingRecord, _ := pageResultRepo.FindBySurveyAndPage(surveyRecord.ID, yamlResult.Meta.PageID, pageFingerprintMD5)
	isNewRecord := existingRecord == nil

	// db_cache 来源的旧模板可能含硬编码值，需过滤；ai 来源的固定模板已是纯占位符，无需处理
	var cleanYamlTemplate string
	if yamlResult.Source == "db_cache" {
		cleanYamlTemplate = extractor.StripYamlHardcodedValues(yamlResult.YamlContent)
	} else {
		cleanYamlTemplate = yamlResult.YamlContent
	}

	pageResultRecord, err := pageResultRepo.GetOrCreate(
		surveyRecord.ID,
		yamlResult.Meta.PageID,
		cleanYamlTemplate, // 保存过滤后的占位符模板
		answerResultJSON,
		pageFingerprint, // 移除 recommended_answers 后的 interactions JSON
		pageFingerprintMD5,
	)
	if err != nil {
		fmt.Printf("警告: page_results 保存失败: %v\n", err)
	} else {
		if isNewRecord {
			fmt.Printf("结果: INSERT → 新建记录 id=%d\n", pageResultRecord.ID)
		} else {
			fmt.Printf("结果: SELECT → 命中缓存 id=%d\n", pageResultRecord.ID)
		}
		fmt.Printf("记录详情:\n")
		fmt.Printf("  id                = %d\n", pageResultRecord.ID)
		fmt.Printf("  survey_id         = %d\n", pageResultRecord.SurveyID)
		fmt.Printf("  page_id           = %s\n", pageResultRecord.PageID)
		fmt.Printf("  page_fingerprint_md5 = %s\n", pageResultRecord.PageFingerprintMD5)
		fmt.Printf("  page_fingerprint     = %d 字符\n", len(pageResultRecord.PageFingerprint))
		fmt.Printf("  recommend_answer = %d 字符\n", len(pageResultRecord.RecommendAnswer))
		fmt.Printf("  screenshot_path   = %q\n", pageResultRecord.ScreenshotPath)

		// 步骤 5.5: 数据库 - 保存 page_result_prompt 记录（仅新建时插入）
		if isNewRecord {
			fmt.Println("========================================")
			fmt.Println(">>> 步骤 5.5: 数据库 - 保存 page_result_prompt 记录")
			fmt.Println("========================================")
			promptText := buildPromptForStorage(answerResult.SystemPrompt, answerResult.UserPrompt, 300)
			promptRepo := database.NewPageResultPromptRepo()
			promptID, err := promptRepo.Create(
				pageResultRecord.ID,
				promptText,
				answerResult.PageSummary,
			)
			if err != nil {
				fmt.Printf("警告: page_result_prompt 保存失败: %v\n", err)
			} else {
				fmt.Printf("结果: INSERT → 新建记录 id=%d\n", promptID)
				fmt.Printf("  page_result_id = %d\n", pageResultRecord.ID)
				fmt.Printf("  prompt         = %d 字符\n", len(promptText))
				fmt.Printf("  page_summary   = %d 字符\n", len(answerResult.PageSummary))
			}
		}

		// 步骤 6: 数据库 - 保存 page_html_source 记录
		fmt.Println("========================================")
		fmt.Println(">>> 步骤 6: 数据库 - 保存 page_html_source 记录")
		fmt.Println("========================================")
		rawHTMLMD5 := database.CalcMD5(msg.RawHTML)
		fmt.Printf("SQL: SELECT id FROM page_html_source WHERE raw_html_md5=%q\n", rawHTMLMD5)
		htmlRepo := database.NewPageHTMLRepo()
		existingHTML, _ := htmlRepo.FindByRawHTMLMD5(rawHTMLMD5)
		isNewHTML := existingHTML == nil

		htmlRecord, err := htmlRepo.GetOrCreate(pageResultRecord.ID, msg.RawHTML, cleanHTML)
		if err != nil {
			fmt.Printf("警告: page_html_source 保存失败: %v\n", err)
		} else {
			if isNewHTML {
				fmt.Printf("结果: INSERT → 新建记录 id=%d\n", htmlRecord.ID)
			} else {
				fmt.Printf("结果: SELECT → 命中缓存 id=%d\n", htmlRecord.ID)
			}
			fmt.Printf("记录详情:\n")
			fmt.Printf("  id               = %d\n", htmlRecord.ID)
			fmt.Printf("  page_results_id  = %d\n", htmlRecord.PageResultID)
			fmt.Printf("  raw_html_md5     = %s\n", htmlRecord.RawHTMLMD5)
			fmt.Printf("  raw_html         = %d 字符\n", len(htmlRecord.RawHTML))
			fmt.Printf("  clean_html       = %d 字符\n", len(htmlRecord.CleanHTML))
		}
	}

	pageResultsID := int64(0)
	if pageResultRecord != nil {
		pageResultsID = pageResultRecord.ID
	}
	response := Response{
		Code:               200,
		Message:            "success",
		RequestID:          msg.RequestID,
		PageResultsID:      pageResultsID,
		YamlTemplateSource: yamlResult.Source,
		ElapsedSec:         elapsed.Seconds(),
		Data:               finalJson.Data,
	}

	respBytes, _ := json.Marshal(response)
	if err := conn.WriteMessage(websocket.TextMessage, respBytes); err != nil {
		log.Printf("write error: %v", err)
		return
	}

	fmt.Printf(">>> 完成: session=%s, page_id=%s, source=%s, elapsed=%.2fs\n",
		msg.SessionID, yamlResult.Meta.PageID, yamlResult.Source, elapsed.Seconds())
}

func handleSubmit(conn *websocket.Conn, msg Msg, basePath string) {
	resp := Response{
		Code:      200,
		Message:   "success",
		RequestID: msg.RequestID,
		Data: map[string]interface{}{
			"session_id": msg.SessionID,
			"page_id":    msg.PageID,
			"status":     "saving",
		},
	}
	respBytes, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, respBytes)
}

// handleScreenshot 处理截图上传请求
// page_results_id 为空时记录错误日志并终止程序
// 文件名格式: {page_results_id}.jpg
func handleScreenshot(conn *websocket.Conn, msg Msg, basePath string) {
	fmt.Println("========================================")
	fmt.Println(">>> 接收截图请求")
	fmt.Println("========================================")
	fmt.Printf("SessionID:       %s\n", msg.SessionID)
	fmt.Printf("PageResultsID:   %v\n", msg.PageResultsID)
	fmt.Printf("URL:             %s\n", msg.URL)
	fmt.Printf("ImageBase64长度: %d 字符\n", len(msg.ImageBase64))
	fmt.Println()

	// 将 PageResultsID 从 interface{} 转为 string（兼容 number 和 string 两种 JSON 格式）
	pageResultsIDStr := fmt.Sprintf("%v", msg.PageResultsID)
	if pageResultsIDStr == "" || pageResultsIDStr == "<nil>" {
		fmt.Printf("错误: page_results_id 为空，程序终止\n")
		log.Fatalf("handleScreenshot: page_results_id is required for session=%s", msg.SessionID)
	}

	pageResultsID, err := strconv.ParseInt(pageResultsIDStr, 10, 64)
	if err != nil || pageResultsID == 0 {
		fmt.Printf("错误: 无效的 page_results_id=%q，程序终止\n", pageResultsIDStr)
		log.Fatalf("handleScreenshot: invalid page_results_id=%q for session=%s: %v", pageResultsIDStr, msg.SessionID, err)
	}

	fmt.Println("========================================")
	fmt.Println(">>> 步骤 1: 保存截图文件")
	fmt.Println("========================================")
	screenshotDir := filepath.Join(basePath, "screenshots", msg.SessionID)
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		fmt.Printf("错误: mkdir 失败，程序终止: %v\n", err)
		log.Fatalf("handleScreenshot mkdir error: %v", err)
	}

	filename := fmt.Sprintf("%d.jpg", pageResultsID)
	filePath := filepath.Join(screenshotDir, filename)
	fmt.Printf("目标路径: %s\n", filePath)

	imageData, err := base64.StdEncoding.DecodeString(msg.ImageBase64)
	if err != nil {
		fmt.Printf("错误: Base64 解码失败，程序终止: %v\n", err)
		log.Fatalf("handleScreenshot base64 decode error for session=%s: %v", msg.SessionID, err)
	}
	fmt.Printf("解码后大小: %d bytes\n", len(imageData))

	if err := os.WriteFile(filePath, imageData, 0644); err != nil {
		fmt.Printf("错误: 写入文件失败，程序终止: %v\n", err)
		log.Fatalf("handleScreenshot write file error: %v", err)
	}
	fmt.Printf("结果: 文件写入成功\n")
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println(">>> 步骤 2: 回写数据库 screenshot_path")
	fmt.Println("========================================")
	fmt.Printf("SQL: UPDATE page_results SET screenshot_path=%q WHERE id=%d\n", filePath, pageResultsID)
	repo := database.NewPageResultRepo()
	if err := repo.UpdateScreenshotPath(pageResultsID, filePath); err != nil {
		fmt.Printf("错误: 数据库更新失败，程序终止: %v\n", err)
		log.Fatalf("handleScreenshot UpdateScreenshotPath error: %v", err)
	}
	fmt.Printf("结果: UPDATE 成功，影响 page_results_id=%d\n", pageResultsID)
	fmt.Printf("截图路径: %s\n", filePath)
	fmt.Println()
	fmt.Printf(">>> 截图处理完成: %s (%d bytes)\n", filePath, len(imageData))
}

func sendError(conn *websocket.Conn, errMsg string, requestID string) {
	resp := Response{Code: 500, Message: errMsg, RequestID: requestID}
	bytes, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, bytes)
}

func extractSurveyID(rawURL string, rawHTML string) string {
	if rawURL == "" && rawHTML == "" {
		return ""
	}

	sites := database.GetAllActiveSites()
	fmt.Printf("[extractSurveyID] rawURL=%q sites=%d\n", rawURL, len(sites))

	if rawURL != "" {
		cleanedURL := regexp.MustCompile(`(https?://[^/]+[/a-zA-Z0-9_.-]*)[^a-zA-Z0-9_/.-]*$`).ReplaceAllString(rawURL, "$1")
		fmt.Printf("[extractSurveyID] cleanedURL=%q\n", cleanedURL)
		for _, sw := range sites {
			fmt.Printf("[extractSurveyID] checking site URL=%s surveyName=%s\n", sw.URL, sw.SurveyName)
			if strings.HasPrefix(cleanedURL, sw.URL) || cleanedURL == sw.URL {
				fmt.Printf("[extractSurveyID] URL matched! returning %s\n", sw.SurveyName)
				return sw.SurveyName
			}
		}
		fmt.Printf("[extractSurveyID] URL matching failed for rawURL=%q\n", rawURL)
	}

	fmt.Printf("[extractSurveyID] no match found, returning empty\n")
	return ""
}

func main() {
	// 设置工作目录为项目根目录
	setProjectRoot()

	// 初始化数据库
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/survey202605.db"
	}
	if err := database.Init(dbPath); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer database.Close()
	fmt.Printf("数据库已初始化: %s\n", dbPath)

	if err := database.InitSiteCache(); err != nil {
		log.Fatalf("站点缓存初始化失败: %v", err)
	}
	fmt.Printf("站点缓存已加载: %d 个站点\n", len(database.GetAllActiveSites()))

	port := os.Getenv("WS_PORT")
	if port == "" {
		port = "10501"
	}

	http.HandleFunc("/ws", wsHandler)

	// 健康检查接口
	http.HandleFunc("/api/v2/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	fmt.Printf("WS server running on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// setProjectRoot 设置工作目录为项目根目录
func setProjectRoot() {
	// 0. 优先检查当前目录是否有 go.mod（最可靠，兼容 go run）
	cwd, err := os.Getwd()
	fmt.Printf("[setProjectRoot] current dir: %s\n", cwd)
	if err == nil {
		// 检查是否是项目根目录（需要同时有 go.mod、configs/ 和 core/）
		goModPath := filepath.Join(cwd, "go.mod")
		configsPath := filepath.Join(cwd, "configs")
		corePath := filepath.Join(cwd, "core")
		if _, err := os.Stat(goModPath); err == nil {
			if _, err1 := os.Stat(configsPath); err1 == nil {
				if _, err2 := os.Stat(corePath); err2 == nil {
					// 当前目录已是项目根目录，无需切换
					fmt.Printf("[setProjectRoot] project root found (has go.mod, configs, core), no change needed\n")
					return
				}
			}
		}
	}

	// 1. 优先使用环境变量
	if envRoot := os.Getenv("PROJECT_ROOT"); envRoot != "" {
		os.Chdir(envRoot)
		fmt.Printf("[setProjectRoot] changed to PROJECT_ROOT: %s\n", envRoot)
		return
	}

	// 2. 尝试从可执行文件路径推断
	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("[setProjectRoot] failed to get executable path: %v\n", err)
		return
	}
	fmt.Printf("[setProjectRoot] executable: %s\n", exePath)

	exeDir := filepath.Dir(exePath)
	baseName := filepath.Base(exeDir)

	// 如果在 ws_server 目录中，向上一级
	if baseName == "ws_server" {
		os.Chdir(filepath.Dir(exeDir))
		newCwd, _ := os.Getwd()
		fmt.Printf("[setProjectRoot] changed from ws_server dir to: %s\n", newCwd)
		return
	}

	// 如果在临时目录（go run），尝试向上查找包含 go.mod 的目录
	// Go run temp path patterns: contains "go-build" or parent contains "go-build"
	dir := exeDir
	for i := 0; i < 8; i++ {
		goModPath := filepath.Join(dir, "go.mod")
		configsPath := filepath.Join(dir, "configs")
		corePath := filepath.Join(dir, "core")
		if _, err := os.Stat(goModPath); err == nil {
			if _, err1 := os.Stat(configsPath); err1 == nil {
				if _, err2 := os.Stat(corePath); err2 == nil {
					os.Chdir(dir)
					newCwd, _ := os.Getwd()
					fmt.Printf("[setProjectRoot] found project root, changed to: %s\n", newCwd)
					return
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	fmt.Printf("[setProjectRoot] WARNING: could not find project root!\n")
}

// buildPromptForStorage 组装存入数据库的 prompt 文本
// user prompt 中的 HTML fence 内容截取前 htmlMaxLen 字符，避免存储过大
func buildPromptForStorage(systemPrompt, userPrompt string, htmlMaxLen int) string {
	const htmlFenceOpen = "```html\n"
	const htmlFenceClose = "\n```"

	truncatedUser := userPrompt
	if idx := strings.Index(userPrompt, htmlFenceOpen); idx != -1 {
		htmlBodyStart := idx + len(htmlFenceOpen)
		tail := userPrompt[htmlBodyStart:]
		if endIdx := strings.Index(tail, htmlFenceClose); endIdx != -1 {
			htmlBody := tail[:endIdx]
			var truncatedBody string
			if len(htmlBody) > htmlMaxLen {
				truncatedBody = htmlBody[:htmlMaxLen] + "...[truncated]"
			} else {
				truncatedBody = htmlBody
			}
			truncatedUser = userPrompt[:htmlBodyStart] + truncatedBody + tail[endIdx:]
		}
	}

	return "=== System Prompt ===\n" + systemPrompt + "\n\n=== User Prompt ===\n" + truncatedUser
}

// fixEncoding 检测 CP1252 特征字节并替换为对应 UTF-8 字符
func fixEncoding(b []byte) string {
	cp1252map := map[byte]rune{
		0x82: '\x82',   // comma
		0x91: '\u2018', // LEFT SINGLE QUOTATION MARK
		0x92: '\u2019', // RIGHT SINGLE QUOTATION MARK
		0x93: '\u201C', // LEFT DOUBLE QUOTATION MARK
		0x94: '\u201D', // RIGHT DOUBLE QUOTATION MARK
		0x96: '\u2013', // EN DASH
		0x97: '\u2014', // EM DASH
	}

	needFix := false
	for _, byt := range b {
		if _, ok := cp1252map[byt]; ok {
			needFix = true
			break
		}
	}
	if !needFix {
		return string(b)
	}

	out := make([]rune, 0, len(b))
	for _, byt := range b {
		if r, ok := cp1252map[byt]; ok {
			out = append(out, r)
		} else {
			out = append(out, rune(byt))
		}
	}
	return string(out)
}
