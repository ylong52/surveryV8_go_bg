package extractor

// screenshot_ocr.go — 截图两步 OCR 识别与分析
//
//  第一步（视觉多模型投票）：
//    Qwen VL + Claude 并行识别 → 逐题投票裁决（最多重试5轮）→ 最终人工标注答案
//  第二步（分析比对）：
//    将投票后的最终答案与推荐答案比对 → match/mismatch/unclear
//
//  投票流程：
//    - 并行调用 Qwen VL + Claude（若 API key 已配置）
//    - 逐题对齐各模型答案后统计票数
//    - 全票一致 → confirmed / 多数 → majority / 平票 → uncertain（重试）
//    - 仍 uncertain → human_review_required
//    - 详细投票过程写入 AI 事件日志
//
//  主入口：OCRAndAnalyze(screenshotPath, recommendAnswerJSON, surveyID, basePath)
//    → 返回 ocrJSON(ocr_step+analysis_step), ocrVerified(match/mismatch/unclear/human_review_required), error

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"survey.cli/core/database"

	"gopkg.in/yaml.v3"
)

// =============================================================================
// 数据结构
// =============================================================================

// ocrStepInteraction 第一步 OCR 识别：单题结果
type ocrStepInteraction struct {
	Subject      string      `json:"subject"`
	QuestionType string      `json:"question_type"`
	UserSelected interface{} `json:"user_selected"` // string / []string / matrixUserSelected / null
	Note         string      `json:"note,omitempty"`
}

// ocrStep 第一步完整结果
type ocrStep struct {
	Model        string               `json:"model"`
	Interactions []ocrStepInteraction `json:"interactions"`
	PageNote     string               `json:"page_note,omitempty"`
}

// analysisInteraction 第二步分析：单题结果
type analysisInteraction struct {
	Subject                string      `json:"subject"`
	QuestionType           string      `json:"question_type"`
	RecommendedAnswers     interface{} `json:"recommended_answers"`
	UserSelected           interface{} `json:"user_selected"`
	FollowedRecommendation *bool      `json:"followed_recommendation"` // nil=null
	MatchMethod            string      `json:"match_method"`
}

// analysisStep 第二步完整结果
type analysisStep struct {
	Interactions []analysisInteraction `json:"interactions"`
	Overall      string               `json:"overall"` // match / mismatch / unclear
}

// OCRFullResult ocr_json 字段最终存储格式（含多模型投票结果）
type OCRFullResult struct {
	OCRStep      ocrStep      `json:"ocr_step"`
	AnalysisStep analysisStep `json:"analysis_step"`
}

// recommendInteraction 解析 recommend_answer 用的临时结构
type recommendInteraction struct {
	QuestionType       string      `json:"question_type"`
	Subject            string      `json:"subject"`
	RecommendedAnswers interface{} `json:"recommended_answers"`
}

// matrixRow 矩阵题单行识别结果
type matrixRow struct {
	RowIndex    int    `json:"row_index"`
	RowText     string `json:"row_text"`
	SelectedCol *int   `json:"selected_col"` // null = 无法识别
}

// matrixUserSelected 矩阵题 user_selected 完整结构
type matrixUserSelected struct {
	TotalRows int         `json:"total_rows"`
	TotalCols int         `json:"total_cols"`
	ColLabels []string    `json:"col_labels"`
	Rows      []matrixRow `json:"rows"`
}

// recommendPage 解析 recommend_answer 后的页面级结构
type recommendPage struct {
	IsMatrixPage bool
	Interactions []recommendInteraction
}

// =============================================================================
// 多模型投票 — 数据结构
// =============================================================================

// interactionItem 推荐答案中提取的题目信息（用于 prompt 构建）
type interactionItem struct {
	Subject      string `json:"subject"`
	QuestionType string `json:"question_type"`
}

// OCRInteraction 多模型解析后的单题答案（来自单个模型）
type OCRInteraction struct {
	Subject      string      `json:"subject"`
	QuestionType string      `json:"question_type"`
	UserSelected interface{} `json:"user_selected"`
	Note         string      `json:"note,omitempty"`
}

// MatrixRowOCR 多模型解析后的矩阵行
type MatrixRowOCR struct {
	RowIndex    int         `json:"row_index"`
	RowText     string      `json:"row_text"`
	SelectedCol interface{} `json:"selected_col"` // int 或 null
	Note        string      `json:"note,omitempty"`
}

// ParsedResult 单个模型的解析结果
type ParsedResult struct {
	PageType  string            `json:"page_type"` // "normal" / "matrix"
	Items     []OCRInteraction  `json:"interactions"`
	Rows      []MatrixRowOCR    `json:"rows"`
	ColLabels []string          `json:"col_labels"`
	TotalRows int               `json:"total_rows"`
	TotalCols int               `json:"total_cols"`
	PageNote  string            `json:"page_note,omitempty"`
}

// ModelResult 单个模型的完整结果
type ModelResult struct {
	ModelName string        `json:"model_name"`
	Parsed    ParsedResult  `json:"parsed"`
	RawJSON   string        `json:"raw_json"`
	Elapsed   time.Duration  `json:"elapsed_ms"`
	Error     string        `json:"error,omitempty"` // 错误信息，Error!=nil 时填入
}

// VoteStatus 投票状态
type VoteStatus string

const (
	VoteConfirmed   VoteStatus = "confirmed"
	VoteMajority   VoteStatus = "majority"
	VoteUncertain  VoteStatus = "uncertain"
	VoteHumanReview VoteStatus = "human_review_required"
)

// VotedItem 单题的投票结果
type VotedItem struct {
	Index        int           `json:"index"`
	Subject      string        `json:"subject"`
	QuestionType string        `json:"question_type"`
	FinalAnswer  interface{}   `json:"final_answer"`
	Status       VoteStatus    `json:"status"`        // confirmed / majority / uncertain / human_review_required
	ModelAnswers map[string]interface{} `json:"model_answers"` // 各模型原始答案
	Note         string        `json:"note,omitempty"`
	ColLabels    []string      `json:"col_labels,omitempty"` // 矩阵页列标签
}

// =============================================================================
// 配置加载（复用 env.yaml → volces_vision + anthropic）
// =============================================================================

const ocrEnvYAMLPath = "configs/env.yaml"

type volcesVisionCfg struct {
	APIKey      string  `yaml:"api_key"`
	BaseURL     string  `yaml:"base_url"`
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	Timeout     int     `yaml:"timeout"`
}

type envOCRFile struct {
	VolcesVision volcesVisionCfg `yaml:"volces_vision"`
	Anthropic    anthropicCfg    `yaml:"anthropic"`
}

func loadVolcesVisionConfig() (*volcesVisionCfg, error) {
	data, err := os.ReadFile(ocrEnvYAMLPath)
	if err != nil {
		return nil, fmt.Errorf("loadVolcesVisionConfig: 读取 %s: %w", ocrEnvYAMLPath, err)
	}
	var f envOCRFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("loadVolcesVisionConfig: 解析 %s: %w", ocrEnvYAMLPath, err)
	}
	cfg := &f.VolcesVision
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("volces_vision.api_key 为空，请检查 %s", ocrEnvYAMLPath)
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	if cfg.Model == "" {
		cfg.Model = "qwen-vl-plus"
	}
	if cfg.Temperature <= 0 {
		cfg.Temperature = 0.7
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120
	}
	return cfg, nil
}

func loadAnthropicVisionConfig() (*anthropicCfg, error) {
	data, err := os.ReadFile(ocrEnvYAMLPath)
	if err != nil {
		return nil, fmt.Errorf("loadAnthropicVisionConfig: 读取 %s: %w", ocrEnvYAMLPath, err)
	}
	var f envOCRFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("loadAnthropicVisionConfig: 解析 %s: %w", ocrEnvYAMLPath, err)
	}
	cfg := &f.Anthropic
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic.api_key 为空，跳过 Claude 视觉模型")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-6"
	}
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 4096
	}
	if cfg.Temperature <= 0 {
		cfg.Temperature = 0.0
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 180
	}
	return cfg, nil
}

// =============================================================================
// 提示词常量（fallback）
// =============================================================================

