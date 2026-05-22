/**
 * ============================================================================
 * YAML 题目结构提取器 — YamlStructureExtractor
 * ============================================================================
 *
 * 【职责】
 *   从问卷页面 HTML 中提取题目结构，输出标准 YAML 格式。
 *   结果保存到 data/cache/{survey_id}/yamlTemplate/{pageId}.yaml，
 *   作为后续 analyze 调用 AnswerRecommender 时的结构模板，
 *   减少 AI 对题型和输出格式的判断错误。
 *
 * 【调用时机】
 *   - 仅在 yamlTemplate 文件不存在时（首次访问该页面）触发
 *   - 串行：YAML 提取完成后再调用 AnswerRecommender
 *   - 失败时静默（不抛出），主流程继续无模板模式
 *
 * 【使用方式】
 *   const extractor = new YamlStructureExtractor(decoratedClient);
 *   const yamlStr = await extractor.extract({ cleanHtml, sessionId });
 * ============================================================================
 */

// ── YAML 结构提取提示词（固定，不随 prompts.yaml 变动）──────────────────────
const YAML_SYSTEM_PROMPT = `你是一个问卷结构解析助手。我会给你一段海外调查问卷的 HTML 源码文件，该文件可能包含多个题目（SPA 多题页面）。请严格从文件内容中提取所有题目信息，按题号顺序输出为 YAML 格式的题目列表。

所有内容必须 100% 来源于文件，不得添加、修改、推断或省略任何信息。`;

const YAML_USER_PROMPT_PREFIX = `---

**多题页面处理规则**

一个 HTML 文件中可能包含多道题目，每道题对应一个独立的 \`<fieldset>\` 或 \`<div class="sg-question">\` 块。请逐题扫描，所有题目统一收录至顶层 \`questions\` 列表中，按 HTML 中出现的顺序排列，示例结构如下：

\`\`\`yaml
questions:
  - id: 27
    number: "16"
    title: "..."
    type: single
    ...
  - id: 109
    number: "18"
    title: "..."
    type: multi
    ...
\`\`\`

---

**题型识别规则**

type 字段的值必须且只能从以下 32 个枚举值中选取，不得自造任何其他值：

\`\`\`
选择类：
  RADIO / radio / SINGLE                    → single
  CHECKBOX / checkbox / MULTIPLE            → multi
  SELECT / select                           → select

文本输入类：
  input[type=text] / INPUT / TEXT / FILL    → open_text
  input[type=email] / EMAIL                 → email
  input[type=tel] / PHONE                   → phone
  input[type=number] / NUMBER               → number
  input[type=date] / DATE                   → date
  input[type=time] / TIME                   → time
  textarea / OPEN_END                       → open_end

矩阵类：
  TABLE+RADIO / MATRIX                      → matrix_radio
  TABLE+CHECKBOX / MATRIX_MULTI             → matrix_checkbox
  LIKERT                                    → likert
  SEMANTIC_DIFF                             → semantic_diff

滑动类：
  SLIDER                                    → slider
  NPS                                       → nps
  RATING / LIKERT_CARD                      → likert_card

排序类：
  MULTI_SELECT（含 xnumber）               → multi_select
  RANKING_DRAG                              → ranking_drag
  RANKING_SELECT                            → ranking_select
  MAX_DIFF                                  → max_diff
  CONJOINT                                  → conjoint

分类类：
  CARDSORT / CARDSORT_DRAG                  → cardsort_drag
  CARDSORT_SELECT                           → cardsort_select

特殊类：
  IMAGE_SELECT                              → image_select
  HEATMAP                                   → heatmap

非交互 / 混合：
  MIXED                                     → mixed
  NAV_BUTTON / 过渡说明页                   → instruction
  无法识别                                  → unknown
\`\`\`

识别优先级：优先以 JSON 中的 type 字段为准；JSON 无法判断时，以 HTML 控件类型为辅助依据；两者均无法确定时，输出 \`unknown\`。

---

**字段提取规则**

每道题提取以下字段：

- \`id\` —— JSON questions 对象的顶层 key
- \`number\` —— HTML \`<span class="sg-question-number">\` 的文字
- \`title\` —— HTML \`<legend>\` 或题干容器内的完整文字（去除题号与必填星号）
- \`type\` —— 严格从上方 32 个枚举值中选取
- \`required\` —— JSON 中该题的 required 字段（true / false）

**有选项的题目**（single / multi / select / image_select）额外提取：

- \`options\` 列表，每项包含：
    - \`label\` —— 来自 JSON options[id].value 字段
    - 顺序严格按 HTML 中 \`<li>\` 或 \`<option>\` 实际渲染顺序排列

**文本输入题**（open_text / open_end / email / phone / number / date / time）额外提取：

- \`input_type\` —— 来自 HTML input 标签的 type 属性（text / textarea / number / email / tel / date / time）
- \`placeholder\` —— 若 HTML 中存在 placeholder 属性则提取，否则省略

**矩阵题**（matrix_radio / matrix_checkbox / likert / semantic_diff）额外提取：

- \`columns\` —— 来自 HTML \`<thead>\` 中每个 \`<th>\` 的文字，id 来自对应 class（如 col-10154）
- \`rows\` —— 来自 HTML \`<tbody>\` 中每行第一个 \`<th>\` 的文字，id 来自对应 class（如 row-33）
- 顺序严格按 HTML 出现顺序排列

**滑动类题目**（slider / nps）额外提取：

- \`min\` —— 最小值，来自 HTML 或 JSON 中的 min 配置
- \`max\` —— 最大值，来自 HTML 或 JSON 中的 max 配置
- \`min_label\` —— 左端标签文字，若存在则提取
- \`max_label\` —— 右端标签文字，若存在则提取

**response_format 字段**（每题均需输出）：

- \`type\` —— 对应填写：single_select / multi_select / single_per_row / multi_per_row / open_text / open_end / ranking / cardsort 等，与题型保持一致
- \`orientation\` —— 来自 JSON properties.orientation，无则省略
- \`option_sort\` —— 来自 JSON properties.option_sort，无则省略
- \`max_selections\` —— 仅多选题填写，来自 JSON properties.xnumber

---

**严格禁止以下行为：**

- 遗漏页面中任何一道题目
- type 字段使用上方 32 个枚举值以外的任何值
- 凭空添加文件中没有的选项或字段
- 修改任何题干或选项的原始措辞
- 改变选项的排列顺序（以 HTML 渲染顺序为唯一基准）
- 对题型进行主观猜测，必须以文件中的控件类型和 JSON type 字段为唯一依据
- 输出文件中不存在的任何字段

---

直接输出 YAML，不要任何解释文字，不要 markdown 代码块包裹。

HTML 源码如下：
`;

