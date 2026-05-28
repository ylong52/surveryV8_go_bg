package extractor

// ai_caller.go — 步骤 4：调用 DeepSeek API 提取 YAML 结果
//
// 读取 configs/env.yaml 获取 API 配置，支持 3 次重试，
// 自动处理限流、超时、服务器错误，并写入结构化日志。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// deepSeekEnvConfig 对应 configs/env.yaml → deepseek 节点
type deepSeekEnvConfig struct {
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	Timeout     int     `yaml:"timeout"`
}

type envYAMLFile struct {
	DeepSeek deepSeekEnvConfig `yaml:"deepseek"`
}

// deepSeekRequest 对应 DeepSeek /chat/completions 请求体
type deepSeekRequest struct {
	Model       string             `json:"model"`
	Messages    []deepSeekMessage  `json:"messages"`
	Temperature float64            `json:"temperature"`
	Stream      bool               `json:"stream"`
}

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// deepSeekResponse 只解析需要的字段
type deepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// aiFailureLog 对应失败日志 YAML 结构
type aiFailureLog struct {
	RequestTime  string `yaml:"request_time"`
	SystemPrompt string `yaml:"system_prompt"`
	UserPrompt   string `yaml:"user_prompt"`
	RawResponse  string `yaml:"raw_response,omitempty"`
	ErrorType    string `yaml:"error_type"`
	RetryCount   int    `yaml:"retry_count"`
}

const aiEnvYAMLPath = "configs/env.yaml"

// loadDeepSeekEnvConfig 从 configs/env.yaml 读取 DeepSeek 节点配置
func loadDeepSeekEnvConfig() (*deepSeekEnvConfig, error) {
	data, err := os.ReadFile(aiEnvYAMLPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", aiEnvYAMLPath, err)
	}
	var f envYAMLFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse %s: %w", aiEnvYAMLPath, err)
	}
	if f.DeepSeek.APIKey == "" {
		return nil, fmt.Errorf("deepseek.api_key is empty in %s", aiEnvYAMLPath)
	}
	if f.DeepSeek.BaseURL == "" {
		f.DeepSeek.BaseURL = "https://api.deepseek.com"
	}
	if f.DeepSeek.Model == "" {
		f.DeepSeek.Model = "deepseek-chat"
	}
	if f.DeepSeek.Timeout <= 0 {
		f.DeepSeek.Timeout = 60
	}
	return &f.DeepSeek, nil
}

// CallAI 调用 DeepSeek API，返回 AI 响应（不验证格式）
// @param surveyID     问卷 ID（用于日志文件命名）
// @param systemPrompt 系统提示词
// @param userPrompt   用户提示词（含完整 HTML）
// @param basePath     存储根目录（日志写入 basePath/logs/）
// @returns            AI 响应内容（已去除 markdown 代码块），或错误
func CallAI(surveyID, systemPrompt, userPrompt, basePath string) (string, error) {
	return CallAIWithValidation(surveyID, systemPrompt, userPrompt, basePath, false)
}

