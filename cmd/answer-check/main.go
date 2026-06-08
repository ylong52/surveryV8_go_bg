package main

// main.go — answer-check 交互式答题入口
//
// 流程：
//   1. 解析命令行参数
//   2. 加载 HTML 文件（支持拖入）
//   3. 清洗 HTML + 提取 PageID
//   4. 调用 AI 生成推荐答案
//   5. 从 HTML 解析选项列表（radio/checkbox/input）
//   6. 启动交互式答题
//   7. 持久化到 user_submitted_answers 表
//   8. 输出结果摘要
//
// 用法：
//   go run cmd/answer-check/main.go [-survey_id=xxx] [-session=xxx] [-html=path]
//   go run cmd/answer-check/main.go                      # 全部用默认值，html 提示拖入
//   go run cmd/answer-check/main.go -html="page.html"    # 指定 HTML 文件

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"survey.cli/core/cleaner"
	"survey.cli/core/database"
	"survey.cli/core/extractor"
	"survey.cli/core/simulator"
)

func setupProxy() {
	data, err := os.ReadFile("configs/env.yaml")
	if err != nil {
		return
	}
	var f struct {
		Proxy struct {
			HTTPSProxy string `yaml:"https_proxy"`
			HTTPProxy  string `yaml:"http_proxy"`
			NoProxy    string `yaml:"no_proxy"`
		} `yaml:"proxy"`
	}
	if err := yaml.Unmarshal(data, &f); err != nil {
		return
	}
	if f.Proxy.HTTPSProxy != "" && os.Getenv("HTTPS_PROXY") == "" {
		os.Setenv("HTTPS_PROXY", f.Proxy.HTTPSProxy)
	}
	if f.Proxy.HTTPProxy != "" && os.Getenv("HTTP_PROXY") == "" {
		os.Setenv("HTTP_PROXY", f.Proxy.HTTPProxy)
	}
}

func main() {
	// ── 参数解析 ──
	surveyID := flag.String("survey_id", "test_0002", "问卷 ID")
	sessionID := flag.String("session", "session_id_test0002", "会话 ID")
	htmlPath := flag.String("html", "", "HTML 文件路径（.html/.txt），不传则提示拖入")
	flag.Parse()

	// ── 设置代理 ──
	setupProxy()

	// ── 初始化数据库 ──
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/survey202605.db"
	}
	if err := database.Init(dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "数据库初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// 初始化站点缓存（用于 page_id 正则匹配）
	if err := database.InitSiteCache(); err != nil {
		fmt.Fprintf(os.Stderr, "站点缓存初始化失败: %v\n", err)
		os.Exit(1)
	}

	// ── 加载 HTML 文件 ──
	if *htmlPath == "" {
		fmt.Println("请拖入 HTML 或 TXT 文件，然后按回车:")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		*htmlPath = cleanFilePath(strings.TrimSpace(input))
	} else {
		*htmlPath = cleanFilePath(*htmlPath)
	}

	// 验证文件存在 + 扩展名
	ext := strings.ToLower(filepath.Ext(*htmlPath))
	if ext != ".html" && ext != ".txt" {
		fmt.Fprintf(os.Stderr, "不支持的文件类型: %s（仅支持 .html 和 .txt）\n", ext)
		os.Exit(1)
	}

	rawHTML, err := os.ReadFile(*htmlPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取文件失败: %v\n", err)
		os.Exit(1)
	}

	// ── 清洗 HTML + 提取 PageID ──
	cleanHTML := cleaner.StripForDirectSurvey(string(rawHTML))
	meta, err := extractor.ExtractPageMetadataFromRaw(*surveyID, string(rawHTML))
	if err != nil {
		fmt.Printf("警告: PageID 提取失败（将使用默认值）: %v\n", err)
		meta = extractor.PageMetadata{
			SurveyID:  *surveyID,
			PageID:    "page_1",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
	}

	// ── 调用 AI 生成推荐答案 ──
	fmt.Printf("\n[1/4] 生成推荐答案: survey_id=%s, page_id=%s\n", *surveyID, meta.PageID)

	basePath := "./storage"

	// 提取 YAML 结构（题型检测 + 模板）
	yamlResult, err := extractor.ExtractYamlStructureWithPageID(
		*surveyID,
		0,
		meta.PageID,
		cleanHTML,
		basePath,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "YAML 模板提取失败: %v\n", err)
		os.Exit(1)
	}

	// 生成推荐答案
	aiResult, err := extractor.GenerateRealtimeAnswerOnlyWithMeta(
		meta,
		cleanHTML,
		yamlResult.YamlContent,
		*surveyID,
		0,
		basePath,
		yamlResult.DetectedTypes,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "AI 推荐答案生成失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("    推荐答案生成完成，共 %d 题\n\n", len(aiResult.Interactions))

	// ── 从 HTML 解析选项列表 ──
	fmt.Printf("[2/4] 解析选项列表...\n")
	htmlParser := simulator.NewHTMLOptionParser(string(rawHTML))
	questions := htmlParser.ParseQuestions(aiResult.Interactions)

	// ── 启动交互式答题 ──
	fmt.Printf("[3/4] 启动交互式答题...\n\n")
	sim := simulator.NewInteractiveSimulator(
		*surveyID,
		*sessionID,
		questions,
		meta.PageID,
	)
	sim.SetDebugMode(true) // 启用调试输出

	result, err := sim.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "答题失败: %v\n", err)
		os.Exit(1)
	}

	if !result.AllFilled {
		fmt.Println("\n退出答题")
		os.Exit(0)
	}

	// ── 持久化到数据库 ──
	fmt.Printf("\n[4/4] 保存答案到数据库...\n")
	repo := database.NewUserSubmittedAnswerRepo()

	for _, ans := range result.Answers {
		q := questions[ans.QuestionIndex]
		qID := "q" + strconv.Itoa(ans.QuestionIndex+1)
		optPrefix := simulator.GetOptIDPrefix(qID)

		answerJSON := buildAnswerJSON(q, ans, optPrefix)

		submitted := &database.UserSubmittedAnswer{
			SessionID:       *sessionID,
			PageResultsID:   0,
			Question:        q.Subject,
			Answer:          answerJSON,
			InteractionType: q.QuestionType,
			Timestamp:       time.Now().UnixMilli(),
		}
		if err := repo.Upsert(submitted); err != nil {
			fmt.Printf("    警告: 保存答案失败 [%s]: %v\n", q.Subject, err)
		}
	}

	// ── 输出结果摘要 ──
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                      答题完成                                    ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  问卷 ID   : %s\n", *surveyID)
	fmt.Printf("║  会话 ID   : %s\n", *sessionID)
	fmt.Printf("║  页面 ID   : %s\n", meta.PageID)
	fmt.Printf("║  答题数量   : %d/%d 题\n", len(result.Answers), len(questions))
	fmt.Printf("║  提交时间   : %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("╚══════════════════════════════════════════════════════════════════════╝")
}