var fallbackOCRSystemPrompt = `你是精细化的问卷截图识别助手。

【核心任务】
扫描问卷截图，识别其中所有可见题目和用户选中的答案，输出 JSON。无论题目是什么，都要识别。

【图像处理与识别规则】
1. 滚动去重与状态校验：
   - 截图若包含滚动页面，同一元素可能多次出现。必须对比所有出现位置，以"最后/最清晰"的视觉呈现为准。
   - 重点识别选中（高亮/勾选/边框颜色变化）状态。
2. 多选框（Checkbox）识别增强：
   - 严格检查所有选项的视觉状态（如背景色高亮、选中图标）。
   - 严禁因选项过多而漏掉边缘项（如截图底部或顶部的选中项）。
3. 按钮式多选题识别增强（重点！极易漏识别）：
   - 按钮式多选题的特征：多个选项横向或纵向排列，每个都是一个独立按钮/色块。
   - 选中状态判断：按钮有深色/绿色/蓝色填充背景，或边框颜色明显不同于未选中按钮。
   - 未选中状态：按钮为白色/浅色背景，视觉上与选中按钮差异明显。
   - ⚠️ 扫描策略（强制执行）：
     1. 从左到右（或从上到下）依次检查每一个按钮。
     2. 遇到第一个高亮按钮后，继续扫描下一个按钮，不能停止。
     3. 必须遍历完所有按钮后才能输出结果，严禁在找到第一个选中项后停止扫描。
     4. 按钮横向排列一行时，尤其容易只报告最左/最右一个被选中的，需特别警惕。
   - 典型错误：实际选中了 [A][B][C] 三个按钮，但模型只报告 ["A"]。
   - 颜色阈值：如果无法确定一个按钮是否被选中，按"选中"处理，宁可多选不可漏选。
4. Likert 量表识别（重要！）：
   - Likert 量表（如 Strongly Disagree / Disagree / Agree / Strongly Agree，或 Never / Rarely / Sometimes / Often / Always 等）属于普通单选题。
   - 只要截图中能看到选项文字且能判断哪个被选中，就必须输出 user_selected 为该选项的标签文字（如 "Strongly Agree"），而不是 null。
   - "likert_card" 特指下拉选择或卡片翻转题，其选项在截图中完全不可见，这种情况下才返回 null。
5. 动态属性：
   - 无法识别/找不到：user_selected 返回 null，并记录原因。

【陷阱题识别规则】（严禁违反）
- 识别包含以下关键词的题目为陷阱题：
  "For data quality purposes", "please select", "attention check", "quality check",
  "select strongly agree", "select somewhat agree", "select agree", "choose agree",
  "must select", "make sure you select"
- 陷阱题与普通题相同，必须以视觉圆点为准，忠实记录用户的实际勾选。
- 严禁用题干语义覆盖用户的真实选择。
- 在 note 中标记 "attention_check_detected"，并额外记录题干要求的选项，
  例如：note = "attention_check_detected | expected: Somewhat Agree"
- 视觉圆点优先级：视觉圆点 > 语义指令。此为强制规则，不可绕过。

【极性反转题识别规则】（严禁违反）
- 如果题干是负向感受陈述（如 creeps me out / bores me / annoys me / disgusts me / embarrasses me / upsets me / irritates me / makes me uncomfortable 等），选 "Agree" / "Strongly Agree" 表示用户认同该负面感受（即：我确实有此感受）；选 "Disagree" / "Strongly Disagree" 表示用户否认该负面感受（即：我没有此感受）。
- 严禁将负向题中的 "Agree" 理解为"不认可题干"，必须结合语义和圆点位置综合判断。
- 严禁在负向题上因语义歧义而输出 null 或跳题。

【相邻列识别规则】（严禁违反）
- Likert 四列场景（Strongly Disagree / Somewhat Disagree / Somewhat Agree / Strongly Agree），Somewhat Agree 和 Somewhat Disagree 为左右相邻列，最容易误判。
- 严禁在相邻列之间模棱两可就随便选一个，必须明确判断圆点位于中左（Somewhat Disagree）还是中右（Somewhat Agree）。
- 如果无法明确判断，必须输出 note 标记 "adjacent_col_uncertain"，严禁不写备注就随意填写。

【输出规则】
- 必须输出纯 JSON，禁止包含 Markdown 标记（如代码块），禁止任何解释。
- 格式要求：
  - 单选：user_selected 为字符串。
  - 多选：user_selected 为字符串数组（按视觉顺序或题目原定顺序排列）。
  - 文本题：user_selected 为字符串。
  - Likert 量表：user_selected 为字符串（选项标签文字）。
  - likert_card（选项不可见）：user_selected 为 null。
  - 其他无法识别：user_selected 为 null。

【输出示例】
假设截图中有以下题目：
  题干: "I love watching action movies"
  选项: [Never] [Rarely] [Sometimes] [Often] [Always]
  用户选中了 "Often"

正确输出：
{
  "interactions": [
    {
      "subject": "I love watching action movies",
      "question_type": "single",
      "user_selected": "Often",
      "note": ""
    }
  ],
  "page_note": ""
}

【当前环境约束】
- 你具备极强的多模态视觉捕捉能力，需仔细扫描每一行选项，特别是视觉区分度较低的选中态，确保不漏掉任何一个被选中的项目。

【按钮式多选题输出示例】
假设截图中有以下按钮式多选题：
  题干: "What is your racial background?"
  选项横向排列: [White/Caucasian] [Asian/Asian-American] [Black/African-American] [Other]
  White/Caucasian 和 Asian/Asian-American 两个按钮均为绿色高亮背景（选中态）

正确输出：
{
  "interactions": [
    {
      "subject": "What is your racial background?",
      "question_type": "multi",
      "user_selected": ["White/Caucasian", "Asian/Asian-American"],
      "note": ""
    }
  ],
  "page_note": ""
}

⚠️ 常见错误示例（严禁这样做）：
{
  "user_selected": ["White/Caucasian"]   // ❌ 漏选了 Asian/Asian-American
}
{
  "user_selected": "White/Caucasian"       // ❌ 多选题必须用数组，不是字符串
}

` // ← fallbackOCRSystemPrompt 结束

var fallbackOCRSystemPromptMatrix = `你是精细化的问卷截图识别助手，专门处理矩阵表格题。

【核心任务】
根据矩阵行列结构，精准提取截图中的用户交互结果。

【陷阱题识别规则】（矩阵题适用）（严禁违反）
- 识别包含以下关键词的题目为陷阱题：
  "For data quality purposes", "please select", "attention check", "quality check",
  "select strongly agree", "select somewhat agree", "select agree", "choose agree",
  "must select", "make sure you select"
- 陷阱题与普通题相同，必须以视觉圆点为准，忠实记录用户的实际勾选。
- 严禁用题干语义覆盖用户的真实选择。
- 在 note 中标记 "attention_check_detected"，并额外记录题干要求的选项，
  例如：note = "attention_check_detected | expected: Somewhat Agree"
- 视觉圆点优先级：视觉圆点 > 语义指令。此为强制规则，不可绕过。

【极性反转题识别规则】（矩阵题适用）（严禁违反）
- 如果某行题干是负向感受陈述（如 creeps me out / bores me / annoys me / disgusts me / embarrasses me / upsets me / irritates me / makes me uncomfortable 等），选 "Agree" / "Strongly Agree" 表示用户认同该负面感受（即：我确实有此感受）；选 "Disagree" / "Strongly Disagree" 表示用户否认该负面感受（即：我没有此感受）。
- 严禁将负向题中的 "Agree" 理解为"不认可题干"，必须结合语义和圆点位置综合判断。
- 严禁在负向题上因语义歧义而输出 null 或跳题。

【相邻列识别规则】（矩阵题适用）（严禁违反）
- Likert 四列场景（Strongly Disagree / Somewhat Disagree / Somewhat Agree / Strongly Agree），Somewhat Agree 和 Somewhat Disagree 为左右相邻列，最容易误判。
- 严禁在相邻列之间模棱两可就随便选一个，必须明确判断圆点位于中左（Somewhat Disagree）还是中右（Somewhat Agree）。
- 如果无法明确判断，必须输出 note 标记 "adjacent_col_uncertain"，严禁不写备注就随意填写。

【图像处理与识别规则】
1. 滚动去重与状态校验：
   - 截图若包含滚动页面，同一元素可能多次出现。必须对比所有出现位置，以"最后/最清晰"的视觉呈现为准。
   - 重点识别每行的选中列（高亮/勾选/边框颜色变化）。
2. 矩阵识别增强：
   - 严格检查每行的列标签（col_labels）和选中状态。
   - col_labels：从截图表头读取每列的文字，顺序从左到右。
   - selected_col：整数，1 = 最左列，依次向右递增，null = 无法识别该行选择。
   - 严禁漏掉任何一行或一列。

【输出规则】
- 必须输出纯 JSON，禁止包含 Markdown 标记（如代码块），禁止任何解释。
- 格式要求：
{
  "page_type": "matrix",
  "total_rows": <行数>,
  "total_cols": <列数>,
  "col_labels": ["列标题1", "列标题2", ...],
  "rows": [
    {"row_index": 1, "row_text": "子题文字原文", "selected_col": <列号整数或null>, "note": ""},
    {"row_index": 2, "row_text": "子题文字原文", "selected_col": <列号整数或null>, "note": ""}
  ]
}`