// CallAIWithValidation 调用 DeepSeek API，支持选择验证方式
// @param surveyID        问卷 ID（用于日志文件命名）
// @param systemPrompt   系统提示词
// @param userPrompt     用户提示词（含完整 HTML）
// @param basePath       存储根目录（日志写入 basePath/logs/）
// @param validateJSON   true=验证JSON格式，false=验证YAML格式
// @returns              AI 响应内容（已去除 markdown 代码块），或错误
func CallAIWithValidation(surveyID, systemPrompt, userPrompt, basePath string, validateJSON bool) (string, error) {
	cfg, err := loadDeepSeekEnvConfig()
	if err != nil {
		return "", fmt.Errorf("CallAI: load config: %w", err)
	}

	const maxRetries = 3
	var lastErr error
	var lastRawResponse string

	for attempt := 0; attempt < maxRetries; attempt++ {
		startReq := time.Now()
		content, statusCode, reqErr := doDeepSeekRequest(cfg, systemPrompt, userPrompt)
		elapsed := time.Since(startReq)

		// 网络/超时错误
		if reqErr != nil {
			msg := fmt.Sprintf("网络错误: %v, retry %d/%d", reqErr, attempt+1, maxRetries)
			writeAILog(basePath, surveyID, cfg.Model, attempt, msg, "", elapsed)
			lastErr = reqErr
			time.Sleep(1 * time.Second)
			continue
		}

		// API 认证失败 — 不重试
		if statusCode == 401 {
			authErr := fmt.Errorf("API 认证失败: invalid API key")
			writeAILog(basePath, surveyID, cfg.Model, attempt, authErr.Error(), "", elapsed)
			return "", authErr
		}

		// 限流 — 等待 5 秒重试
		if statusCode == 429 {
			msg := fmt.Sprintf("API 限流 (429), retry %d/%d after 5s", attempt+1, maxRetries)
			writeAILog(basePath, surveyID, cfg.Model, attempt, msg, "", elapsed)
			lastErr = fmt.Errorf("API 限流 (429)")
			time.Sleep(5 * time.Second)
			continue
		}

		// 服务器错误 — 等待 2 秒重试
		if statusCode >= 500 {
			msg := fmt.Sprintf("API 服务器错误 (%d), retry %d/%d", statusCode, attempt+1, maxRetries)
			writeAILog(basePath, surveyID, cfg.Model, attempt, msg, "", elapsed)
			lastErr = fmt.Errorf("API 服务器错误 (%d)", statusCode)
			time.Sleep(2 * time.Second)
			continue
		}

		// 其他非 200 — 不重试
		if statusCode != 200 {
			otherErr := fmt.Errorf("API 错误: HTTP %d", statusCode)
			writeAILog(basePath, surveyID, cfg.Model, attempt, otherErr.Error(), "", elapsed)
			return "", otherErr
		}

		// 成功 200：去除 markdown 代码块
		lastRawResponse = content
		cleaned := stripMarkdownFence(content)

		// 根据模式选择验证方式
		if validateJSON {
			if jsonErr := validateJSONString(cleaned); jsonErr != nil {
				msg := fmt.Sprintf("JSON 解析失败, retry %d/%d: %v", attempt+1, maxRetries, jsonErr)
				writeAILog(basePath, surveyID, cfg.Model, attempt, msg, content, elapsed)
				lastErr = fmt.Errorf("JSON 解析失败: %w", jsonErr)
				time.Sleep(1 * time.Second)
				continue
			}
		} else {
			// 默认验证 YAML
			if yamlErr := validateYAMLString(cleaned); yamlErr != nil {
				msg := fmt.Sprintf("YAML 解析失败, retry %d/%d: %v", attempt+1, maxRetries, yamlErr)
				writeAILog(basePath, surveyID, cfg.Model, attempt, msg, content, elapsed)
				lastErr = fmt.Errorf("YAML 解析失败: %w", yamlErr)
				time.Sleep(1 * time.Second)
				continue
			}
		}

		// 全部通过
		writeAILog(basePath, surveyID, cfg.Model, attempt, "成功", cleaned, elapsed)
		return cleaned, nil
	}

	// 所有重试均失败
	saveAIFailureLog(basePath, surveyID, systemPrompt, userPrompt, lastRawResponse, lastErr, maxRetries)
	return "", fmt.Errorf("调用 AI 失败，已重试 %d 次: %w", maxRetries, lastErr)
}

// validateJSONString 验证字符串是否为有效 JSON
func validateJSONString(s string) error {
	var out interface{}
	return json.Unmarshal([]byte(s), &out)
}

// doDeepSeekRequest 发起单次 HTTP 请求到 DeepSeek /chat/completions
// @returns content（AI 回复文本），statusCode，网络层错误
func doDeepSeekRequest(cfg *deepSeekEnvConfig, systemPrompt, userPrompt string) (string, int, error) {
	reqBody := deepSeekRequest{
		Model: cfg.Model,
		Messages: []deepSeekMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: cfg.Temperature,
		Stream:      false,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("marshal request: %w", err)
	}

	// 使用更宽松的超时配置：总超时 + 连接超时分开
	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second // 默认 120 秒
	}

	// 创建带有自定义 Transport 的 HTTP Client
	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     60 * time.Second,
		TLSHandshakeTimeout: 30 * time.Second,
	}

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	req, err := http.NewRequest("POST", cfg.BaseURL+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	// 始终读完 body 以释放连接
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", resp.StatusCode, nil
	}

	// 修复无效 UTF-8 序列，防止 JSON 解析失败
	respContent := strings.ToValidUTF8(string(respBytes), "�")

	var dsResp deepSeekResponse
	if err := json.Unmarshal([]byte(respContent), &dsResp); err != nil {
		return "", resp.StatusCode, fmt.Errorf("parse response JSON: %w", err)
	}
	if len(dsResp.Choices) == 0 || dsResp.Choices[0].Message.Content == "" {
		return "", resp.StatusCode, fmt.Errorf("AI 响应为空")
	}

	return dsResp.Choices[0].Message.Content, resp.StatusCode, nil
}

