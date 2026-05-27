package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"survey.cli/core/cleaner"
)

// SurveyTestCase 测试用例结构
type SurveyTestCase struct {
	SurveyID  string
	Name      string
	DocDir    string
	HTMLFiles []string
	Pattern   string
}

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║          Qualtrics HTML page_id 提取测试                      ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 定义测试用例
	testCases := []SurveyTestCase{
		{
			SurveyID:  "sv_3bhtjkfaeoedj5c",
			Name:      "SV_3BHtJkfaEOEDj5c",
			DocDir:    "survey.qualtrics.comjfeformSV_3BHtJkfaEOEDj5c",
			HTMLFiles: []string{},
			Pattern:   "<legend> <label class=\"QuestionText\">题目内容</label></legend>",
		},
		{
			SurveyID:  "sv_em9m11pt2xiyuno",
			Name:      "SV_eM9M11Pt2XiyUNo",
			DocDir:    "sescalentcspacehandp.qualtrics.comjfeformSV_eM9M11Pt2XiyUNo",
			HTMLFiles: []string{},
			Pattern:   "<legend><div>题目内容</div></legend> 或 <legend> <label>题目内容</label></legend>",
		},
	}

	// 获取工作目录
	wd, err := os.Getwd()
	if err != nil {
		fmt.Printf("❌ 获取工作目录失败: %v\n", err)
		os.Exit(1)
	}
	docsRoot := filepath.Join(wd, "..", "docs")

	// 统计
	totalFiles := 0
	successFiles := 0
	skippedFiles := 0
	failedFiles := 0

	for _, tc := range testCases {
		fmt.Printf("═══════════════════════════════════════════════════════════════\n")
		fmt.Printf("📋 测试集: %s\n", tc.Name)
		fmt.Printf("   SurveyID: %s\n", tc.SurveyID)
		fmt.Printf("   匹配模式: %s\n", tc.Pattern)
		fmt.Println()

		// 动态获取目录下的所有 .txt 文件
		docPath := filepath.Join(docsRoot, tc.DocDir)

		entries, err := os.ReadDir(docPath)
		if err != nil {
			fmt.Printf("❌ 读取目录失败: %v\n", err)
			continue
		}

		tc.HTMLFiles = []string{}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
				tc.HTMLFiles = append(tc.HTMLFiles, entry.Name())
			}
		}

		// 加载该问卷的正则配置
		patterns, err := cleaner.LoadPatternsForSurvey(tc.SurveyID)
		if err != nil {
			fmt.Printf("❌ 加载配置失败: %v\n", err)
			continue
		}

		fmt.Printf("📂 正则配置 (共 %d 条):\n", len(patterns))
		for i, p := range patterns {
			fmt.Printf("   %d. %s\n", i+1, p)
		}
		fmt.Println()

		for _, htmlFile := range tc.HTMLFiles {
			totalFiles++
			filePath := filepath.Join(docPath, htmlFile)

			fmt.Printf("📄 测试文件: %s\n", htmlFile)

			// 读取 HTML 文件
			htmlData, err := os.ReadFile(filePath)
			if err != nil {
				fmt.Printf("   ❌ 读取文件失败: %v\n\n", err)
				failedFiles++
				continue
			}
			html := string(htmlData)

			// 检测是否为无效页面（JavaScript is required）
			if !containsValidQuestion(html) {
				fmt.Printf("   ⏭️  跳过: 不是有效的问卷页面（缺少QuestionText元素）\n\n")
				skippedFiles++
				continue
			}

			// 提取 page_id
			pageID, err := cleaner.ExtractPageIDWithPatterns(html, patterns)
			if err != nil {
				fmt.Printf("   ❌ 提取 page_id 失败: %v\n", err)
				showActualStructure(html)
				fmt.Println()
				failedFiles++
				continue
			}

			fmt.Printf("   ✅ PageID: %s\n", pageID)
			fmt.Println()
			successFiles++
		}
	}

	// 输出统计
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("📊 测试统计")
	fmt.Printf("   总文件数: %d\n", totalFiles)
	fmt.Printf("   成功:     %d ✅\n", successFiles)
	fmt.Printf("   跳过:     %d ⏭️\n", skippedFiles)
	fmt.Printf("   失败:     %d ❌\n", failedFiles)
	fmt.Println("═══════════════════════════════════════════════════════════════")

	if failedFiles > 0 {
		os.Exit(1)
	}
}

// containsValidQuestion 检测 HTML 是否包含有效的问卷问题元素
// 通过检查是否包含 class="QuestionText" 的元素
func containsValidQuestion(html string) bool {
	re := regexp.MustCompile(`(?i)class="[^"]*QuestionText[^"]*"`)
	return re.MatchString(html)
}

// showActualStructure 显示 HTML 中的实际结构，帮助调试
func showActualStructure(html string) {
	re := regexp.MustCompile(`(?i)<legend[^>]*>[\s\S]{0,300}?</legend>`)
	matches := re.FindString(html)

	if matches != "" {
		display := matches
		if len(display) > 200 {
			display = display[:200] + "..."
		}
		display = strings.ReplaceAll(display, "\n", " ")
		display = strings.ReplaceAll(display, "\r", "")
		fmt.Printf("   💡 实际结构: %s\n", display)
	}

	if strings.Contains(html, "QuestionText") {
		fmt.Printf("   ℹ️  HTML 中包含 QuestionText\n")
	}
	if strings.Contains(html, "class=\"") {
		fmt.Printf("   ℹ️  HTML 中包含 class 属性\n")
	}
}