// ocrSystemPrompt 运行时提示词（数据库优先，fallback 兜底）
var ocrSystemPrompt string

// ocrSystemPromptMatrix 矩阵题运行时提示词（数据库优先，fallback 兜底）
var ocrSystemPromptMatrix string

// InitOCRPrompts 从数据库加载 OCR 提示词，需在 database.Init() 之后调用
func InitOCRPrompts() {
	if content, err := database.LoadPromptByKey("ocr.system"); err == nil && content != "" {
		ocrSystemPrompt = content
		fmt.Printf("[OCR] 从数据库加载 ocr.system 提示词（%d 字符）\n", len(content))
	} else {
		ocrSystemPrompt = fallbackOCRSystemPrompt
		fmt.Printf("[OCR] 使用 fallback ocr.system 提示词\n")
	}

	if content, err := database.LoadPromptByKey("ocr.system.matrix"); err == nil && content != "" {
		ocrSystemPromptMatrix = content
		fmt.Printf("[OCR] 从数据库加载 ocr.system.matrix 提示词（%d 字符）\n", len(content))
	} else {
		ocrSystemPromptMatrix = fallbackOCRSystemPromptMatrix
		fmt.Printf("[OCR] 使用 fallback ocr.system.matrix 提示词\n")
	}
}

// =============================================================================
// 封闭题题型集合
// =============================================================================

var closedQuestionTypes = map[string]bool{
	"single":           true,
	"multi":            true,
	"matrix_radio":     true,
	"matrix_checkbox":  true,
	"dropdown":         true,
	"ranking":          true,
}

// =============================================================================
// 主入口
// =============================================================================

// OCRAndAnalyze 对截图执行两步 OCR + 分析，返回 ocr_json 和 ocr_verified
//
//  步骤1（多模型投票）：
//    - 并行调用 Qwen VL + Claude（若已配置）
//    - 逐题对齐各模型答案，投票裁决（最多重试5轮）
//    - 投票结果写 AI 事件日志
//  步骤2（分析比对）：
//    - 将投票后的最终答案与推荐答案比对
//    - 封闭题代码精确匹配 / 开放题 DeepSeek 语义比对 / likert_card 跳过
//
// @param screenshotPath     截图文件路径
// @param recommendAnswerJSON recommend_answer 字段原始 JSON 字符串
// @param surveyID           问卷 ID（用于日志）
// @param basePath           存储根目录（日志写入）
// @returns ocrJSON(完整JSON字符串), ocrVerified(match/mismatch/unclear/human_review_required), error
func OCRAndAnalyze(screenshotPath, recommendAnswerJSON, surveyID, basePath string) (string, string, error) {
	if screenshotPath == "" {
		return "", "unclear", fmt.Errorf("screenshot_path 为空")
	}
	if recommendAnswerJSON == "" {
		return "", "unclear", fmt.Errorf("recommend_answer 为空，无题目数据")
	}

	page, err := parseRecommendPage(recommendAnswerJSON)
	if err != nil {
		return "", "unclear", fmt.Errorf("解析 recommend_answer 失败: %w", err)
	}
	if len(page.Interactions) == 0 {
		return "", "unclear", fmt.Errorf("recommend_answer 中无 interactions 数据")
	}

	imageBytes, err := os.ReadFile(screenshotPath)
	if err != nil {
		return "", "unclear", fmt.Errorf("读取截图文件失败: %w", err)
	}
	imageBase64 := base64.StdEncoding.EncodeToString(imageBytes)

	fmt.Printf("[OCR] 步骤1: 多模型投票识别，题目数=%d，矩阵页=%v\n", len(page.Interactions), page.IsMatrixPage)

	// 将 recommendInteraction 转为 interactionItem（用于 prompt 构建）
	hintItems := make([]interactionItem, len(page.Interactions))
	for i, it := range page.Interactions {
		hintItems[i] = interactionItem{Subject: it.Subject, QuestionType: it.QuestionType}
	}

	// 多模型投票识别 → VotedItem 列表
	votedItems, modelResults := runVotingOCR(imageBase64, hintItems, page.IsMatrixPage, surveyID)

	// 步骤2：将投票结果与推荐答案比对
	fmt.Printf("[OCR] 步骤2: 开始分析比对\n")

	// 矩阵页：将投票结果重建为 matrixUserSelected 供分析比对使用
	var matrixForAnalysis *matrixUserSelected
	if page.IsMatrixPage && votedItems != nil {
		m := votedItemsToMatrix(votedItems, page.Interactions)
		matrixForAnalysis = &m
	}

	step2 := runAnalysisStepFromVoted(votedItems, page.Interactions, page.IsMatrixPage, matrixForAnalysis, surveyID, basePath)

	// 组装 ocrStep（兼容旧格式，同时嵌入投票后答案）
	ocrInteractions := votedItemsToOCRStepInteractions(votedItems, page.IsMatrixPage)
	step1 := &ocrStep{
		Model:        buildModelNames(modelResults),
		Interactions: ocrInteractions,
	}

	// 写 AI 事件日志
	writeOCRVotingLog(surveyID, modelResults, votedItems)

	full := OCRFullResult{
		OCRStep:      *step1,
		AnalysisStep: step2,
	}
	jsonBytes, err := json.Marshal(full)
	if err != nil {
		return "", "unclear", fmt.Errorf("序列化 OCR 结果失败: %w", err)
	}

	fmt.Printf("[OCR] 完成: overall=%s\n", step2.Overall)
	return string(jsonBytes), step2.Overall, nil
}

// buildModelNames 拼接调用的模型名称
func buildModelNames(results []ModelResult) string {
	var names []string
	for _, r := range results {
		if r.Error == "" {
			names = append(names, r.ModelName)
		}
	}
	return strings.Join(names, "+")
}

// votedItemsToOCRStepInteractions 将投票结果转为旧格式 ocrStepInteraction
// 矩阵页：重建 matrixUserSelected 结构；普通页：逐题展开
func votedItemsToOCRStepInteractions(votedItems []VotedItem, isMatrix bool) []ocrStepInteraction {
	if len(votedItems) == 0 {
		return nil
	}

	// 矩阵页：重建 matrixUserSelected，用 votedItems 中第一项的 ColLabels
	if isMatrix {
		colLabels := votedItems[0].ColLabels
		rows := make([]matrixRow, 0, len(votedItems))
		for _, item := range votedItems {
			col := -1
			switch v := item.FinalAnswer.(type) {
			case float64:
				col = int(v)
			case int:
				col = v
			}
			var colPtr *int
			if col > 0 {
				colPtr = &col
			}
			rows = append(rows, matrixRow{
				RowIndex:    item.Index,
				RowText:     item.Subject,
				SelectedCol: colPtr,
			})
		}
		matrixData := matrixUserSelected{
			TotalRows: len(rows),
			TotalCols: len(colLabels),
			ColLabels: colLabels,
			Rows:      rows,
		}
		return []ocrStepInteraction{{
			Subject:      "matrix_page",
			QuestionType: "matrix_page",
			UserSelected: matrixData,
			Note:         "",
		}}
	}

	// 普通页：逐题展开
	result := make([]ocrStepInteraction, 0, len(votedItems))
	for _, item := range votedItems {
		result = append(result, ocrStepInteraction{
			Subject:      item.Subject,
			QuestionType: item.QuestionType,
			UserSelected: item.FinalAnswer,
			Note:         item.Note,
		})
	}
	return result
}

