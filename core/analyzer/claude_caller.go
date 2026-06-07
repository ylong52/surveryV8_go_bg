package analyzer

// claude_caller.go — 调用 Anthropic Claude API，记录调用日志

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"survey.cli/core/extractor"
)

// CallClaude 调用 Anthropic Claude API，并追加调用记录到事件日志
// @param surveyIDStr  survey 主键字符串（用于日志）
// @param system       系统提示词
// @param user         用户消息
// @returns            AI 回复文本，或错误
func CallClaude(surveyIDStr, system, user string) (string, error) {
	start := time.Now()
	result, err := extractor.CallClaudeAPI(system, user)
	elapsed := time.Since(start)

	status := "成功"
	if err != nil {
		status = fmt.Sprintf("失败: %v", err)
	}
	writeClaudeCallLog(surveyIDStr, elapsed, status)

	return result, err
}

// writeClaudeCallLog 追加一行调用记录到 storage/logs/events/sys_{YYYYMMDD}.log
func writeClaudeCallLog(surveyIDStr string, elapsed time.Duration, status string) {
	logDir := filepath.Join("storage", "logs", "events")
	_ = os.MkdirAll(logDir, 0755)

	date := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, fmt.Sprintf("sys_%s.log", date))

	line := fmt.Sprintf("%s|CLAUDE_CALL|survey_id=%s|elapsed=%.1fs|status=%s\n",
		time.Now().Format("2006-01-02 15:04:05"),
		surveyIDStr,
		elapsed.Seconds(),
		status,
	)

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}
