package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"

	"survey.cli/core/cleaner"
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
	SessionID   string `json:"session_id"`
	RawHTML     string `json:"rawhtml"`
	URL         string `json:"url"`
	Type        string `json:"type"`
	DebugMode   string `json:"debugmode"`
	RequestID   string `json:"requestId,omitempty"`
	ImageBase64 string `json:"imageBase64,omitempty"`
	PageID      string `json:"page_id,omitempty"`
}

type Response struct {
	Code       int         `json:"code"`
	Message    string      `json:"message"`
	RequestID  string      `json:"requestId,omitempty"`
	YamlSource string      `json:"yaml_source,omitempty"`
	ElapsedSec float64     `json:"elapsed_sec,omitempty"`
	Data       interface{} `json:"data,omitempty"`
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
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

		// ========== 类型路由分发 ==========
		switch msg.Type {
		case "analyze_request":
			handleAnalyzeRequest(conn, msg, basePath)
		case "submit":
			handleSubmit(conn, msg, basePath)
		default:
			sendError(conn, "unknown type: "+msg.Type, msg.RequestID)
		}
	}
}

func handleAnalyzeRequest(conn *websocket.Conn, msg Msg, basePath string) {
	startTime := time.Now()

	surveyID := extractSurveyID(msg.URL, msg.RawHTML)
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
	yamlResult, err := extractor.ExtractYamlStructureWithPageID(surveyID, pageMeta.PageID, cleanHTML, basePath)
	if err != nil {
		sendError(conn, "ExtractYamlStructureWithPageID error: "+err.Error(), msg.RequestID)
		return
	}
	fmt.Printf("PageID:    %s\n", yamlResult.Meta.PageID)
	fmt.Printf("Source:    %s\n", yamlResult.Source)
	fmt.Printf("CachePath: %s\n", yamlResult.CachePath)

	var finalJson extractor.FinalAnswerJson

	fmt.Println("========================================")
	fmt.Println(">>> 步骤 4: 生成推荐答案")
	fmt.Println("========================================")

	if yamlResult.Source == "cache" {
		fmt.Println("Source: cache（使用缓存模板）")
		fmt.Println("调用 AI: GenerateRealtimeAnswerOnlyWithMeta...")
		answerResult, err := extractor.GenerateRealtimeAnswerOnlyWithMeta(pageMeta, cleanHTML, yamlResult.YamlContent, surveyID, msg.SessionID, basePath)
		if err != nil {
			sendError(conn, "GenerateRealtimeAnswerOnly error: "+err.Error(), msg.RequestID)
			return
		}
		finalJson = answerResult.FinalJson
		fmt.Printf("AI 响应来源: %s\n", answerResult.Source)
		fmt.Printf("推荐答案数量: %d\n", len(answerResult.Interactions))
	} else {
		fmt.Println("Source: ai（实时生成）")
		fmt.Println("调用 AI: GenerateRealtimeYamlWithAnswersWithMeta...")
		answerResult, err := extractor.GenerateRealtimeYamlWithAnswersWithMeta(pageMeta, cleanHTML, yamlResult.YamlContent, surveyID, msg.SessionID, basePath, extractor.PageSettings{})
		if err != nil {
			sendError(conn, "GenerateRealtimeYamlWithAnswersWithMeta error: "+err.Error(), msg.RequestID)
			return
		}
		finalJson = answerResult.FinalJson
		fmt.Printf("AI 响应来源: %s\n", answerResult.Source)
		fmt.Printf("推荐答案数量: %d\n", len(answerResult.Answers))
	}

	_, _ = json.MarshalIndent(finalJson, "", "  ")
	elapsed := time.Since(startTime)

	response := Response{
		Code:       200,
		Message:    "success",
		RequestID:  msg.RequestID,
		YamlSource: yamlResult.Source,
		ElapsedSec: elapsed.Seconds(),
		Data:       finalJson.Data,
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
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("handleSubmit panic: %v", r)
			}
		}()

		screenshotDir := filepath.Join(basePath, "screenshots", msg.SessionID)
		if err := os.MkdirAll(screenshotDir, 0755); err != nil {
			log.Printf("mkdir error: %v", err)
			return
		}

		filename := fmt.Sprintf("%s_%s.jpg", msg.PageID, time.Now().Format("20060102150405"))
		filePath := filepath.Join(screenshotDir, filename)

		imageData, err := base64.StdEncoding.DecodeString(msg.ImageBase64)
		if err != nil {
			log.Printf("base64 decode error: %v", err)
			return
		}

		if err := os.WriteFile(filePath, imageData, 0644); err != nil {
			log.Printf("write file error: %v", err)
			return
		}

		fmt.Printf(">>> 截图已保存: %s\n", filePath)
	}()

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

