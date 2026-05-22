package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"

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
	SessionID string `json:"session_id"`
	RawHTML   string `json:"rawhtml"`
	URL       string `json:"url"`
	Type      string `json:"type"`
	DebugMode string `json:"debugmode"`
}

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
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
			sendError(conn, "JSON parse error: "+err.Error())
			continue
		}

		// 解析 survey_id 从 URL
		surveyID := extractSurveyID(msg.URL)
		if surveyID == "" {
			surveyID = "unknown"
		}

		startTime := time.Now()

		// ========== 调试：打印原始输入 ==========
		fmt.Println()
		fmt.Println("========================================")
		fmt.Println("            接收到的原始数据")
		fmt.Println("========================================")
		fmt.Printf("SessionID: %s\n", msg.SessionID)
		fmt.Printf("URL:       %s\n", msg.URL)
		fmt.Printf("SurveyID:  %s\n", surveyID)
		fmt.Printf("RawHTML长度: %d 字符\n", len(msg.RawHTML))
		fmt.Println()

		// 原始 HTML 预览（截断显示）
		// previewLen := 500
		// if len(msg.RawHTML) > previewLen {
		// 	fmt.Printf("RawHTML预览（前%d字符）:\n%s...\n\n", previewLen, msg.RawHTML[:previewLen])
		// } else {
		// 	fmt.Printf("RawHTML内容:\n%s\n\n", msg.RawHTML)
		// }

		// 步骤1: 修复编码
		// fmt.Println("========================================")
		// fmt.Println(">>> 步骤 1: 修复 HTML 编码")
		// fmt.Println("========================================")
		fixedHTML := fixEncoding([]byte(msg.RawHTML))
		// fmt.Printf("修复后长度: %d 字符\n", len(fixedHTML))
		// if len(fixedHTML) > previewLen {
		// 	fmt.Printf("修复后预览（前%d字符）:\n%s...\n\n", previewLen, fixedHTML[:previewLen])
		// } else {
		// 	fmt.Printf("修复后内容:\n%s\n\n", fixedHTML)
		// }

		// 步骤2: 清洗 HTML
		// fmt.Println("========================================")
		// fmt.Println(">>> 步骤 2: 清洗 HTML")
		// fmt.Println("========================================")
		cleanHTML := cleaner.StripForDirectSurvey(fixedHTML)
		// fmt.Printf("清洗后长度: %d 字符\n", len(cleanHTML))
		// if len(cleanHTML) > previewLen {
		// 	fmt.Printf("清洗后预览（前%d字符）:\n%s...\n\n", previewLen, cleanHTML[:previewLen])
		// } else {
		// 	fmt.Printf("清洗后内容:\n%s\n\n", cleanHTML)
		// }

		// 步骤3: 提取 YAML 结构
		fmt.Println("========================================")
		fmt.Println(">>> 步骤 3: 提取 YAML 结构")
		fmt.Println("========================================")
		yamlResult, err := extractor.ExtractYamlStructure(surveyID, cleanHTML, basePath)
		if err != nil {
			sendError(conn, "ExtractYamlStructure error: "+err.Error())
			continue
		}
		fmt.Printf("PageID:    %s\n", yamlResult.Meta.PageID)
		fmt.Printf("Source:    %s\n", yamlResult.Source)
		fmt.Printf("CachePath: %s\n", yamlResult.CachePath)
		fmt.Printf("YAML长度:  %d 字符\n", len(yamlResult.YamlContent))
		if len(yamlResult.YamlContent) > 300 {
			fmt.Printf("YAML预览（前300字符）:\n%s...\n\n", yamlResult.YamlContent[:300])
		} else {
			fmt.Printf("YAML内容:\n%s\n\n", yamlResult.YamlContent)
		}

		var finalJson extractor.FinalAnswerJson

		fmt.Println("========================================")
		fmt.Println(">>> 步骤 4: 生成推荐答案")
		fmt.Println("========================================")

		if yamlResult.Source == "cache" {
			// 缓存命中：单次 AI 调用
			fmt.Println("Source: cache（使用缓存模板）")
			fmt.Println("调用 AI: GenerateRealtimeAnswerOnly...")
			answerResult, err := extractor.GenerateRealtimeAnswerOnly(cleanHTML, yamlResult.YamlContent, surveyID, msg.SessionID, basePath)
			if err != nil {
				sendError(conn, "GenerateRealtimeAnswerOnly error: "+err.Error())
				continue
			}
			finalJson = answerResult.FinalJson
			fmt.Printf("AI 响应来源: %s\n", answerResult.Source)
			fmt.Printf("推荐答案数量: %d\n", len(answerResult.Interactions))
		} else {
			// 未命中：AI 生成实时 YAML + 答案
			fmt.Println("Source: ai（实时生成）")
			fmt.Println("调用 AI: GenerateRealtimeYamlWithAnswers...")
			answerResult, err := extractor.GenerateRealtimeYamlWithAnswers(cleanHTML, yamlResult.YamlContent, surveyID, msg.SessionID, basePath, extractor.PageSettings{})
			if err != nil {
				sendError(conn, "GenerateRealtimeYamlWithAnswers error: "+err.Error())
				continue
			}
			finalJson = answerResult.FinalJson
			fmt.Printf("AI 响应来源: %s\n", answerResult.Source)
			fmt.Printf("推荐答案数量: %d\n", len(answerResult.Answers))
		}

		jsonBytes, _ := json.MarshalIndent(finalJson, "", "  ")
		fmt.Println("FinalJson:")
		fmt.Println(string(jsonBytes))

		elapsed := time.Since(startTime)

		// 构造响应
		response := Response{
			Code:    200,
			Message: "success",
			Data: map[string]interface{}{
				"session_id":  msg.SessionID,
				"page_id":     yamlResult.Meta.PageID,
				"yaml_source": yamlResult.Source,
				"yaml_cache":  yamlResult.CachePath,
				"elapsed_sec": elapsed.Seconds(),
				"final_json":  finalJson,
			},
		}

		respBytes, _ := json.Marshal(response)
		if err := conn.WriteMessage(websocket.TextMessage, respBytes); err != nil {
			log.Printf("write error: %v", err)
			break
		}

		fmt.Printf(">>> 完成: session=%s, page_id=%s, source=%s, elapsed=%.2fs\n",
			msg.SessionID, yamlResult.Meta.PageID, yamlResult.Source, elapsed.Seconds())

		// 传给前端
		if err := conn.WriteMessage(websocket.TextMessage, jsonBytes); err != nil {
			log.Printf("write error: %v", err)
			break
		}
		fmt.Println("FinalJson 传给前端完成")
	}
}

func sendError(conn *websocket.Conn, errMsg string) {
	resp := Response{Code: 500, Message: errMsg}
	bytes, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, bytes)
}

func extractSurveyID(url string) string {
	// 从 URL 提取 survey_id，例如:
	// https://survey.alchemer.com/s3/8809865/Macalester-2026-prospective-families
	// 返回 "8809865"
	parts := strings.Split(url, "/")
	for i, p := range parts {
		if p == "s3" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func main() {
	port := os.Getenv("WS_PORT")
	if port == "" {
		port = "10501"
	}

	http.HandleFunc("/ws", wsHandler)

	fmt.Printf("WS server running on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
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