// votedItemsToMatrix reconstructs matrixUserSelected from votedItems (for matrix analysis step)
func votedItemsToMatrix(votedItems []VotedItem, origInteractions []recommendInteraction) matrixUserSelected {
	colLabels := votedItems[0].ColLabels
	rows := make([]matrixRow, 0, len(votedItems))
	for i, item := range votedItems {
		if item.QuestionType != "matrix_row" {
			continue
		}
		col := -1
		switch v := item.FinalAnswer.(type) {
		case float64:
			col = int(v)
		case int:
			col = v
		}
		var colPtr *int
		if col > 0 {
			colPtr = &col
		}
		// 优先用原始题干的 row_text（更干净），否则用 subject
		rowText := item.Subject
		if i < len(origInteractions) {
			rowText = origInteractions[i].Subject
		}
		rows = append(rows, matrixRow{
			RowIndex:    item.Index,
			RowText:     rowText,
			SelectedCol: colPtr,
		})
	}
	return matrixUserSelected{
		TotalRows: len(rows),
		TotalCols: len(colLabels),
		ColLabels: colLabels,
		Rows:      rows,
	}
}

// parseRecommendPage 从 recommend_answer JSON 提取页面级结构（含 is_matrix_page）
func parseRecommendPage(jsonStr string) (*recommendPage, error) {
	var wrapped struct {
		Data struct {
			PageMetadata struct {
				IsMatrixPage bool `json:"is_matrix_page"`
			} `json:"page_metadata"`
			Interactions []recommendInteraction `json:"interactions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &wrapped); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}

	isMatrix := wrapped.Data.PageMetadata.IsMatrixPage
	for _, it := range wrapped.Data.Interactions {
		if strings.HasPrefix(it.QuestionType, "matrix") {
			isMatrix = true
			break
		}
	}

	return &recommendPage{
		IsMatrixPage: isMatrix,
		Interactions: wrapped.Data.Interactions,
	}, nil
}

// =============================================================================
// 步骤1核心：多模型并行 + 投票
// =============================================================================

// runVotingOCR 多模型并行 OCR + 投票裁决入口
func runVotingOCR(imageBase64 string, interactions []interactionItem, isMatrix bool, surveyID string) ([]VotedItem, []ModelResult) {
	// 选择 system prompt
	systemPrompt := ocrSystemPrompt
	userPrompt := buildOCRUserPrompt(interactions)
	if isMatrix {
		systemPrompt = ocrSystemPromptMatrix
		userPrompt = buildMatrixUserPrompt(interactions)
	}

	// 打印活动模型
	models := getActiveVisionModels()
	modelNames := make([]string, len(models))
	for i, m := range models {
		modelNames[i] = m.name
	}
	mode := "普通"
	if isMatrix {
		mode = "矩阵"
	}
	fmt.Printf("[OCR] %s模式 | 并行调用: %s\n", mode, strings.Join(modelNames, " + "))

	// 并行调用所有模型
	fmt.Printf("[OCR] 正在并行调用 %d 个模型...\n", len(models))
	t0 := time.Now()
	initial := callAllVisionModels(systemPrompt, userPrompt, imageBase64, "image/jpeg")
	fmt.Printf("[OCR] 调用完成，总耗时=%.1fs（并行）\n", time.Since(t0).Seconds())

	for _, r := range initial {
		if r.Error != "" {
			fmt.Printf("[OCR]   ✗ %-8s 失败: %s\n", r.ModelName, r.Error)
		} else {
			qCount := len(r.Parsed.Items)
			if r.Parsed.PageType == "matrix" {
				qCount = len(r.Parsed.Rows)
			}
			fmt.Printf("[OCR]   ✓ %-8s %.1fs | %d 题\n", r.ModelName, float64(r.Elapsed)/1000.0, qCount)
		}
	}

	// 投票裁决（含重试，最多5轮）
	fmt.Printf("[OCR] 开始多模型投票裁决（最多重试 5 次）...\n")
	voted := runVotingWithRetry(systemPrompt, userPrompt, imageBase64, "image/jpeg", initial, 5)
	if voted == nil {
		fmt.Printf("[OCR] 所有模型均调用失败，无法进行投票\n")
		return nil, initial
	}

	// 汇总统计
	counts := map[VoteStatus]int{}
	for _, item := range voted {
		counts[item.Status]++
	}
	fmt.Printf("[OCR] 投票汇总: total=%d | confirmed=%d | majority=%d | human_review=%d\n",
		len(voted), counts[VoteConfirmed], counts[VoteMajority], counts[VoteHumanReview])

	return voted, initial
}

// =============================================================================
// 多模型配置与调用
// =============================================================================

type visionCaller struct {
	name string
	fn   func(string, string, string, string) (string, error)
}

func getActiveVisionModels() []visionCaller {
	models := []visionCaller{}

	// Qwen VL 始终可用
	models = append(models, visionCaller{name: "Qwen", fn: callVolcesVLVision})

	// Claude 若已配置则加入
	cfg, err := loadAnthropicVisionConfig()
	if err == nil && cfg.APIKey != "" {
		models = append(models, visionCaller{name: "Claude", fn: func(sys, user, img64, mime string) (string, error) {
			return callClaudeVLVision(cfg, sys, user, img64, mime)
		}})
	}
	return models
}

// callVolcesVLVision 调用 Qwen VL（OpenAI-compatible）
func callVolcesVLVision(systemPrompt, userPrompt, imageBase64, mimeType string) (string, error) {
	cfg, err := loadVolcesVisionConfig()
	if err != nil {
		return "", err
	}
	return callQwenVLWithPrompt(cfg, systemPrompt, userPrompt, imageBase64)
}

// callClaudeVLVision 调用 Claude Vision（Anthropic native API）
func callClaudeVLVision(cfg *anthropicCfg, systemPrompt, userPrompt, imageBase64, mimeType string) (string, error) {
	mediaType := strings.SplitN(mimeType, ";", 2)[0]
	if mediaType == "" {
		mediaType = "image/jpeg"
	}

	type imgPart struct {
		Type   string `json:"type"`
		Source struct {
			Type       string `json:"type"`
			MediaType  string `json:"media_type"`
			Data       string `json:"data"`
		} `json:"source"`
	}
	type textPart struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}

	body := map[string]interface{}{
		"model":       cfg.Model,
		"max_tokens":  cfg.MaxTokens,
		"system":      systemPrompt,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []interface{}{
					textPart{Type: "text", Text: userPrompt},
					imgPart{
						Type: "image",
						Source: struct {
							Type       string `json:"type"`
							MediaType  string `json:"media_type"`
							Data       string `json:"data"`
						}{
							Type:      "base64",
							MediaType: mediaType,
							Data:      imageBase64,
						},
					},
				},
			},
		},
		"temperature": cfg.Temperature,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal claude vision request: %w", err)
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	if timeout == 0 {
		timeout = 180 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	url := cfg.BaseURL + "/v1/messages"
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create claude vision request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("claude vision HTTP: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read claude vision response: %w", err)
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("claude vision HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", fmt.Errorf("parse claude vision JSON: %w", err)
	}
	for _, block := range result.Content {
		if block.Type == "text" && block.Text != "" {
			return cleanJSON(block.Text), nil
		}
	}
	return "", fmt.Errorf("Claude 返回为空")
}

// callAllVisionModels 并行调用所有已配置的视觉模型
func callAllVisionModels(sys, user, img64, mime string) []ModelResult {
	callers := getActiveVisionModels()
	results := make([]ModelResult, len(callers))
	var wg sync.WaitGroup
	for i, c := range callers {
		wg.Add(1)
		go func(idx int, caller visionCaller) {
			defer wg.Done()
			start := time.Now()
			raw, err := caller.fn(sys, user, img64, mime)
			elapsed := time.Since(start)

			r := ModelResult{
				ModelName: caller.name,
				RawJSON:   raw,
				Elapsed:   elapsed,
			}
			if err != nil {
				r.Error = err.Error()
				fmt.Printf("[OCR]   ✗ %-8s 调用失败: %v\n", caller.name, err)
			} else {
				if p, pErr := parseOCRResult(raw); pErr == nil {
					r.Parsed = p
				} else {
					r.Error = pErr.Error()
				}
			}
			results[idx] = r
		}(i, c)
	}
	wg.Wait()
	return results
}

// =============================================================================
// JSON 解析
// =============================================================================

// cleanJSON 清洗模型原始输出，提取纯 JSON
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)

	// 剥离 Qwen <think> 块
	if idx := strings.Index(s, "<think>"); idx != -1 {
		s = strings.TrimSpace(s[idx+len("<think>"):])
	}

	// 优先从 ```json ... ``` 或 ``` ... ``` 代码块中提取
	if start := strings.Index(s, "```json"); start != -1 {
		after := s[start+7:]
		if end := strings.Index(after, "```"); end != -1 {
			return strings.TrimSpace(after[:end])
		}
	}
	if start := strings.Index(s, "```"); start != -1 {
		after := strings.TrimLeft(s[start+3:], "\r\n")
		if end := strings.Index(after, "```"); end != -1 {
			return strings.TrimSpace(after[:end])
		}
	}

	// 兜底：定位第一个 { 或 [，截取到最后一个匹配的 } 或 ]
	if start := strings.IndexAny(s, "{["); start != -1 {
		s = s[start:]
		for i := len(s); i > 0; i-- {
			if s[i-1] == '}' || s[i-1] == ']' {
				return strings.TrimSpace(s[:i])
			}
		}
	}

	return strings.TrimSpace(s)
}

// parseOCRResult 解析单模型返回的 JSON，统一矩阵/普通格式
func parseOCRResult(raw string) (ParsedResult, error) {
	// 尝试矩阵格式（page_type="matrix"）
	var m struct {
		PageType  string         `json:"page_type"`
		TotalRows int            `json:"total_rows"`
		TotalCols int            `json:"total_cols"`
		ColLabels []string       `json:"col_labels"`
		Rows      []MatrixRowOCR `json:"rows"`
		PageNote  string         `json:"page_note,omitempty"`
	}
	if err := json.Unmarshal([]byte(raw), &m); err == nil && m.PageType == "matrix" {
		return ParsedResult{
			PageType:  "matrix",
			Rows:      m.Rows,
			ColLabels: m.ColLabels,
			TotalRows: m.TotalRows,
			TotalCols: m.TotalCols,
			PageNote:  m.PageNote,
		}, nil
	}

	// 普通格式
	var n struct {
		Interactions []OCRInteraction `json:"interactions"`
		PageNote     string           `json:"page_note,omitempty"`
	}
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		return ParsedResult{}, fmt.Errorf("JSON 解析失败: %w", err)
	}
	return ParsedResult{PageType: "normal", Items: n.Interactions, PageNote: n.PageNote}, nil
}

