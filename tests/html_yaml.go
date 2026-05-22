package main

// CLI 工具：读取 HTML 文件 → 清洗 → extractor.Extract（提取元数据+组装Prompt+写日志）→ 保存到 cache
//   步骤 7: 当 Source="cache" 时，根据缓存模板 + 实时 HTML 生成最新 YAML 数据
//
// 用法：
//   go run tests/html_yaml.go --html <HTML路径> --survey_id <ID> [--base_path <路径>]

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"survey.cli/core/cleaner"
	"survey.cli/core/extractor"
)

func main() {
	htmlPath := flag.String("html", "", "HTML 文件路径")
	surveyID := flag.String("survey_id", "", "问卷 ID")
	basePath := flag.String("base_path", "./storage", "存储根目录（默认 ./storage）")
	flag.Parse()

	// 如果没有 --html 参数，交互式等待用户拖入文件
	if *htmlPath == "" {
		fmt.Println("请拖入 .html 或 .txt 文件（按回车确认）：")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		*htmlPath = strings.TrimSpace(input)
		// 去除首尾引号（用户拖入时可能带引号）
		*htmlPath = strings.TrimPrefix(strings.TrimSuffix(*htmlPath, `"`), `"`)
		*htmlPath = strings.TrimPrefix(strings.TrimSuffix(*htmlPath, "'"), "'")
	}

	// 检查文件后缀
	ext := strings.ToLower(filepath.Ext(*htmlPath))
	if ext != ".html" && ext != ".txt" {
		log.Fatalf("不支持的文件类型: %s，仅支持 .html 和 .txt 文件", ext)
	}

	// 如果没有 survey_id，交互式询问
	if *surveyID == "" {
		fmt.Print("请输入 survey_id：")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		*surveyID = strings.TrimSpace(input)
	}

	if *htmlPath == "" || *surveyID == "" {
		log.Fatal("html 文件路径和 survey_id 不能为空")
	}

	// ① 读取 HTML 文件，自动修复 CP1252 乱码
	rawBytes, err := os.ReadFile(*htmlPath)
	if err != nil {
		log.Fatalf("读取文件失败: %v", err)
	}
	rawHTML := fixEncoding(rawBytes)

	// ② HTML 清洗（StripForDirectSurvey 保留 legend/label 等语义标签，不影响 page_id 提取）
	cleanHTML := cleaner.StripForDirectSurvey(rawHTML)

	// ③ extractor.ExtractYamlStructure：提取YAML结构
	result, err := extractor.ExtractYamlStructure(*surveyID, cleanHTML, *basePath)
	if err != nil {
		log.Fatalf("extractor.ExtractYamlStructure 失败: %v", err)
	}

	fmt.Printf("=== SurveyID: %s ===\n", result.Meta.SurveyID)
	fmt.Printf("=== PageID: %s ===\n", result.Meta.PageID)
	fmt.Printf("=== Timestamp: %s ===\n", result.Meta.Timestamp)
	fmt.Printf("=== YAML长度: %d字符 ===\n", len(result.YamlContent))
	fmt.Printf("=== 缓存路径: %s ===\n", result.CachePath)
	fmt.Printf("=== 来源: %s ===\n", result.Source)

	// ④ 步骤 7: 当 Source="cache" 时，生成实时 YAML 数据
	if result.Source == "cache" {
		fmt.Println()
		fmt.Println("=== 触发步骤 7: 生成实时 YAML 数据 ===")
		realtimeResult, err := extractor.GenerateRealtimeYamlData(cleanHTML, result.YamlContent, *surveyID, *basePath)
		if err != nil {
			log.Fatalf("步骤 7 GenerateRealtimeYamlData 失败: %v", err)
		}
		fmt.Printf("=== 实时 YAML 长度: %d字符 ===\n", len(realtimeResult.YamlData))
		fmt.Printf("=== 实时 YAML 保存路径: %s ===\n", realtimeResult.CachePath)
		fmt.Printf("=== 实时 YAML 来源: %s ===\n", realtimeResult.Source)
	} else {
		fmt.Println()
		fmt.Println("=== 跳过步骤 7（Source != cache）===")
	}
}

// fixEncoding 检测 CP1252 特征字节并替换为对应 UTF-8 字符
// Qualtrics/Alchemer HTML 有时以 Windows-1252 编码保存，含 0x92–0x97 等字节
func fixEncoding(b []byte) string {
	cp1252map := map[byte]string{
		0x82: ",", // SINGLE LOW-9 QUOTATION MARK
		0x91: "‘", // LEFT SINGLE QUOTATION MARK
		0x92: "’", // RIGHT SINGLE QUOTATION MARK  (最常见：I've → I've)
		0x93: "“", // LEFT DOUBLE QUOTATION MARK
		0x94: "”", // RIGHT DOUBLE QUOTATION MARK
		0x96: "–", // EN DASH
		0x97: "—", // EM DASH
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

	out := make([]byte, 0, len(b))
	for _, byt := range b {
		if r, ok := cp1252map[byt]; ok {
			out = append(out, []byte(r)...)
		} else {
			out = append(out, byt)
		}
	}
	return string(out)
}
