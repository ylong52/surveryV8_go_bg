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
	"time"

	"survey.cli/core/cleaner"
	"survey.cli/core/extractor"
)

func main() {
	// AI 调用计数器
	aiCallCount := 0

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

	// 打印开始信息
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("            开始处理问卷页面")
	fmt.Println("========================================")
	fmt.Printf("HTML 文件: %s\n", *htmlPath)
	fmt.Printf("Survey ID: %s\n", *surveyID)
	fmt.Printf("存储路径: %s\n", *basePath)
	fmt.Println()

	// ① 读取 HTML 文件，自动修复 CP1252 乱码
	fmt.Println(">>> 步骤 1: 读取并修复 HTML 文件编码...")
	rawBytes, err := os.ReadFile(*htmlPath)
	if err != nil {
		log.Fatalf("读取文件失败: %v", err)
	}
	rawHTML := fixEncoding(rawBytes)
	fmt.Printf("    HTML 长度: %d 字符\n", len(rawHTML))
	fmt.Println()

	// ② HTML 清洗（StripForDirectSurvey 保留 legend/label 等语义标签，不影响 page_id 提取）
	fmt.Println(">>> 步骤 2: 清洗 HTML...")
	cleanHTML := cleaner.StripForDirectSurvey(rawHTML)
	fmt.Printf("    清洗后长度: %d 字符\n", len(cleanHTML))
	fmt.Println()

	// ③ extractor.ExtractYamlStructure：提取YAML结构
	fmt.Println(">>> 步骤 3: 提取 YAML 结构...")
	startStep := time.Now()
	result, err := extractor.ExtractYamlStructure(*surveyID, cleanHTML, *basePath)
	elapsed := time.Since(startStep)
	if err != nil {
		log.Fatalf("extractor.ExtractYamlStructure 失败: %v", err)
	}

	// 根据来源判断是否调用了 AI
	if result.Source == "cache" {
		fmt.Println("    [缓存命中] 使用本地模板，跳过 AI 调用")
	} else {
		aiCallCount++
		fmt.Printf("    [AI 调用] 耗时 %.2f 秒\n", elapsed.Seconds())
	}

	fmt.Println()
	fmt.Println("========== YAML 结构提取结果 ==========")
	fmt.Printf("SurveyID: %s\n", result.Meta.SurveyID)
	fmt.Printf("PageID: %s\n", result.Meta.PageID)
	fmt.Printf("Timestamp: %s\n", result.Meta.Timestamp)
	fmt.Printf("YAML长度: %d 字符\n", len(result.YamlContent))
	fmt.Printf("缓存路径: %s\n", result.CachePath)
	fmt.Printf("来源: %s\n", result.Source)
	fmt.Println()

	// ④ 步骤 7: 当 Source="cache" 时，生成实时 YAML 数据
	if result.Source == "cache" {
		fmt.Println(">>> 步骤 7: 生成实时 YAML 数据...")
		fmt.Println("    [AI 调用 #1] 生成实时 YAML 结构")
		startAI := time.Now()
		realtimeResult, err := extractor.GenerateRealtimeYamlData(cleanHTML, result.YamlContent, *surveyID, *basePath)
		elapsedAI := time.Since(startAI)
		aiCallCount++
		if err != nil {
			log.Fatalf("步骤 7 GenerateRealtimeYamlData 失败: %v", err)
		}
		fmt.Printf("    [AI #1 耗时] %.2f 秒\n", elapsedAI.Seconds())
		fmt.Println()

		fmt.Println("========== 实时 YAML 生成结果 ==========")
		fmt.Printf("YAML长度: %d 字符\n", len(realtimeResult.YamlData))
		fmt.Printf("保存路径: %s\n", realtimeResult.CachePath)
		fmt.Printf("来源: %s\n", realtimeResult.Source)
		fmt.Println()

		// ⑤ 步骤 8: 生成推荐答案
		fmt.Println(">>> 步骤 8: 生成推荐答案...")
		fmt.Println("    [AI 调用 #2] 生成推荐答案 + page_summary")
		startAI = time.Now()
		answerResult, err := extractor.GenerateRealtimeYamlWithAnswers(cleanHTML, result.YamlContent, *surveyID, *basePath, realtimeResult.PageSettings)
		elapsedAI = time.Since(startAI)
		aiCallCount++
		if err != nil {
			log.Fatalf("步骤 8 GenerateRealtimeYamlWithAnswers 失败: %v", err)
		}
		fmt.Printf("    [AI #2 耗时] %.2f 秒\n", elapsedAI.Seconds())
		fmt.Println()

		fmt.Println("========== 推荐答案生成结果 ==========")
		fmt.Printf("推荐答案数量: %d\n", len(answerResult.Answers))
		fmt.Printf("来源: %s\n", answerResult.Source)
		fmt.Printf("PageID: %s\n", answerResult.PageID)
		fmt.Println()

		// 打印 FinalJson
		fmt.Println("========== Final JSON 输出 ==========")
		fmt.Printf("%+v\n", answerResult.FinalJson)
		fmt.Println()

		fmt.Println("========== 处理完成 ==========")
		fmt.Printf("总 AI 调用次数: %d\n", aiCallCount)
		fmt.Printf("总耗时: %.2f 秒\n", time.Since(startStep).Seconds())
		fmt.Println()
	} else {
		fmt.Println()
		fmt.Println(">>> 跳过步骤 7-8（来源不是 cache）...")
		fmt.Println()
		fmt.Println("========== 处理完成 ==========")
		fmt.Printf("总 AI 调用次数: %d\n", aiCallCount)
		fmt.Printf("总耗时: %.2f 秒\n", time.Since(startStep).Seconds())
		fmt.Println()
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