// =============================================================================
// Prompt 构建（从 tests/test_ocr.go 迁移）
// =============================================================================

func buildOCRUserPrompt(interactions []interactionItem) string {
	var sb strings.Builder

	if len(interactions) > 0 {
		sb.WriteString(fmt.Sprintf("【参考题目清单】共 %d 题，仅供参考，如截图中的题目与此不同，优先以截图为准。\n\n", len(interactions)))
		for i, item := range interactions {
			sb.WriteString(fmt.Sprintf("题目%d:\n  题干: %s\n  题型: %s\n\n", i+1, item.Subject, item.QuestionType))
		}
	}

	sb.WriteString(`【识别要求】
扫描截图中的所有题目，逐题识别用户选中的答案。不要因为题目不在清单中就跳过。

【按钮式多选题识别增强】（重点！极易漏识别）
- 按钮式多选题的特征：多个选项横向或纵向排列，每个都是一个独立按钮/色块。
- 选中状态判断：按钮有深色/绿色/蓝色填充背景，或边框颜色明显不同于未选中按钮。
- 未选中状态：按钮为白色/浅色背景，视觉上与选中按钮差异明显。
- ⚠️ 扫描策略（强制执行）：
  1. 从左到右（或从上到下）依次检查每一个按钮。
  2. 遇到第一个高亮按钮后，继续扫描下一个按钮，不能停止。
  3. 必须遍历完所有按钮后才能输出结果，严禁在找到第一个选中项后停止扫描。
  4. 按钮横向排列一行时，尤其容易只报告最左/最右一个被选中的，需特别警惕。
- 典型错误：实际选中了 [A][B][C] 三个按钮，但模型只报告 ["A"]。
- 颜色阈值：如果无法确定一个按钮是否被选中，按"选中"处理，宁可多选不可漏选。

【Likert 量表识别规则】（严禁违反）
- Likert 量表（如 Strongly Disagree / Disagree / Agree / Strongly Agree，或 Never / Rarely / Sometimes / Often / Always 等）属于普通单选题，只要截图中能看到选项文字且能判断哪个被选中，就必须输出 user_selected 为该选项的标签文字（如 "Strongly Agree"），而不是 null。
- 严禁在 Likert 量表上输出 null，除非截图中该题选项完全不可见。
- "likert_card" 特指下拉选择或卡片翻转题，其选项在截图中完全不可见，这种情况下才返回 null。

【陷阱题识别规则】（严禁违反）
- 识别包含以下关键词的题目为陷阱题：
  "For data quality purposes", "please select", "attention check", "quality check",
  "select strongly agree", "select somewhat agree", "select agree", "choose agree",
  "must select", "make sure you select"
- 陷阱题与普通题相同，必须以视觉圆点为准，忠实记录用户的实际勾选。
- 严禁用题干语义覆盖用户的真实选择。
- 在 note 中标记 "attention_check_detected"，并额外记录题干要求的选项，
  例如：note = "attention_check_detected | expected: Somewhat Agree"
- 视觉圆点优先级：视觉圆点 > 语义指令。此为强制规则，不可绕过。

【极性反转题识别规则】（严禁违反）
- 如果题干是负向感受陈述（如 creeps me out / bores me / annoys me / disgusts me / embarrasses me / upsets me / irritates me / makes me uncomfortable 等），选 "Agree" / "Strongly Agree" 表示用户认同该负面感受（即：我确实有此感受）；选 "Disagree" / "Strongly Disagree" 表示用户否认该负面感受（即：我没有此感受）。
- 严禁将负向题中的 "Agree" 理解为"不认可题干"，必须结合语义和圆点位置综合判断。
- 严禁在负向题上因语义歧义而输出 null 或跳题。

【相邻列识别规则】（严禁违反）
- Likert 四列场景（Strongly Disagree / Somewhat Disagree / Somewhat Agree / Strongly Agree），Somewhat Agree 和 Somewhat Disagree 为左右相邻列，最容易误判。
- 严禁在相邻列之间模棱两可就随便选一个，必须明确判断圆点位于中左（Somewhat Disagree）还是中右（Somewhat Agree）。
- 如果无法明确判断，必须输出 note 标记 "adjacent_col_uncertain"，严禁不写备注就随意填写。

【输出示例】
假设截图中有以下题目：
  题干: "I love watching action movies"
  选项: [Never] [Rarely] [Sometimes] [Often] [Always]
  用户选中了 "Often"

正确输出：
{
  "interactions": [
    {
      "subject": "I love watching action movies",
      "question_type": "single",
      "user_selected": "Often",
      "note": ""
    }
  ],
  "page_note": ""
}

请输出 JSON：
{
  "interactions": [
    {
      "subject": "截图中的题干原文",
      "question_type": "single/multi/text 等",
      "user_selected": "用户选择（单选string/多选array/文本string/null）",
      "note": "备注"
    }
  ],
  "page_note": "全页备注"
}

【按钮式多选题输出示例】
假设截图中有以下按钮式多选题：
  题干: "What is your racial background?"
  选项横向排列: [White/Caucasian] [Asian/Asian-American] [Black/African-American] [Other]
  White/Caucasian 和 Asian/Asian-American 两个按钮均为绿色高亮背景（选中态）

正确输出：
{
  "interactions": [
    {
      "subject": "What is your racial background?",
      "question_type": "multi",
      "user_selected": ["White/Caucasian", "Asian/Asian-American"],
      "note": ""
    }
  ],
  "page_note": ""
}

⚠️ 常见错误示例（严禁这样做）：
{
  "user_selected": ["White/Caucasian"]   // ❌ 漏选了 Asian/Asian-American
}
{
  "user_selected": "White/Caucasian"       // ❌ 多选题必须用数组，不是字符串
}`)
	return sb.String()
}