export class YamlStructureExtractor {
  /**
   * @param {object} aiClient - decorated AI client（与 AnswerRecommender 共用）
   */
  constructor(aiClient) {
    this._client = aiClient;
  }

  /**
   * 从 HTML 中提取题目结构，返回 YAML 字符串。
   * 失败时返回空字符串（由调用方决定是否记录）。
   *
   * @param {object} params
   * @param {string} params.cleanHtml    - HtmlExtractor 输出的清洗 HTML
   * @param {string} [params.sessionId] - 用于 AI 日志追踪
   * @param {string} [params.channelId] - AI 渠道（默认 volces/DeepSeek）
   * @returns {Promise<string>} YAML 字符串；失败时返回 ''
   */
  async extract({ cleanHtml = '', sessionId = '', channelId = 'volces' } = {}) {
    if (!cleanHtml) return '';

    const prompt = YAML_USER_PROMPT_PREFIX + cleanHtml.slice(0, 60000);

    let raw;
    try {
      raw = await this._client.infer({
        sessionId,
        prompt,
        systemPrompt: YAML_SYSTEM_PROMPT,
        channelId,
      });
    } catch (err) {
      console.warn('[YamlStructureExtractor] AI 调用失败:', err.message);
      return '';
    }

    const content = raw?.choices?.[0]?.message?.content ?? '';
    return this._cleanYaml(content);
  }

  /**
   * 清理 AI 响应：去除 markdown 代码块包裹，保留纯 YAML 文本。
   * @param {string} raw
   * @returns {string}
   */
  _cleanYaml(raw) {
    return raw
      .replace(/^```(?:yaml)?\s*/i, '')
      .replace(/\s*```\s*$/i, '')
      .trim();
  }
}