// stripMarkdownFence 去除 AI 输出中可能存在的 ```yaml ... ``` 包裹
func stripMarkdownFence(s string) string {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"```yaml", "```yml", "```"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimPrefix(s, prefix)
			break
		}
	}
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	// 去除控制字符（YAML 不允许控制字符）
	s = stripControlChars(s)
	return s
}

// stripControlChars 移除字符串中的控制字符
// YAML 规范不允许控制字符（0x00-0x1F，除 \t \n \r 外）
// 处理流程：先用 ToValidUTF8 修复无效 UTF-8 序列，再用 Unicode 码点过滤控制字符
func stripControlChars(s string) string {
	// 关键修复：将无效的 UTF-8 序列替换为 Unicode 替换字符
	// 解决 AI 输出中 "â\x80\x99" 等被误解的编码序列问题
	s = strings.ToValidUTF8(s, "\uFFFD")

	var result strings.Builder
	for _, r := range s {
		// 保留制表符、换行、回车
		if r == '\t' || r == '\n' || r == '\r' {
			result.WriteRune(r)
		// 保留所有可打印 Unicode 字符（0x20 及以上，不包括 DEL）
		// 但过滤 C1 控制字符 (0x80-0x9F)
		} else if r >= 0x20 && r < 0x7F {
			result.WriteRune(r)
		} else if r >= 0xA0 {
			// 保留 Latin-1 补充字符 (0xA0) 及以上
			result.WriteRune(r)
		}
		// Unicode C0 控制字符 (0x00-0x1F) 被过滤
		// DEL (0x7F) 被过滤
		// Unicode C1 控制字符 (0x80-0x9F) 被过滤（这是 YAML 规范的要求）
	}
	return result.String()
}

// validateYAMLString 检验字符串是否为合法 YAML
func validateYAMLString(s string) error {
	var out interface{}
	return yaml.Unmarshal([]byte(s), &out)
}

// writeAILog 追加步骤 4 日志到 {basePath}/logs/{surveyID}_{Ymd}.log
func writeAILog(basePath, surveyID, model string, attempt int, status, aiResponse string, elapsed time.Duration) {
	logDir := filepath.Join(basePath, "logs")
	_ = os.MkdirAll(logDir, 0755)

	date := time.Now().Format("20060102")
	logPath := filepath.Join(logDir, fmt.Sprintf("%s_%s.log", surveyID, date))

	sep := strings.Repeat("=", 80)
	var sb strings.Builder
	sb.WriteString(sep + "\n")
	sb.WriteString("【步骤 4】调用 AI 提取\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("时间: %s\n", time.Now().UTC().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("模型: %s\n", model))
	sb.WriteString(fmt.Sprintf("重试次数: %d/3\n", attempt))
	if aiResponse != "" {
		sb.WriteString("\n--- [AI 响应] ---\n")
		sb.WriteString(aiResponse + "\n")
	}
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("耗时: %.1fs\n", elapsed.Seconds()))
	sb.WriteString(fmt.Sprintf("状态: %s\n", status))
	sb.WriteString(sep + "\n")

	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(sb.String())
}

// saveAIFailureLog 将失败请求完整信息保存到
// {basePath}/logs/extract_failures/{surveyID}_{timestamp}.yaml
func saveAIFailureLog(basePath, surveyID, systemPrompt, userPrompt, rawResponse string, lastErr error, retries int) {
	dir := filepath.Join(basePath, "logs", "extract_failures")
	_ = os.MkdirAll(dir, 0755)

	ts := time.Now().Format("20060102_150405")
	path := filepath.Join(dir, fmt.Sprintf("%s_%s.yaml", surveyID, ts))

	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}

	entry := aiFailureLog{
		RequestTime:  time.Now().UTC().Format(time.RFC3339),
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		RawResponse:  rawResponse,
		ErrorType:    errMsg,
		RetryCount:   retries,
	}

	data, err := yaml.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}