func buildMatrixUserPrompt(interactions []interactionItem) string {
	var sb strings.Builder

	if len(interactions) > 0 {
		sb.WriteString(fmt.Sprintf("【参考题目清单】共 %d 项，优先识别为矩阵题。\n", len(interactions)))
		for i, item := range interactions {
			sb.WriteString(fmt.Sprintf("行%d: %s\n", i+1, item.Subject))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(`【识别任务】
请先判断截图是哪种结构：

A. 矩阵题（表格式）：表格左侧是行（子题题干），上方有统一列标签（如 Yes/No，或 Likert 选项）
B. 普通题（非矩阵）：各题目独立，无统一列标签，可能是单选、多选、文本等

请输出纯 JSON：
- 如果是矩阵题（判断依据：能识别出 2 列及以上且列标签适用于所有行）：
{
  "page_type": "matrix",
  "total_rows": <总行数>,
  "total_cols": <总列数>,
  "col_labels": ["列1标签", "列2标签", ...],
  "rows": [
    {"row_index": 1, "row_text": "子题题干原文", "selected_col": <列号整数或null>, "note": "备注"},
    {"row_index": 2, "row_text": "子题题干原文", "selected_col": <列号整数或null>, "note": "备注"}
  ]
}

- 如果不是矩阵题（无法识别出统一列标签，或列标签不适用于所有行）：
{
  "page_type": "normal",
  "interactions": [
    {"subject": "题干原文", "question_type": "single/multi/text", "user_selected": "用户选择", "note": "备注"},
    ...
  ],
  "page_note": ""
}

【陷阱题识别规则】（矩阵题适用）（严禁违反）
- 识别包含以下关键词的题目为陷阱题：
  "For data quality purposes", "please select", "attention check", "quality check",
  "select strongly agree", "select somewhat agree", "select agree", "choose agree",
  "must select", "make sure you select"
- 陷阱题与普通题相同，必须以视觉圆点为准，忠实记录用户的实际勾选。
- 严禁用题干语义覆盖用户的真实选择。
- 在 note 中标记 "attention_check_detected"，并额外记录题干要求的选项，
  例如：note = "attention_check_detected | expected: Somewhat Agree"
- 视觉圆点优先级：视觉圆点 > 语义指令。此为强制规则，不可绕过。

【极性反转题识别规则】（矩阵题适用）（严禁违反）
- 如果某行题干是负向感受陈述（如 creeps me out / bores me / annoys me / disgusts me / embarrasses me / upsets me / irritates me / makes me uncomfortable 等），选 "Agree" / "Strongly Agree" 表示用户认同该负面感受（即：我确实有此感受）；选 "Disagree" / "Strongly Disagree" 表示用户否认该负面感受（即：我没有此感受）。
- 严禁将负向题中的 "Agree" 理解为"不认可题干"，必须结合语义和圆点位置综合判断。
- 严禁在负向题上因语义歧义而输出 null 或跳题。

【相邻列识别规则】（矩阵题适用）（严禁违反）
- Likert 四列场景（Strongly Disagree / Somewhat Disagree / Somewhat Agree / Strongly Agree），Somewhat Agree 和 Somewhat Disagree 为左右相邻列，最容易误判。
- 严禁在相邻列之间模棱两可就随便选一个，必须明确判断圆点位于中左（Somewhat Disagree）还是中右（Somewhat Agree）。
- 如果无法明确判断，必须输出 note 标记 "adjacent_col_uncertain"，严禁不写备注就随意填写。

【输出要求】
- 必须输出纯 JSON，禁止包含 Markdown 代码块。
- 仔细扫描每一行，selected_col / user_selected 严禁留空或跳过任何一行。
`)
	return sb.String()
}

// =============================================================================
// 投票机制
// =============================================================================

var attentionKeywords = []string{
	"for data quality purposes", "please select", "attention check",
	"quality check", "select strongly agree", "select somewhat agree",
	"select agree", "choose agree", "must select", "make sure you select",
}

func isAttentionCheck(subject string) bool {
	low := strings.ToLower(subject)
	for _, kw := range attentionKeywords {
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}

// normalizeAns 将答案归一化为小写字符串，用于比较
func normalizeAns(v interface{}) string {
	switch val := v.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(val))
	case []interface{}:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				parts = append(parts, strings.ToLower(strings.TrimSpace(s)))
			}
		}
		return strings.Join(parts, ",")
	case float64:
		return fmt.Sprintf("%.0f", val)
	case int:
		return fmt.Sprintf("%d", val)
	case nil:
		return "null"
	default:
		return strings.ToLower(fmt.Sprintf("%v", val))
	}
}

