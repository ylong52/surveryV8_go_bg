package extractor

// yaml_prompt.go — YAML结构提取专用提示词构建
//   BuildYamlPrompt - 从HTML提取YAML结构专用的提示词（不截断，完整保留）

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const yamlPromptsYAMLPath = "core/extractor/yaml_prompts.yaml"

// YamlPrompt YAML专用提示词
type YamlPrompt struct {
	System string // 系统提示词
	User   string // 用户提示词（含完整HTML）
}

// yamlPromptsConfig 对应 yaml_prompts.yaml 的字段
type yamlPromptsConfig struct {
	SystemPrefix             string `yaml:"system_prefix"`
	MultiQuestionRules       string `yaml:"multi_question_rules"`
	QuestionTypeRules        string `yaml:"question_type_rules"`
	FieldExtractionRules     string `yaml:"field_extraction_rules"`
	QuestionTypeDifferential string `yaml:"question_type_differential"`
	ResponseFormatRules      string `yaml:"response_format_rules"`
	Prohibitions            string `yaml:"prohibitions"`
	OutputFormatRules        string `yaml:"output_format_rules"`
	PageDetectionRules       string `yaml:"page_detection_rules"`
}

// loadYamlPromptsConfig 读取 yaml_prompts.yaml
func loadYamlPromptsConfig() (yamlPromptsConfig, error) {
	data, err := os.ReadFile(yamlPromptsYAMLPath)
	if err != nil {
		return yamlPromptsConfig{}, fmt.Errorf("loadYamlPromptsConfig: read %s: %w", yamlPromptsYAMLPath, err)
	}

	var cfg yamlPromptsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return yamlPromptsConfig{}, fmt.Errorf("loadYamlPromptsConfig: parse yaml: %w", err)
	}
	return cfg, nil
}

// BuildYamlPrompt 构建YAML提取专用提示词
//   - 检查HTML长度，超过60000直接报错（不截断）
//   - 传给AI时保留完整HTML
//   - 日志中保存完整HTML
// @param surveyID  问卷ID
// @param pageID    页面ID
// @param cleanHTML 清洗后HTML（不超过60000字符）
// @returns         YamlPrompt，或错误
func BuildYamlPrompt(surveyID, pageID, cleanHTML string) (YamlPrompt, error) {
	// 检查HTML长度，超过60000直接报错
	if len(cleanHTML) > 60000 {
		return YamlPrompt{}, fmt.Errorf("HTML长度 %d 超过60000字符限制", len(cleanHTML))
	}

	// 读取配置
	cfg, err := loadYamlPromptsConfig()
	if err != nil {
		return YamlPrompt{}, fmt.Errorf("BuildYamlPrompt: %w", err)
	}

	// 拼接系统提示词
	systemPrompt := cfg.SystemPrefix + "\n" +
		cfg.QuestionTypeRules + "\n" +
		cfg.MultiQuestionRules + "\n" +
		cfg.FieldExtractionRules + "\n" +
		cfg.QuestionTypeDifferential + "\n" +
		cfg.ResponseFormatRules + "\n" +
		cfg.PageDetectionRules + "\n" +
		cfg.Prohibitions + "\n" +
		cfg.OutputFormatRules

	// 构建用户提示词
	userPrompt := "【页面信息】\n" +
		"survey_id: " + surveyID + "\n" +
		"page_id: " + pageID + "\n\n" +
		"【HTML内容】\n" +
		cleanHTML

	return YamlPrompt{System: systemPrompt, User: userPrompt}, nil
}

// BuildYamlPromptFromMeta 从PageMetadata构建YAML专用提示词
// @param surveyID  问卷ID
// @param meta      页面元数据
// @param cleanHTML 清洗后HTML
// @returns         YamlPrompt，或错误
func BuildYamlPromptFromMeta(surveyID string, meta PageMetadata, cleanHTML string) (YamlPrompt, error) {
	return BuildYamlPrompt(surveyID, meta.PageID, cleanHTML)
}

// GetHtmlPreview 返回HTML的前300字符预览（用于控制台输出）
func (yp *YamlPrompt) GetHtmlPreview(cleanHTML string) string {
	if len(cleanHTML) <= 300 {
		return cleanHTML
	}
	return cleanHTML[:300] + "..."
}

// WriteYamlPromptLog 将YAML提示词写入日志文件（完整保存，不截断）
// @param basePath  存储根目录
// @param surveyID   问卷ID
// @param meta       页面元数据
// @param yamlPrompt YAML提示词
// @param fullHTML   完整HTML（不截断）
func WriteYamlPromptLog(basePath, surveyID string, meta PageMetadata, yamlPrompt YamlPrompt, fullHTML string) error {
	logDir := basePath + "/logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("WriteYamlPromptLog: mkdir %s: %w", logDir, err)
	}

	sep := strings.Repeat("=", 80)
	var sb strings.Builder

	sb.WriteString(sep + "\n")
	sb.WriteString("【步骤4】YAML提示词整理\n")
	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("时间: %s\n", meta.Timestamp))
	sb.WriteString(fmt.Sprintf("survey_id: %s\n", surveyID))
	sb.WriteString(fmt.Sprintf("page_id: %s\n", meta.PageID))

	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("=== System Prompt (%d字符) ===\n", len(yamlPrompt.System)))
	sb.WriteString(yamlPrompt.System + "\n")

	sb.WriteString(sep + "\n")
	sb.WriteString(fmt.Sprintf("=== User Prompt (%d字符，含HTML) ===\n", len(yamlPrompt.User)))
	sb.WriteString(yamlPrompt.User + "\n")

	logPath := fmt.Sprintf("%s/logs/%s_%s.log", basePath, surveyID, getTodayDate())
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("WriteYamlPromptLog: open %s: %w", logPath, err)
	}
	defer f.Close()

	if _, err := f.WriteString(sb.String()); err != nil {
		return fmt.Errorf("WriteYamlPromptLog: write %s: %w", logPath, err)
	}
	return nil
}

// getTodayDate 返回今天日期格式 20060102
func getTodayDate() string {
	return time.Now().Format("20060102")
}
