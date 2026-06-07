package main

// cmd/analyze/main.go — 人设冲突分析 CLI 入口
//
// 用法:
//   go run cmd/analyze/main.go -survey-id=<id>
//
// 示例:
//   go run cmd/analyze/main.go -survey_id=45

import (
	"flag"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
	"survey.cli/core/analyzer"
	"survey.cli/core/database"
)

// setupProxy 从 configs/env.yaml 读取代理配置，写入环境变量
// 使 http.DefaultTransport (callClaudeAPIInternal 依赖) 能正确走代理
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
	// 仅在环境变量未设置时才写入，避免覆盖外部配置
	if f.Proxy.HTTPSProxy != "" && os.Getenv("HTTPS_PROXY") == "" {
		os.Setenv("HTTPS_PROXY", f.Proxy.HTTPSProxy)
	}
	if f.Proxy.HTTPProxy != "" && os.Getenv("HTTP_PROXY") == "" {
		os.Setenv("HTTP_PROXY", f.Proxy.HTTPProxy)
	}
	if f.Proxy.NoProxy != "" && os.Getenv("NO_PROXY") == "" {
		os.Setenv("NO_PROXY", f.Proxy.NoProxy)
	}
}

func main() {
	startTime := time.Now()

	var surveyID int
	flag.IntVar(&surveyID, "survey-id", 0, "survey 表主键 ID")
	flag.IntVar(&surveyID, "survey_id", 0, "survey 表主键 ID")
	flag.Parse()

	if surveyID <= 0 {
		fmt.Fprintln(os.Stderr, "用法: go run cmd/analyze/main.go -survey-id=<id>")
		fmt.Fprintln(os.Stderr, "      go run cmd/analyze/main.go -survey_id=<id>")
		fmt.Fprintln(os.Stderr, "示例: go run cmd/analyze/main.go -survey_id=45")
		os.Exit(1)
	}

	fmt.Printf("╔══════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║              人设冲突分析  survey_id=%-3d              ║\n", surveyID)
	fmt.Printf("╚══════════════════════════════════════════════════════════╝\n")
	fmt.Println()

	// 设置代理（需在任何 HTTP 调用之前）
	setupProxy()

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/survey202605.db"
	}
	fmt.Printf("[1/5] 初始化数据库: %s\n", dbPath)
	if err := database.Init(dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "数据库初始化失败: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	fmt.Printf("[2/5] 调用 analyzer.Analyze(survey_id=%d) ...\n", surveyID)
	reportPath, err := analyzer.Analyze(surveyID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "分析失败: %v\n", err)
		os.Exit(1)
	}

	elapsed := time.Since(startTime)
	fmt.Println()
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
	fmt.Printf("  ✅ 分析完成\n")
	fmt.Printf("  耗时:  %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("  报告:  %s\n", reportPath)
	fmt.Printf("═══════════════════════════════════════════════════════════\n")
}