func sendError(conn *websocket.Conn, errMsg string, requestID string) {
	resp := Response{Code: 500, Message: errMsg, RequestID: requestID}
	bytes, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, bytes)
}

func extractSurveyID(rawURL string, rawHTML string) string {
	if rawURL == "" && rawHTML == "" {
		return ""
	}

	configPath := findConfigPath()
	fmt.Printf("[extractSurveyID] rawURL=%q configPath=%s\n", rawURL, configPath)

	sites, err := loadSitesFromConfig(configPath)
	if err != nil {
		fmt.Printf("[extractSurveyID] loadSitesFromConfig error: %v\n", err)
		return ""
	}

	// Step 1: URL prefix/exact match (most reliable)
	if rawURL != "" {
		for _, site := range sites {
			fmt.Printf("[extractSurveyID] checking site URL=%s surveyID=%s\n", site.URL, site.SurveyID)
			if strings.HasPrefix(rawURL, site.URL) || rawURL == site.URL {
				fmt.Printf("[extractSurveyID] URL matched! returning %s\n", site.SurveyID)
				return site.SurveyID
			}
		}
		fmt.Printf("[extractSurveyID] URL matching failed for rawURL=%q\n", rawURL)
	}

	fmt.Printf("[extractSurveyID] no match found, returning empty\n")
	return ""
}

type SiteConfig struct {
	Keyword  string `yaml:"keyword"`
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	SurveyID string `yaml:"survey_id"`
}

type FingerprintsConfig struct {
	Sites []SiteConfig `yaml:"sites"`
}

func loadSitesFromConfig(configPath string) ([]SiteConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var cfg FingerprintsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return cfg.Sites, nil
}

func findConfigPath() string {
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	paths := []string{
		filepath.Join(exeDir, "configs", "page-fingerprints.yaml"),
		"configs/page-fingerprints.yaml",
		filepath.Join(".", "configs", "page-fingerprints.yaml"),
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "configs/page-fingerprints.yaml"
}

func main() {
	// 设置工作目录为项目根目录
	setProjectRoot()

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
	if err == nil {
		if _, err := os.Stat(filepath.Join(cwd, "go.mod")); err == nil {
			// 当前目录已是项目根目录，无需切换
			return
		}
	}

	// 1. 优先使用环境变量
	if envRoot := os.Getenv("PROJECT_ROOT"); envRoot != "" {
		os.Chdir(envRoot)
		return
	}

	// 2. 尝试从可执行文件路径推断
	exePath, err := os.Executable()
	if err != nil {
		return
	}

	exeDir := filepath.Dir(exePath)
	baseName := filepath.Base(exeDir)

	// 如果在 ws_server 目录中，向上一级
	if baseName == "ws_server" {
		os.Chdir(filepath.Dir(exeDir))
		return
	}

	// 如果在临时目录（go run），尝试向上查找包含 go.mod 的目录
	// Go run temp path patterns: contains "go-build" or parent contains "go-build"
	dir := exeDir
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			os.Chdir(dir)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
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