// cleanFilePath 清理拖入路径中的多余字符
//
// 处理场景：
//   - PowerShell 拖入：`"E:\path\file.txt"` → 带引号
//   - 前置乱码字符：`e"E:\path\file.txt"` → 引号前有杂字符
//   - 末尾引号残留：`E:\path\file.txt"` → 末尾多余引号
func cleanFilePath(raw string) string {
	s := strings.TrimSpace(raw)

	// 优先：用正则从输入中提取 Windows 绝对路径（盘符:\...）
	// 处理末尾可能残留的 " 或空白
	re := regexp.MustCompile(`[A-Za-z]:[^\r\n"']+`)
	if m := re.FindString(s); m != "" {
		return strings.TrimRight(m, `"' `)
	}

	// 兜底：去除首尾引号
	s = strings.Trim(s, `"'`)
	return strings.TrimSpace(s)
}

// buildAnswerJSON 构建答案 JSON（带 opt_id 和 xpath）
func buildAnswerJSON(q simulator.SimQuestion, ans simulator.SimAnswer, optPrefix int) string {
	qID := "q" + strconv.Itoa(ans.QuestionIndex+1)

	switch q.QuestionType {
	case "single", "dropdown", "matrix_radio":
		if ans.SelectedOptID > 0 {
			optIndex := ans.SelectedOptID - optPrefix - 1
			if optIndex >= 0 && optIndex < len(q.Options) {
				opt := q.Options[optIndex]
				data := map[string]interface{}{
					"question_id":   qID,
					"question_type": q.QuestionType,
					"answer": map[string]interface{}{
						"label":  opt.Label,
						"opt_id": ans.SelectedOptID,
						"xpath": opt.XPath,
					},
				}
				b, _ := json.Marshal(data)
				return string(b)
			}
		}

	case "multi", "matrix_checkbox":
		if len(ans.SelectedOptIDs) > 0 {
			var answers []map[string]interface{}
			for _, optID := range ans.SelectedOptIDs {
				optIndex := optID - optPrefix - 1
				if optIndex >= 0 && optIndex < len(q.Options) {
					opt := q.Options[optIndex]
					answers = append(answers, map[string]interface{}{
						"label":  opt.Label,
						"opt_id": optID,
						"xpath": opt.XPath,
					})
				}
			}
			if len(answers) > 0 {
				data := map[string]interface{}{
					"question_id":   qID,
					"question_type": q.QuestionType,
					"answer":        answers,
				}
				b, _ := json.Marshal(data)
				return string(b)
			}
		}

	default:
		// 填空题
		data := map[string]interface{}{
			"question_id":   qID,
			"question_type": q.QuestionType,
			"answer":        ans.TextValue,
		}
		b, _ := json.Marshal(data)
		return string(b)
	}

	// 兜底返回原始格式
	return simulator.FormatAnswer(q, ans)
}