// displayAns 将答案格式化为可读字符串（保留大小写）
func displayAns(v interface{}) string {
	switch val := v.(type) {
	case string:
		if val == "" {
			return "null（未识别）"
		}
		return val
	case []interface{}:
		parts := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ", ")
	case float64:
		return fmt.Sprintf("col %.0f", val)
	case int:
		return fmt.Sprintf("col %d", val)
	case nil:
		return "null（未识别）"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// voteAnswers 投票裁决，返回 (最终答案, 状态, 备注)
func voteAnswers(answers map[string]interface{}) (interface{}, VoteStatus, string) {
	if len(answers) == 0 {
		return nil, VoteUncertain, "无模型结果"
	}
	// 单模型直接 confirmed
	if len(answers) == 1 {
		for _, v := range answers {
			return v, VoteConfirmed, ""
		}
	}

	// 统计归一化票数
	counts := map[string]int{}
	rawOf := map[string]interface{}{}
	for _, v := range answers {
		norm := normalizeAns(v)
		counts[norm]++
		rawOf[norm] = v
	}

	total := len(answers)

	// 全票一致
	if len(counts) == 1 {
		for norm := range counts {
			return rawOf[norm], VoteConfirmed, ""
		}
	}

	// 找最高票
	best, bestCount := "", 0
	for norm, cnt := range counts {
		if cnt > bestCount {
			bestCount = cnt
			best = norm
		}
	}

	// 各模型详情（用于 note）
	parts := make([]string, 0, len(answers))
	for name, v := range answers {
		parts = append(parts, fmt.Sprintf("%s=%s", name, displayAns(v)))
	}
	detail := strings.Join(parts, " / ")

	if bestCount*2 > total {
		return rawOf[best], VoteMajority, "多数裁决: " + detail
	}
	return nil, VoteUncertain, "需人工复核: " + detail
}

// runVoting 单轮投票
func runVoting(results []ModelResult) []VotedItem {
	var ok []ModelResult
	for _, r := range results {
		if r.Error == "" {
			ok = append(ok, r)
		}
	}
	if len(ok) == 0 {
		return nil
	}

	isMatrix := ok[0].Parsed.PageType == "matrix"
	colLabels := ok[0].Parsed.ColLabels

	// 取各模型中最大题目数
	total := 0
	for _, r := range ok {
		n := len(r.Parsed.Items)
		if isMatrix {
			n = len(r.Parsed.Rows)
		}
		if n > total {
			total = n
		}
	}

	items := make([]VotedItem, total)
	for i := 0; i < total; i++ {
		answers := map[string]interface{}{}
		subject, qType := "", ""

		if isMatrix {
			for _, r := range ok {
				if i < len(r.Parsed.Rows) {
					row := r.Parsed.Rows[i]
					answers[r.ModelName] = row.SelectedCol
					if subject == "" {
						subject = row.RowText
					}
				}
			}
			qType = "matrix_row"
		} else {
			for _, r := range ok {
				if i < len(r.Parsed.Items) {
					it := r.Parsed.Items[i]
					answers[r.ModelName] = it.UserSelected
					if subject == "" {
						subject = it.Subject
						qType = it.QuestionType
					}
				}
			}
		}

		// 陷阱题：跳过投票，忠实记录各模型原始结果
		if isAttentionCheck(subject) {
			noteParts := []string{"attention_check_detected"}
			for name, v := range answers {
				noteParts = append(noteParts, fmt.Sprintf("%s=%s", name, displayAns(v)))
			}
			items[i] = VotedItem{
				Index:        i + 1,
				Subject:      subject,
				QuestionType: qType,
				FinalAnswer:  answers[ok[0].ModelName],
				Status:       VoteConfirmed,
				ModelAnswers: answers,
				Note:         strings.Join(noteParts, " | "),
				ColLabels:    colLabels,
			}
			continue
		}

		finalAns, status, note := voteAnswers(answers)
		items[i] = VotedItem{
			Index:        i + 1,
			Subject:      subject,
			QuestionType: qType,
			FinalAnswer:  finalAns,
			Status:       status,
			ModelAnswers: answers,
			Note:         note,
			ColLabels:    colLabels,
		}
	}

	return items
}

// getAnswerAtIdx 从 ModelResult 中提取第 idx 题答案（0-based）
func getAnswerAtIdx(r ModelResult, idx int) interface{} {
	if r.Error != "" {
		return nil
	}
	if r.Parsed.PageType == "matrix" {
		if idx < len(r.Parsed.Rows) {
			return r.Parsed.Rows[idx].SelectedCol
		}
	} else {
		if idx < len(r.Parsed.Items) {
			return r.Parsed.Items[idx].UserSelected
		}
	}
	return nil
}

// runVotingWithRetry 投票 + 重试机制（最多 maxRetry 轮）
func runVotingWithRetry(sys, user, img64, mime string, initial []ModelResult, maxRetry int) []VotedItem {
	voted := runVoting(initial)
	if voted == nil {
		return nil
	}

	for retry := 0; retry < maxRetry; retry++ {
		var uncertain []int
		for i, item := range voted {
			if item.Status == VoteUncertain {
				uncertain = append(uncertain, i)
			}
		}
		if len(uncertain) == 0 {
			break
		}

		fmt.Printf("[OCR]   [重试 %d/%d] %d 题存疑，重新调用所有模型...\n", retry+1, maxRetry, len(uncertain))

		newResults := callAllVisionModels(sys, user, img64, mime)
		resolved := 0

		for _, idx := range uncertain {
			newAns := map[string]interface{}{}
			for _, r := range newResults {
				if r.Error == "" {
					newAns[r.ModelName] = getAnswerAtIdx(r, idx)
				}
			}
			finalAns, status, note := voteAnswers(newAns)
			if status != VoteUncertain {
				voted[idx].FinalAnswer = finalAns
				voted[idx].Status = status
				voted[idx].Note = joinNote(voted[idx].Note, fmt.Sprintf("重试%d解决 | %s", retry+1, note))
				for k, v := range newAns {
					voted[idx].ModelAnswers[k] = v
				}
				resolved++
			}
		}
		fmt.Printf("[OCR]   [重试 %d] 解决 %d/%d 题\n", retry+1, resolved, len(uncertain))
	}

	// 仍 uncertain → human_review_required
	for i := range voted {
		if voted[i].Status == VoteUncertain {
			voted[i].Status = VoteHumanReview
		}
	}
	return voted
}

func joinNote(a, b string) string {
	if a == "" {
		return b
	}
	return a + " | " + b
}

// =============================================================================
// 步骤2：分析比对（使用投票后的结果）
// =============================================================================

// runAnalysisStepFromVoted 将投票后的最终答案与推荐答案比对
func runAnalysisStepFromVoted(votedItems []VotedItem, origInteractions []recommendInteraction, isMatrix bool, matrixForAnalysis *matrixUserSelected, surveyID, basePath string) analysisStep {
	if isMatrix {
		return runMatrixAnalysisStepFromVoted(votedItems, origInteractions, matrixForAnalysis)
	}

	result := analysisStep{
		Interactions: make([]analysisInteraction, 0, len(origInteractions)),
	}

	for i, orig := range origInteractions {
		item := analysisInteraction{
			Subject:            orig.Subject,
			QuestionType:       orig.QuestionType,
			RecommendedAnswers: orig.RecommendedAnswers,
			UserSelected:       nil,
			MatchMethod:        "",
		}

		// 从投票结果取最终答案
		if i < len(votedItems) {
			item.UserSelected = votedItems[i].FinalAnswer
		}

		item.FollowedRecommendation, item.MatchMethod = compareAnswers(
			orig.QuestionType,
			item.UserSelected,
			orig.RecommendedAnswers,
			surveyID,
			basePath,
		)

		result.Interactions = append(result.Interactions, item)
	}

	result.Overall = calcOverall(result.Interactions)
	return result
}

// runMatrixAnalysisStepFromVoted 矩阵页分析：逐行精确匹配
func runMatrixAnalysisStepFromVoted(votedItems []VotedItem, origInteractions []recommendInteraction, matrixForAnalysis *matrixUserSelected) analysisStep {
	result := analysisStep{
		Interactions: make([]analysisInteraction, 0, len(origInteractions)),
	}

	if matrixForAnalysis == nil || len(matrixForAnalysis.Rows) == 0 {
		result.Overall = "unclear"
		return result
	}

	// 建立 normalized(row_text) → matrixRow 查找表
	rowMap := make(map[string]matrixRow)
	for _, r := range matrixForAnalysis.Rows {
		rowMap[normalizeAnswer(r.RowText)] = r
	}

	for _, orig := range origInteractions {
		item := analysisInteraction{
			Subject:            orig.Subject,
			QuestionType:       "matrix_row",
			RecommendedAnswers: orig.RecommendedAnswers,
			UserSelected:       nil,
		}

		row, found := rowMap[normalizeAnswer(orig.Subject)]
		if !found || row.SelectedCol == nil {
			item.FollowedRecommendation = nil
			item.MatchMethod = "ocr_not_found"
			result.Interactions = append(result.Interactions, item)
			continue
		}

		colIdx := *row.SelectedCol - 1
		if colIdx < 0 || colIdx >= len(matrixForAnalysis.ColLabels) {
			item.FollowedRecommendation = nil
			item.MatchMethod = "col_index_out_of_range"
			result.Interactions = append(result.Interactions, item)
			continue
		}

		userColLabel := matrixForAnalysis.ColLabels[colIdx]
		item.UserSelected = userColLabel
		matched := normalizeAnswer(userColLabel) == normalizeAnswer(toStringValue(orig.RecommendedAnswers))
		item.FollowedRecommendation = &matched
		item.MatchMethod = "matrix_col_exact"
		result.Interactions = append(result.Interactions, item)
	}

	result.Overall = calcOverall(result.Interactions)
	return result
}

// extractMatrixUserSelected 从 interface{} 提取 matrixUserSelected
func extractMatrixUserSelected(v interface{}) (matrixUserSelected, bool) {
	if m, ok := v.(matrixUserSelected); ok {
		return m, true
	}
	b, err := json.Marshal(v)
	if err != nil {
		return matrixUserSelected{}, false
	}
	var m matrixUserSelected
	if err := json.Unmarshal(b, &m); err != nil {
		return matrixUserSelected{}, false
	}
	return m, true
}

// compareAnswers 根据题型选择比对策略（非矩阵题用）
func compareAnswers(qType string, userSelected, recommended interface{}, surveyID, basePath string) (*bool, string) {
	if qType == "likert_card" {
		return nil, "skipped_card"
	}

	if userSelected == nil {
		return nil, "ocr_not_found"
	}

	if closedQuestionTypes[qType] {
		result := compareClosedQuestion(userSelected, recommended)
		return &result, "code_exact"
	}

	userStr := toStringValue(userSelected)
	recStr := toStringValue(recommended)

	if userStr == "" || recStr == "" {
		return nil, "empty_value"
	}

	sim := charSimilarity(userStr, recStr)
	fmt.Printf("[OCR分析] 开放题字符相似度: %.2f (threshold=0.85)\n", sim)

	if sim >= 0.85 {
		result := true
		return &result, "char_similarity"
	}
	if sim <= 0.15 {
		result := false
		return &result, "char_similarity"
	}

	followed, err := deepSeekTextCompare(userStr, recStr, surveyID, basePath)
	if err != nil {
		fmt.Printf("[OCR分析] DeepSeek 比对失败: %v，返回 null\n", err)
		return nil, "ai_text_error"
	}
	return followed, "ai_text"
}

// compareClosedQuestion 封闭题代码比对（归一化后精确匹配）
func compareClosedQuestion(userSelected, recommended interface{}) bool {
	userSet := toNormalizedSet(userSelected)
	recSet := toNormalizedSet(recommended)

	if len(userSet) == 0 || len(recSet) == 0 {
		return false
	}
	if len(userSet) != len(recSet) {
		return false
	}
	for k := range userSet {
		if !recSet[k] {
			return false
		}
	}
	return true
}

// toNormalizedSet 将 string 或 []string 转成归一化词集合
func toNormalizedSet(v interface{}) map[string]bool {
	result := make(map[string]bool)
	switch val := v.(type) {
	case string:
		n := normalizeAnswer(val)
		if n != "" {
			result[n] = true
		}
	case []interface{}:
		for _, item := range val {
			if s, ok := item.(string); ok {
				n := normalizeAnswer(s)
				if n != "" {
					result[n] = true
				}
			}
		}
	case []string:
		for _, s := range val {
			n := normalizeAnswer(s)
			if n != "" {
				result[n] = true
			}
		}
	}
	return result
}

var (
	optionPrefixRegex = regexp.MustCompile(`^[\\(（]?[a-zA-Z0-9]{1,2}[\\)）\.、]\s*`)
	parenSuffixRegex  = regexp.MustCompile(`\s*[\(（][^)）]{1,50}[\)）]\s*$`)
)

// normalizeAnswer 答案文本归一化：小写、去前缀编号、去括号补充说明
func normalizeAnswer(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = optionPrefixRegex.ReplaceAllString(s, "")
	s = parenSuffixRegex.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	return s
}

// toStringValue 将 interface{} 转成字符串（用于开放题比对）
func toStringValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case []interface{}:
		var parts []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		return strings.Join(parts, " | ")
	}
	return ""
}

