package extractor

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// chdir 切换到项目根目录（core/extractor 上两级），测试完毕后恢复
func chdir(t *testing.T) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	// file = .../core/extractor/extractor_test.go → 上两级为项目根
	root := filepath.Join(filepath.Dir(file), "..", "..")
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("chdir: abs path: %v", err)
	}
	orig, _ := os.Getwd()
	if err := os.Chdir(abs); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}

const testHTML = `<!DOCTYPE html>
<html>
<head><title>Test Survey</title></head>
<body>
<form>
  <legend>What is the make and model of your vehicle?</legend>
  <input type="radio" name="q1" value="1"><label>Toyota</label>
  <input type="radio" name="q1" value="2"><label>Honda</label>
  <script>var surveyData = {"questionId": "q1", "type": "single"};</script>
</form>
</body>
</html>`

// TestExtract_ValidInput 验证正常输入下步骤 1-3 全部执行成功
func TestExtract_ValidInput(t *testing.T) {
	chdir(t)

	tmpBase := t.TempDir()
	result, err := Extract("escalentcspacehandp_em9m11pt2xiyuno", testHTML, tmpBase)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	// 步骤 2：PageMetadata 检查
	if result.Meta.SurveyID != "escalentcspacehandp_em9m11pt2xiyuno" {
		t.Errorf("SurveyID = %q, want escalentcspacehandp_em9m11pt2xiyuno", result.Meta.SurveyID)
	}
	if result.Meta.PageID == "" {
		t.Error("PageID is empty")
	}
	if result.Meta.Timestamp == "" {
		t.Error("Timestamp is empty")
	}

	// 步骤 3：AssembledPrompt 检查
	if result.Prompt.System == "" {
		t.Error("System prompt is empty")
	}
	if !strings.Contains(result.Prompt.User, "【cleanHTML 内容】") {
		t.Errorf("User prompt missing header: %s", result.Prompt.User)
	}
	if !strings.Contains(result.Prompt.User, result.Meta.PageID) {
		t.Errorf("User prompt missing pageID path: %s", result.Prompt.User)
	}

	// 日志文件写入检查
	logDir := filepath.Join(tmpBase, "logs")
	entries, err := os.ReadDir(logDir)
	if err != nil {
		t.Fatalf("log dir not created: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("no log file written")
	}
	logContent, err := os.ReadFile(filepath.Join(logDir, entries[0].Name()))
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	log := string(logContent)
	for _, section := range []string{"【步骤 1】接收参数", "【步骤 2】提取 PageMetadata", "【步骤 3】组装 AI Prompt"} {
		if !strings.Contains(log, section) {
			t.Errorf("log missing section: %s", section)
		}
	}
}

// TestExtract_EmptySurveyID 验证空 surveyID 返回错误
func TestExtract_EmptySurveyID(t *testing.T) {
	chdir(t)
	_, err := Extract("", testHTML, t.TempDir())
	if err == nil {
		t.Fatal("expected error for empty surveyID, got nil")
	}
}

// TestExtract_EmptyHTML 验证空 cleanHTML 返回错误
func TestExtract_EmptyHTML(t *testing.T) {
	chdir(t)
	_, err := Extract("test_survey", "", t.TempDir())
	if err == nil {
		t.Fatal("expected error for empty cleanHTML, got nil")
	}
}

// TestExtract_DefaultBasePath 验证 basePath 为空时使用默认值且不崩溃
func TestExtract_DefaultBasePath(t *testing.T) {
	chdir(t)
	// 用空 basePath，会写到 ./storage/logs/，测试后清理
	result, err := Extract("test_default_path", testHTML, "")
	if err != nil {
		t.Fatalf("Extract with empty basePath failed: %v", err)
	}
	if result.Meta.PageID == "" {
		t.Error("PageID is empty with default basePath")
	}
	// 清理默认路径下生成的日志
	t.Cleanup(func() {
		os.RemoveAll(filepath.Join("storage", "logs"))
	})
}

// TestExtractPageMetadata_PageIDExtraction 验证 page_id 清洗逻辑（legend 优先级）
func TestExtractPageMetadata_PageIDExtraction(t *testing.T) {
	meta, err := extractPageMetadata("s1", testHTML)
	if err != nil {
		t.Fatalf("extractPageMetadata: %v", err)
	}
	// legend 文本 "What is the make and model of your vehicle?" 清洗后应为小写+空格
	if !strings.Contains(meta.PageID, "what") {
		t.Errorf("unexpected PageID: %q", meta.PageID)
	}
}

// TestBuildPrompt_SystemPromptContainsCoreRules 验证 system prompt 包含 coreRules 内容
func TestBuildPrompt_SystemPromptContainsCoreRules(t *testing.T) {
	chdir(t)
	prompt, err := buildPrompt("s1", "page1", testHTML)
	if err != nil {
		t.Fatalf("buildPrompt: %v", err)
	}
	if !strings.Contains(prompt.System, "Core Rules") {
		t.Errorf("system prompt missing Core Rules block; got len=%d", len(prompt.System))
	}
	if !strings.Contains(prompt.System, "你是问卷分析专家") {
		t.Errorf("system prompt missing analyze preamble")
	}
}

// TestExtractJSMeta 验证 script 标签内容提取
func TestExtractJSMeta(t *testing.T) {
	meta := extractJSMeta(testHTML)
	if !strings.Contains(meta, "surveyData") {
		t.Errorf("extractJSMeta: expected surveyData in result, got: %q", meta)
	}
}

// TestExtractJSMeta_NoScript 验证无 script 标签时返回空
func TestExtractJSMeta_NoScript(t *testing.T) {
	meta := extractJSMeta("<html><body><p>no script here</p></body></html>")
	if meta != "" {
		t.Errorf("expected empty, got: %q", meta)
	}
}
