package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
)

var (
	sessionID = flag.String("session_id", "session_test", "Session ID")
	urlStr    = flag.String("url", "https://survey.alchemer.com/s3/8809865/Macalester-2026-prospective-families", "Survey URL")
	wsURL     = flag.String("ws", "ws://localhost:10501/ws", "WebSocket server URL")
	htmlFile  = flag.String("html", "", "HTML file path")
)

type WsMessage struct {
	SessionID string `json:"session_id"`
	RawHTML   string `json:"rawhtml"`
	URL       string `json:"url"`
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	DebugMode string `json:"debugmode,omitempty"`
	SurveyID  string `json:"survey_id,omitempty"`
}

func main() {
	flag.Parse()

	fmt.Println("=== WS Client ===")
	fmt.Printf("Session ID: %s\n", *sessionID)
	fmt.Printf("URL:       %s\n", *urlStr)
	fmt.Printf("WS Server: %s\n", *wsURL)
	fmt.Printf("HTML File: %s\n", *htmlFile)
	fmt.Println()

	var rawHTML string
	if *htmlFile != "" {
		content, err := os.ReadFile(*htmlFile)
		if err != nil {
			fmt.Printf("ERROR: 读取文件失败: %v\n", err)
			os.Exit(1)
		}
		rawHTML = string(content)
		fmt.Printf(">> 已加载文件: %s (%.2f KB)\n", *htmlFile, float64(len(rawHTML))/1024)
	} else {
		fmt.Println(">> 拖入 HTML 文件路径（或直接回车输入文件路径）:")
		fmt.Print(">> ")

		reader := bufio.NewReader(os.Stdin)
		filePath, _ := reader.ReadString('\n')
		filePath = strings.TrimSpace(filePath)
		filePath = strings.Trim(filePath, "\"")

		if filePath != "" {
			content, err := os.ReadFile(filePath)
			if err != nil {
				fmt.Printf("ERROR: 读取文件失败: %v\n", err)
				os.Exit(1)
			}
			rawHTML = string(content)
			fmt.Printf(">> 已加载文件: %s (%.2f KB)\n", filePath, float64(len(rawHTML))/1024)
		} else {
			fmt.Println(">> 未提供文件，跳过 rawhtml")
		}
	}

	// fmt.Println()
	// fmt.Println(">> 输入 type（AI 动态推荐，留空回车使用 @ai）:")
	// fmt.Print(">> ")
	// typeStr, _ := reader.ReadString('\n')
	// typeStr = strings.TrimSpace(typeStr)
	// if typeStr == "" {
	// 	typeStr = "@ai"
	// }

	// 生成随机请求ID
	requestID := uuid.New().String()

	// 从 URL 提取 survey_id（用于调试）
	surveyID := extractSurveyID(*urlStr)

	msg := WsMessage{
		SessionID: *sessionID,
		RawHTML:   rawHTML,
		URL:       *urlStr,
		Type:      "analyze_request",
		RequestID: requestID,
		DebugMode: "true",
		SurveyID:  surveyID,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("ERROR: JSON 序列化失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("=== 发送消息 ===")
	fmt.Printf("SESSION_ID:  %s\n", msg.SessionID)
	fmt.Printf("REQUEST_ID:  %s\n", msg.RequestID)
	fmt.Printf("TYPE:        %s\n", msg.Type)
	fmt.Printf("SURVEY_ID:   %s\n", msg.SurveyID)
	if len(msg.RawHTML) > 300 {
		fmt.Printf("RAWHTML:     [%d 字符] %.300s...\n", len(msg.RawHTML), msg.RawHTML)
	} else {
		fmt.Printf("RAWHTML:     %s\n", msg.RawHTML)
	}
	fmt.Printf("URL:         %s\n", msg.URL)
	fmt.Printf("DEBUGMODE:   %s\n", msg.DebugMode)
	fmt.Println("=============")

	parsedURL, err := url.Parse(*wsURL)
	if err != nil {
		fmt.Printf("ERROR: WebSocket URL 解析失败: %v\n", err)
		os.Exit(1)
	}

	conn, _, err := websocket.DefaultDialer.Dial(parsedURL.String(), nil)
	if err != nil {
		fmt.Printf("ERROR: WebSocket 连接失败: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		fmt.Printf("ERROR: 发送消息失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println(">> 消息已发送，等待服务端响应...")

	// 循环读取所有响应消息
	for {
		_, resp, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure) {
				fmt.Printf("WARNING: 读取响应失败: %v\n", err)
			}
			break
		}

		respStr := string(resp)
		fmt.Printf("\n=== 服务端响应 (%.2f KB) ===\n", float64(len(respStr))/1024)
		// 尝试美化打印为 JSON 格式
		var prettyJSON map[string]interface{}
		if err := json.Unmarshal([]byte(respStr), &prettyJSON); err == nil {
			prettyBytes, _ := json.MarshalIndent(prettyJSON, "", "  ")
			fmt.Println(string(prettyBytes))
		} else {
			fmt.Println(respStr)
		}
		// fmt.Println("===========================")

		// // 检查是否是最终响应（包含 final_json），如果是则退出循环
		// var respData map[string]interface{}
		// if json.Unmarshal(resp, &respData) == nil {
		// 	if data, ok := respData["data"].(map[string]interface{}); ok {
		// 		if _, hasFinalJson := data["final_json"]; hasFinalJson {
		// 			fmt.Println("\n>> 收到 final_json，响应完成")
		// 			break
		// 		}
		// 	}
		// }
	}

	fmt.Println(">> 客户端退出")
}

// SiteConfig 站点配置（与 page-fingerprints.yaml 结构对应）
type SiteConfig struct {
	Keyword  string `yaml:"keyword"`
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	SurveyID string `yaml:"survey_id"`
}

// FingerprintsConfig 配置文件结构
type FingerprintsConfig struct {
	Sites []SiteConfig `yaml:"sites"`
}

// loadSitesFromConfig 加载配置文件，返回站点列表
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

// findConfigPath 查找配置文件路径
func findConfigPath() string {
	paths := []string{
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

// extractSurveyID 从 URL 匹配 page-fingerprints.yaml 配置获取 survey_id
func extractSurveyID(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	configPath := findConfigPath()
	sites, err := loadSitesFromConfig(configPath)
	if err != nil {
		return ""
	}

	// URL 前缀或精确匹配
	for _, site := range sites {
		if strings.HasPrefix(rawURL, site.URL) || rawURL == site.URL {
			return site.SurveyID
		}
	}
	return ""
}