// charSimilarity 字符级相似度（基于公共字符频次，范围 0.0~1.0）
func charSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == b {
		return 1.0
	}

	freqA := make(map[rune]int)
	for _, r := range a {
		freqA[r]++
	}
	common := 0
	for _, r := range b {
		if freqA[r] > 0 {
			common++
			freqA[r]--
		}
	}

	lenA := utf8.RuneCountInString(a)
	lenB := utf8.RuneCountInString(b)
	if lenA == 0 || lenB == 0 {
		return 0.0
	}
	return float64(2*common) / float64(lenA+lenB)
}

// =============================================================================
// DeepSeek 开放题语义比对
// =============================================================================

const openTextCompareSystem = `你是答案比对助手。判断"用户填写"和"AI推荐答案"是否表达了相同或高度相近的意思。
只返回 JSON，不要解释：
{"followed": true}   → 意思相同或高度相近
{"followed": false}  → 意思明显不同
{"followed": null}   → 无法判断（开放性观点类问题，无客观对错）`

// deepSeekTextCompare 调用 DeepSeek 判断开放题语义一致性
func deepSeekTextCompare(userText, recommendText, surveyID, basePath string) (*bool, error) {
	userPrompt := fmt.Sprintf("AI推荐答案: %q\n用户实际填写: %q", recommendText, userText)

	rawResp, err := CallAIWithValidation(surveyID, openTextCompareSystem, userPrompt, basePath, true)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek 调用失败: %w", err)
	}

	var parsed struct {
		Followed *bool `json:"followed"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(rawResp)), &parsed); err != nil {
		return nil, fmt.Errorf("解析 DeepSeek 比对结果失败: %w | raw: %s", err, rawResp[:min(100, len(rawResp))])
	}
	return parsed.Followed, nil
}

// =============================================================================
// Overall 综合判断
// =============================================================================

// calcOverall 根据各题 followed_recommendation 综合计算 overall
func calcOverall(items []analysisInteraction) string {
	trueCount := 0
	falseCount := 0

	for _, item := range items {
		if item.FollowedRecommendation == nil {
			continue
		}
		if *item.FollowedRecommendation {
			trueCount++
		} else {
			falseCount++
		}
	}

	valid := trueCount + falseCount
	if valid == 0 {
		return "unclear"
	}
	if falseCount > 0 {
		return "mismatch"
	}
	return "match"
}

// =============================================================================
// AI 事件日志：投票过程记录
// =============================================================================

// writeOCRVotingLog 将投票过程写入 AI 事件日志（storage/logs/events/sys_YYYYMMDD.log）
func writeOCRVotingLog(surveyID string, results []ModelResult, votedItems []VotedItem) {
	// 构造日志数据
	logData := map[string]interface{}{
		"ocr_type": "multi_model_voting",
		"models":   results,
	}

	if votedItems != nil {
		// 汇总统计
		statusCounts := map[string]int{}
		for _, item := range votedItems {
			statusCounts[string(item.Status)]++
		}
		logData["voting_summary"] = map[string]interface{}{
			"total":           len(votedItems),
			"status_counts":   statusCounts,
			"items":           votedItems,
		}
	}

	// 写入事件日志（复用 aiCaller.go 中的事件日志机制）
	writeEventLog("screenshot_ocr_voting", surveyID, logData)
}

// writeEventLog 写入 AI 事件日志（storage/logs/events/sys_YYYYMMDDHH.log）
// 兼容 ws_server/main.go 的 writeAILog 格式：{timestamp}|{event_type}|{session_id}|{compact_json}
func writeEventLog(eventType, sessionID string, data map[string]interface{}) {
	// 事件日志写入 storage/logs/events/
	eventLogDir := "storage/logs/events"
	if err := os.MkdirAll(eventLogDir, 0755); err != nil {
		fmt.Printf("[OCR] 写事件日志失败: mkdir %s: %v\n", eventLogDir, err)
		return
	}

	logFileName := fmt.Sprintf("sys_%s.log", time.Now().Format("20060102150405"))
	logFilePath := fmt.Sprintf("%s/%s", eventLogDir, logFileName)

	dataBytes, _ := json.Marshal(data)
	compactData := strings.ReplaceAll(string(dataBytes), "\n", "")
	compactData = strings.ReplaceAll(compactData, "\r", "")
	compactData = strings.Join(strings.Fields(compactData), "")

	logLine := fmt.Sprintf("%s|%s|%s|%s\n",
		time.Now().Format("2006-01-02 15:04:05.000"),
		eventType,
		sessionID,
		compactData,
	)

	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[OCR] 写事件日志失败: open %s: %v\n", logFilePath, err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(logLine); err != nil {
		fmt.Printf("[OCR] 写事件日志失败: write: %v\n", err)
		return
	}
	f.Sync()

	fmt.Printf("[OCR] 事件日志已写入: %s\n", logFilePath)
}

// =============================================================================
// Qwen VL API 调用（保留原有实现，复用 volcesVisionCfg）
// =============================================================================

type qwenVLMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type qwenVLReq struct {
	Model       string          `json:"model"`
	Messages    []qwenVLMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
	MaxTokens   int             `json:"max_tokens"`
}

type qwenVLResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// callQwenVLWithPrompt 调用 Qwen VL，最多重试 3 次
func callQwenVLWithPrompt(cfg *volcesVisionCfg, systemPrompt, userPrompt, imageBase64 string) (string, error) {
	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		start := time.Now()
		content, statusCode, err := doQwenVLRequest(cfg, systemPrompt, userPrompt, imageBase64)
		elapsed := time.Since(start)

		if err != nil {
			lastErr = err
			time.Sleep(1 * time.Second)
			continue
		}
		if statusCode == 401 {
			return "", fmt.Errorf("Qwen VL API 认证失败 (401)")
		}
		if statusCode == 429 {
			lastErr = fmt.Errorf("Qwen VL API 限流 (429)")
			time.Sleep(5 * time.Second)
			continue
		}
		if statusCode >= 500 {
			lastErr = fmt.Errorf("Qwen VL API 服务器错误 (%d)", statusCode)
			time.Sleep(2 * time.Second)
			continue
		}
		if statusCode != 200 {
			return "", fmt.Errorf("Qwen VL API HTTP %d: %s", statusCode, content)
		}

		cleaned := strings.TrimSpace(cleanJSON(content))
		fmt.Printf("[OCR] Qwen VL 响应长度: %d chars, 耗时: %.1fs (attempt %d/%d)\n",
			len(cleaned), elapsed.Seconds(), attempt+1, maxRetries)
		return cleaned, nil
	}

	return "", fmt.Errorf("Qwen VL 调用失败，已重试 %d 次: %w", maxRetries, lastErr)
}

// doQwenVLRequest 单次 HTTP 请求
func doQwenVLRequest(cfg *volcesVisionCfg, systemPrompt, userPrompt, imageBase64 string) (string, int, error) {
	imageURL := fmt.Sprintf("data:image/jpeg;base64,%s", imageBase64)

	reqBody := qwenVLReq{
		Model: cfg.Model,
		Messages: []qwenVLMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role: "user",
				Content: []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": userPrompt,
					},
					map[string]interface{}{
						"type": "image_url",
						"image_url": map[string]string{
							"url": imageURL,
						},
					},
				},
			},
		},
		Temperature: cfg.Temperature,
		MaxTokens:   2048,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("marshal qwen request: %w", err)
	}

	timeout := time.Duration(cfg.Timeout) * time.Second
	client := &http.Client{Timeout: timeout}

	url := cfg.BaseURL + "/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", 0, fmt.Errorf("create qwen request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("qwen HTTP: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, fmt.Errorf("read qwen response: %w", err)
	}
	if resp.StatusCode != 200 {
		return string(respBytes), resp.StatusCode, nil
	}

	var result qwenVLResp
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", resp.StatusCode, fmt.Errorf("parse qwen JSON: %w", err)
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return "", resp.StatusCode, fmt.Errorf("Qwen VL 返回为空")
	}

	return result.Choices[0].Message.Content, resp.StatusCode, nil
}
