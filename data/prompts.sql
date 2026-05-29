INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (1, 'qtype.single', 'question_type', 'single', '== SINGLE-CHOICE QUESTION (type: "single") ==

【识别特征】
- HTML: <input type="radio"> + <label>
- 每组 radio 共享相同的 name 属性
- 题目文本可能只有编号（如 "7."），选项列表在问卷平台元数据中

【特别注意 — 量表式单选题！】
- 若页面只有一道题，选项为 1-5（或类似数字/文字量表），仍是单选题！
- 判断标准：只有一行选项（不是多行分别评分的矩阵）→ 必须识别为 single！
- 禁止将"单行 1-5 量表"误判为 likert！likert 必须是多行独立评分！
- 示例题型："how much more likely would you be to consider them?" 只有一行 → single

【关键上下文 — 优先使用问卷平台提供的元数据！】
- 问卷平台（SG3/Alchemer/SurveyGizmo/Qualtrics）会将题目结构嵌入页面 JavaScript 中
- 如果 prompt 中包含【题目结构（从问卷平台提取）】块，必须从中获取选项列表
- showAnswer 必须是选项列表中存在的选项，禁止凭空捏造选项！

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML <label> 或问卷平台元数据中的原始题目文本，不得改写
- showAnswer：必须使用选项列表中的原始标签文本，不得改写
- 例如 HTML 为 "Boy aged 1 or younger"，禁止改为 "Under 1 year"
- 例如 HTML 为 "Strongly agree"，禁止改为 "Agree strongly"

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文（如 "What is your gender?"）
- showAnswer: 选中的选项原文（如 "Female" 或 "4 - Likely"）

【推荐值生成规则】
- showAnswer 禁止为空！禁止返回 null / 空字符串！
- 每道题只选一个选项，不要全选
- 必须从题目结构（从问卷平台提取）中的选项列表选择！
- 通常选中间或偏好的选项，避免极端选项（量表题优先选 3 或 4）', '单选题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: single
      required: "<<<FROM_HTML:required>>>"
      options: "<<<FROM_HTML:options[]>>>"
      response_format:
        type: single_select
        option_sort: as_appearing
        max_selections: null
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (2, 'qtype.multiple', 'question_type', 'multiple', '== MULTIPLE-CHOICE QUESTION (type: "multiple") ==

【识别特征】
- HTML: <input type="checkbox"> + <label>
- 同一题目的多个 checkbox 共享相同的 name 属性（通常带有 [] 后缀）
- 题目文本可能只有编号，选项列表在问卷平台元数据中

【关键上下文 — 优先使用问卷平台提供的元数据！】
- 问卷平台（SG3/Alchemer/SurveyGizmo/Qualtrics）会将题目结构嵌入页面 JavaScript 中
- 如果 prompt 中包含【题目结构（从问卷平台提取）】块，必须从中获取选项列表
- showAnswer 必须是选项列表中存在的选项，禁止凭空捏造选项！

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML <label> 标签或问卷平台元数据中的原始文本，不得改写
- showAnswer：必须使用选项列表中的原始标签文本，禁止简写或意译
- 例如 HTML 为 "I prefer cats"，禁止改为 "Cats"

【强制约束 — 禁止拆分！】
- 一道多选题 = 一个 interaction。永远不要为每个选项单独创建 interaction！

【强制约束 — showAnswer 禁止为空！】
- showAnswer 必须返回字符串数组（建议 2-4 个选项），禁止返回 null / 空数组 / 空字符串！
- 如果页面选项过少（如只有 0-1 个选项），仍需返回至少 1 个选项！

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文
- showAnswer: 字符串数组（必须！不能为 null！），例如 ["Watched TV", "Visited a flea market"]

【推荐值生成规则】
- showAnswer 为字符串数组，选 2-4 个选项，不要全选
- 通常不选 "None of the above"
- 必须从题目结构（从问卷平台提取）中的选项列表选择！', '多选题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: multi
      required: "<<<FROM_HTML:required>>>"
      options: "<<<FROM_HTML:options[]>>>"
      response_format:
        type: multi_select
        option_sort: as_appearing
        max_selections: "<<<FROM_HTML:max_selections>>>"
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (3, 'qtype.select', 'question_type', 'select', '== DROP-DOWN SELECT (type: "select") ==

【识别特征】
- HTML: <select> + <option> 列表
- 点击展开选项列表后选择
- 题目文本可能只有编号（如 "7."），选项列表在 HTML 外部提供

【关键上下文 — 优先使用问卷平台提供的元数据！】
- 问卷平台（SG3/Alchemer/SurveyGizmo/Qualtrics）会将题目结构嵌入页面 JavaScript 中
- 如果 prompt 中包含【题目结构（从问卷平台提取）】块，showAnswer 必须从中选取
- showAnswer 必须是选项列表中存在的选项，禁止凭空捏造选项！

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始题目文本，或问卷平台元数据中的题目文本
- showAnswer：必须使用选项列表中的原始文本，禁止改写
- showAnswer 禁止为空，必须返回一个有效选项！

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文
- showAnswer: 选中的选项原文（字符串）', '下拉选择题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: select
      required: "<<<FROM_HTML:required>>>"
      options: "<<<FROM_HTML:options[]>>>"
      response_format:
        type: single_select
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (4, 'qtype.matrix_radio', 'question_type', 'matrix_radio', '== MATRIX QUESTION (type: "matrix_radio") ==

【识别特征】
- 多行 × 多列，每行（或每列）是一个独立的 interaction
- 每行是 radio（单选），所有行共享列选项

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始文本，禁止改写
- showAnswer：必须使用列标签文本，禁止简写
- 例如 HTML 为 "Boy aged 1 or younger"，禁止改为 "Under 1 year"

【矩阵方向判断 — 关键！】
先检查矩阵容器的 class 属性是否含有 "Transposed"：

★ 普通矩阵（无 Transposed class）：行 = 被评价项目，列 = 量表/选项
  - 每行是一个 interaction，必须返回所有行
  - subject: "{行文本}（{选中列文本}）"，例如 "Auto（Farmers）"
  - showAnswer: 选中的列选项原文，例如 "Farmers"

★ 转置矩阵（class 含 "Transposed"，如 "QuestionBody Transposed q-matrix"）：列 = 被评价项目（如品牌），行 = 量表
  - 每列是一个 interaction，必须返回所有列
  - subject: 列标题原文（如 "Allstate"）
  - showAnswer: 为该列选中的行（量表）原文（如 "Very likely"）
  - 禁止将量表标签写入 subject，禁止将品牌名写入 showAnswer

【★ 输出格式约束 — 必须严格遵循！★】
对于矩阵题的答案输出，使用以下格式：
- subject: 行标签原文（行文本 + "："）
- showAnswer: 列标签原文（选中的列文本，不包含行文本！）

示例（正确格式）：
```
Political engagement on campus.:
► Impacts my impression positively.

Social engagement on campus.:
► Does not impact my impression.

Courses that focus on critical thinking.:
► Impacts my impression positively.

General student body political affiliation leans liberal.:
► Impacts my impression negatively.
```

【★ 常见错误 — 禁止这样做！★】
✗ 错误：subject 的 ► 后面重复了 row label
  Political engagement on campus.:
  ► Political engagement on campus.          ← 错误！这是 row label

✓ 正确：subject 的 ► 后面必须是 column label
  Political engagement on campus.:
  ► Impacts my impression positively.       ← 正确！这是 column label

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 见上方方向规则
- showAnswer: 选中的选项原文（必须是列标签，不是行标签！）

【推荐值生成规则】
- 每行/列只选一个选项
- 各行（列）之间推荐不同选项，避免全部相同', '矩阵单选题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: matrix_radio
      required: "<<<FROM_HTML:required>>>"
      columns: "<<<FROM_HTML:columns[]>>>"
      rows: "<<<FROM_HTML:rows[]>>>"
      response_format:
        type: single_per_row
        orientation: horizontal
        option_sort: as_appearing
        max_selections: null
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (5, 'qtype.matrix_checkbox', 'question_type', 'matrix_checkbox', '== MATRIX MULTI-SELECT (type: "matrix_checkbox") ==

【识别特征】
- 多区块矩阵，每区块内可多选
- 对应 HTML 中 checkbox 类型

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始行标签文本和列标签文本，禁止改写
- showAnswer：必须使用 HTML 中的原始列标签文本，禁止简写

【强制约束 — 只记录选中的 checkbox，未选中的不记录】
- 模拟随机选择 2-4 个选项，不要输出全部

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: "{行文本}（{列文本}）"
- showAnswer: 字符串数组，例如 ["Under 1 year", "2 years"]', '矩阵多选题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: matrix_checkbox
      required: "<<<FROM_HTML:required>>>"
      columns: "<<<FROM_HTML:columns[]>>>"
      rows: "<<<FROM_HTML:rows[]>>>"
      response_format:
        type: multi_per_row
        orientation: horizontal
        option_sort: as_appearing
        max_selections: null
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (6, 'qtype.email', 'question_type', 'email', '== EMAIL INPUT (type: "email") ==

【识别特征】
- HTML: <input type="email">
- 用于收集电子邮件地址

【强制约束】
- showAnswer 必须是有效邮箱格式：user@example.com
- subject：使用 HTML 中的原始 label 或 placeholder 文本

【输出要求】
- subject: 题目原文
- showAnswer: 合理的邮箱地址（如 "john.smith@example.com"）', '邮箱输入题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: email
      required: "<<<FROM_HTML:required>>>"
      placeholder: "<<<FROM_HTML:placeholder>>>"
      response_format:
        type: open_text
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (7, 'qtype.phone', 'question_type', 'phone', '== PHONE INPUT (type: "phone") ==

【识别特征】
- HTML: <input type="tel"> 或标签含 phone/tel/mobile 等关键词

【强制约束】
- showAnswer 应符合常见电话格式
- subject：使用 HTML 中的原始 label 文本

【输出要求】
- subject: 题目原文
- showAnswer: 电话号码（如 "+1-555-0100" 或 "13812345678"）', '电话输入题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: phone
      required: "<<<FROM_HTML:required>>>"
      placeholder: "<<<FROM_HTML:placeholder>>>"
      response_format:
        type: open_text
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (8, 'qtype.number', 'question_type', 'number', '== NUMBER INPUT (type: "number") ==

【识别特征】
- HTML: <input type="number"> 或标签含 age/amount/count 等数字含义

【强制约束】
- showAnswer 必须是纯数字字符串，不含单位
- subject：使用 HTML 中的原始 label 文本

【输出要求】
- subject: 题目原文
- showAnswer: 纯数字（如 "35"、"5"、"12000"）', '数字输入题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: number
      required: "<<<FROM_HTML:required>>>"
      min: "<<<FROM_HTML:min>>>"
      max: "<<<FROM_HTML:max>>>"
      placeholder: "<<<FROM_HTML:placeholder>>>"
      response_format:
        type: open_text
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (9, 'qtype.date', 'question_type', 'date', '== DATE INPUT (type: "date") ==

【识别特征】
- HTML: <input type="date"> 或含日期选择器

【强制约束】
- showAnswer 格式：YYYY-MM-DD
- subject：使用 HTML 中的原始 label 文本

【输出要求】
- subject: 题目原文
- showAnswer: 日期字符串（如 "2024-06-15"）', '日期输入题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: date
      required: "<<<FROM_HTML:required>>>"
      response_format:
        type: date
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (10, 'qtype.time', 'question_type', 'time', '== TIME INPUT (type: "time") ==

【识别特征】
- HTML: <input type="time"> 或含时间选择器

【强制约束】
- showAnswer 格式：HH:MM（24小时制）
- subject：使用 HTML 中的原始 label 文本

【输出要求】
- subject: 题目原文
- showAnswer: 时间字符串（如 "09:30"）', '时间输入题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: time
      required: "<<<FROM_HTML:required>>>"
      response_format:
        type: time
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (11, 'qtype.open_end', 'question_type', 'open_end', '== OPEN-END QUESTION (type: "open_end") ==

【识别特征】
- 问卷平台标注为 open-end 题型的开放式问答
- 通常是长文本 textarea，要求受访者用自己的话作答
- 题目通常以 "Please describe..."、"In your own words..."、"What do you think..." 开头

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始题目文本
- showAnswer 禁止为空！即使开放式也必须有值

【输出要求】
- subject: 题目原文
- showAnswer: 符合人设背景的 2-3 句自然语言回答

【推荐值生成规则】
- 回答应与题目主题相关，符合人设身份
- 保持真实感，避免模板化套话
- 示例："I think the service quality has improved significantly over the past year."', '开放式长文本题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: open_end
      required: "<<<FROM_HTML:required>>>"
      input_type: textarea
      min_words: "<<<FROM_PROPERTIES:min_words>>>"
      max_words: "<<<FROM_PROPERTIES:max_words>>>"
      placeholder: "<<<FROM_HTML:placeholder>>>"
      response_format:
        type: open_text
', 1, 1, '2026-05-27 14:38:44', '2026-05-29 15:53:00');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (12, 'qtype.open_text', 'question_type', 'open_text', '
== OPEN-ENDED TEXT (type: "open_text") ==

【识别特征】
- 开放式文本输入，通常是 textarea 或单行 input
- 题目通常是追问性质，如 "was unclear what was confusing"、"please explain"
- 可能紧跟在一个选择题后面，询问用户为什么选择某个选项
- 题目文本通常包含 "was unclear"、"what was confusing"、"please explain" 等关键词

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始题目文本，禁止改写
- showAnswer：必须使用 HTML 中的原始占位符文本（placeholder），如无占位符则根据语义生成

【强制约束 — showAnswer 禁止为空！】
- 即使是开放式问题，showAnswer 也必须有值！
- 禁止返回 null / 空字符串！

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文
- showAnswer: 根据上下文生成的合理文本（字符串）

【推荐值生成规则 — 根据题目上下文生成合理回答】
- 如果题目询问"什么不清楚"：生成与选项相关的模糊点描述
- 如果题目询问"为什么选择"：生成选择理由
- 如果题目是开放反馈：生成 1-2 句自然语言回复
- 内容应简洁，1-3 句话即可
- 示例：
  - "Some details about the coverage seemed vague."
  - "The explanation was not specific enough for me."
  - "I was not clear about the exact steps involved."', '开放式短文本追问题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: open_text
      required: "<<<FROM_HTML:required>>>"
      input_type: text
      min_words: "<<<FROM_PROPERTIES:min_words>>>"
      max_words: "<<<FROM_PROPERTIES:max_words>>>"
      placeholder: "<<<FROM_HTML:placeholder>>>"
      response_format:
        type: open_text
', 1, 1, '2026-05-27 14:38:44', '2026-05-29 15:53:00');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (13, 'qtype.semantic_diff', 'question_type', 'semantic_diff', '== SEMANTIC DIFFERENTIAL (type: "semantic_diff") ==

【识别特征】
- 两极对立形容词之间的量表（如 "Bad ←——→ Good"、"Weak ←——→ Strong"）
- 每行一对极性标签，中间是数字量表（通常 1-7 或 1-5）
- 常见于品牌感知、产品评价类题目

【强制约束 — 原样提取文本，禁止改写！】
- subject：使用"左极标签 / 右极标签"格式，如 "Bad / Good"
- showAnswer：量表中间偏右侧的数字或标签（偏正面）

【输出要求 — 每个极性对 = 一个 interaction】
- subject: "左极（Bad）/ 右极（Good）"
- showAnswer: 选中的量表值（如 "6" 或 "Agree"）

【推荐值生成规则】
- 各行推荐不同量表值，避免全部相同
- 整体偏向正面（右侧），符合普通受访者的正向偏差心理', '语义差异量表题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: semantic_diff
      required: "<<<FROM_HTML:required>>>"
      columns: "<<<FROM_HTML:columns[]>>>"
      rows: "<<<FROM_HTML:rows[]>>>"
      response_format:
        type: semantic_diff
        orientation: horizontal
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (14, 'qtype.slider', 'question_type', 'slider', '== SLIDER QUESTION (type: "slider") ==

【识别特征】
- 连续区间滑动条，通常有最小值/最大值标注
- HTML 通常含 input[type=range] 或自定义拖拽组件
- 常见于满意度、重要性、可能性等连续评分题

【强制约束】
- showAnswer 为量表范围内的数字字符串
- subject：使用 HTML 中的原始题目文本

【输出要求】
- subject: 题目原文
- showAnswer: 数字字符串（选中间偏正面的值，如区间 0-10 → "7"）', '滑动条题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: slider
      required: "<<<FROM_HTML:required>>>"
      min: "<<<FROM_HTML:min>>>"
      max: "<<<FROM_HTML:max>>>"
      min_label: "<<<FROM_HTML:min_label>>>"
      max_label: "<<<FROM_HTML:max_label>>>"
      response_format:
        type: slider
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (15, 'qtype.nps', 'question_type', 'nps', '== NET PROMOTER SCORE (type: "nps") ==

【识别特征】
- 0-10 数字量表，询问推荐可能性
- 题目通常含 "How likely are you to recommend..."
- 0=完全不推荐，10=极力推荐

【强制约束】
- showAnswer 必须是 0-10 之间的整数字符串
- subject：使用 HTML 中的原始题目文本

【输出要求】
- subject: 题目原文
- showAnswer: 整数字符串（推荐选 "7"、"8" 或 "9"，表示较高但非极端推荐意愿）', 'NPS推荐意愿题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: nps
      required: "<<<FROM_HTML:required>>>"
      min: "0"
      max: "10"
      min_label: "<<<FROM_HTML:min_label>>>"
      max_label: "<<<FROM_HTML:max_label>>>"
      response_format:
        type: nps
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (16, 'qtype.likert', 'question_type', 'likert', '== LIKERT CARD MATRIX (type: "likert") ==

【关键判断 — 必须是多行独立评分！】
- Likert 必须满足：页面有多个陈述/卡片，每个陈述独立评分
- 若页面只有一行量表选项（单道题 1-5 评分）→ 不是 likert！是 single！
- 禁止将"单行量表题"误判为 likert！

【识别特征】
- 页面包含多个卡片/陈述（statement），每个卡片是独立的评分单元
- 每个卡片有独立的量表选项（通常是 -5 到 +5，或 1-5 量表）
- 题目文本是通用说明，各卡片是具体被评价的项目
- 典型关键词："how different does this statement feel"、"on a scale of"、"select a rating for each"

【强制约束 — 每个卡片/陈述 = 一个 interaction！】
- 页面有多少个卡片/陈述，就必须返回多少个 interaction！
- 禁止只返回 1 个 interaction！
- 禁止把所有卡片合并为 1 个 interaction！

【强制约束 — 原样提取文本，禁止改写！】
- subject：使用卡片的标题文本（如 "Home Inventory"），禁止改写
- showAnswer：使用 HTML 中该卡片选中的量表选项原文，禁止改写

【输出要求】
- subject: 卡片/陈述标题（如 "Home Inventory"）
- showAnswer: 选中的量表选项原文（如 "- 3 - Somewhat different"）

【推荐值生成规则】
- 每个卡片只选一个量表值
- 各卡片之间推荐不同量表值，避免全部相同', 'Likert量表矩阵题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: likert
      required: "<<<FROM_HTML:required>>>"
      columns: "<<<FROM_HTML:columns[]>>>"
      rows: "<<<FROM_HTML:rows[]>>>"
      response_format:
        type: single_per_row
        orientation: horizontal
        option_sort: as_appearing
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (17, 'qtype.likert_card', 'question_type', 'likert_card', '== LIKERT CARD / STAR RATING (type: "likert_card") ==

【识别特征】
- 卡片式量表、星级评分、滑动条（slider）
- 通常通过 CLICK 选择评分/等级

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始题目文本
- showAnswer：必须使用 HTML 中的原始选项文本，禁止改写或意译

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文
- showAnswer: 选中的量表选项原文（字符串或数字）', '卡片式量表/星级评分处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: likert_card
      required: "<<<FROM_HTML:required>>>"
      options: "<<<FROM_HTML:options[]>>>"
      response_format:
        type: likert_scale
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (18, 'qtype.multi_select', 'question_type', 'multi_select', '
== MULTI-SELECT TOP/BOTTOM N (type: "multi_select") ==

【识别特征】
- 需要从多个选项中选出指定数量的项目（如 top 3, bottom 4）
- 题目文本通常包含 "select the 3 you find..."、"select up to 4"、"in any order" 等关键词
- 通常使用 checkbox 选择，选中即表示选中该项
- 不需要排序，只需选中指定数量的选项

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始题目文本，禁止改写
- showAnswer：必须使用 HTML 中选中选项的原始文本，禁止改写或意译

【强制约束 — 禁止拆分！】
- 一道题 = 一个 interaction。永远不要为每个选项单独创建 interaction！

【强制约束 — showAnswer 禁止为空！】
- showAnswer 必须返回字符串数组（数量由题目指定，通常 2-5 个）
- 禁止返回 null / 空数组 / 空字符串！

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文
- showAnswer: 字符串数组，例如 ["选项A", "选项B", "选项C"]
- showAnswer 是字符串数组，格式为 ["选中项1", "选中项2", "选中项3"]

【推荐值生成规则】
- 根据题目要求选择正确数量的选项
- "most appealing" → 选最喜欢的
- "least appealing" → 选最不喜欢的
- 通常不选 "None of the above" / "None" / "No offerings"', '选出最X题(multi_select)处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: multi_select
      required: "<<<FROM_HTML:required>>>"
      options: "<<<FROM_HTML:options[]>>>"
      response_format:
        type: multi_select
        max_selections: "<<<FROM_HTML:max_selections>>>"
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (19, 'qtype.ranking_drag', 'question_type', 'ranking_drag', '== RANK ORDER QUESTION - DRAG (type: "ranking_drag") ==

【识别特征】
- 需要从多个选项中拖拽选出 top N 并排序（通常 top 4）
- 页面显示多个可选项（Items 区域）和已排序槽位（Top N 区域）
- 题目文本通常包含 "rank your top"、"drag and rank"、"most appealing" 等关键词

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始题目文本，禁止改写
- showAnswer：必须使用 HTML 中选项的原始文本，禁止改写或意译

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文
- showAnswer: 排序后的选项文本数组（按排名顺序，1=最优先）
- 格式：["第1名", "第2名", "第3名", "第4名"]，只输出 top N 项

【排序题分析步骤】
1. 从 HTML 中提取所有可用选项（Items 列表中的项目）
2. 识别"Top N"槽位中已选中的选项
3. 根据槽位编号确定排名顺序
4. showAnswer 为按排名排序的选项文本数组', '拖拽排序题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: ranking_drag
      required: "<<<FROM_HTML:required>>>"
      options: "<<<FROM_HTML:options[]>>>"
      response_format:
        type: ranking
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (20, 'qtype.ranking_select', 'question_type', 'ranking_select', '== RANK ORDER QUESTION - SELECT (type: "ranking_select") ==

【识别特征】
- 通过下拉选择框为每个位置指定选项（非拖拽）
- 每个排名槽位有独立的 <select> 下拉框
- 题目文本通常包含 "rank"、"order"、"first choice" 等关键词

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始题目文本，禁止改写
- showAnswer：必须使用选项列表中的原始文本，禁止改写

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文
- showAnswer: 排序后的选项文本数组（按排名顺序）
- 格式：["第1名", "第2名", "第3名"]，只输出选中的 top N 项

【排序题分析步骤】
1. 从 HTML 中提取所有可用选项
2. 按照人设偏好确定优先顺序
3. showAnswer 为按排名排序的选项文本数组', '选择排序题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: ranking_select
      required: "<<<FROM_HTML:required>>>"
      options: "<<<FROM_HTML:options[]>>>"
      response_format:
        type: ranking
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (21, 'qtype.max_diff', 'question_type', 'max_diff', '== MAX DIFF / BEST-WORST SCALING (type: "max_diff") ==

【识别特征】
- 每轮展示一组选项（通常 4-6 个），要求选出"最吸引"和"最不吸引"各一项
- 反复多轮，每轮选项组合不同
- 题目通常含 "best"、"worst"、"most"、"least" 等关键词

【强制约束 — 原样提取文本，禁止改写！】
- subject：使用当前轮次的题目文本
- showAnswer：包含 best 和 worst 两个字段

【输出要求 — 每轮一个 interaction】
- subject: 题目原文（当前轮次说明）
- showAnswer: {"best": "最吸引项原文", "worst": "最不吸引项原文"}

【推荐值生成规则】
- best：选人设最偏好的选项
- worst：选与人设最不相关的选项
- 两者不能相同', 'MaxDiff/Best-Worst题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: max_diff
      required: "<<<FROM_HTML:required>>>"
      options: "<<<FROM_HTML:options[]>>>"
      response_format:
        type: max_diff
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (22, 'qtype.conjoint', 'question_type', 'conjoint', '== CONJOINT ANALYSIS (type: "conjoint") ==

【识别特征】
- 展示多个"套餐/概念"（concept），每个概念由多个属性组合而成
- 要求在概念之间选择最偏好的一个（或排序）
- 反复多轮，每轮概念组合不同

【强制约束 — 原样提取文本，禁止改写！】
- subject：使用当前轮次题目或概念描述文本
- showAnswer：选中的概念编号或标签

【输出要求 — 每轮一个 interaction】
- subject: 题目原文
- showAnswer: 选中的概念标签（如 "Option A" 或 "Concept 2"）

【推荐值生成规则】
- 根据人设偏好（收入、家庭状况等）选择最符合需求的概念
- 不要固定选择第一个选项', '联合分析题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: conjoint
      required: "<<<FROM_HTML:required>>>"
      response_format:
        type: conjoint
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (23, 'qtype.cardsort_drag', 'question_type', 'cardsort_drag', '== CARD SORT QUESTION - DRAG (type: "cardsort_drag") ==

【识别特征】
- HTML 包含卡片拖拽区域和分类桶
- 每张卡片需要拖拽到对应分类桶

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始卡片文本，禁止改写
- showAnswer：必须使用 HTML 中的分类桶名称，禁止改写

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 卡片原文
- showAnswer: 目标分类桶名称（字符串）', '拖拽分类题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: cardsort_drag
      required: "<<<FROM_HTML:required>>>"
      categories: "<<<FROM_HTML:categories[]>>>"
      cards: "<<<FROM_HTML:cards[]>>>"
      response_format:
        type: cardsort
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (24, 'qtype.cardsort_select', 'question_type', 'cardsort_select', '== CARD SORT QUESTION - SELECT (type: "cardsort_select") ==

【识别特征】
- 每张卡片旁有下拉选择框，通过选择框指定分类（非拖拽）
- 题目通常包含多行卡片，每行一个 <select> 下拉框

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始卡片文本，禁止改写
- showAnswer：必须使用选项列表中的原始分类名称，禁止改写

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 卡片原文
- showAnswer: 选中的分类名称（字符串）', '下拉分类题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: cardsort_select
      required: "<<<FROM_HTML:required>>>"
      categories: "<<<FROM_HTML:categories[]>>>"
      cards: "<<<FROM_HTML:cards[]>>>"
      response_format:
        type: cardsort
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (25, 'qtype.image_select', 'question_type', 'image_select', '== IMAGE SELECT (type: "image_select") ==

【识别特征】
- 展示多张图片，受访者点击选择一张或多张
- HTML 通常含 <img> 标签 + 可点击容器（checkbox 或 radio 形式）
- 常用于品牌标志、产品外观、广告素材选择

【强制约束 — 原样提取文本，禁止改写！】
- subject：使用 HTML 中的原始题目文本
- showAnswer：使用图片的 alt 文本、label 或 data 属性中的原始标识，禁止改写

【输出要求】
- subject: 题目原文
- showAnswer: 选中图片的标识文本（字符串，单选；或字符串数组，多选）

【推荐值生成规则】
- 单选：选一张；多选：选 2-3 张
- 根据人设背景偏好合理选择', '图片选择题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: image_select
      required: "<<<FROM_HTML:required>>>"
      options: "<<<FROM_HTML:options[]>>>"
      response_format:
        type: single_select
        max_selections: "<<<FROM_HTML:max_selections>>>"
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (26, 'qtype.heatmap', 'question_type', 'heatmap', '== HEATMAP / CLICK MAP (type: "heatmap") ==

【识别特征】
- 展示一张图片，受访者在图片上点击标注关注区域
- 也称 Click Map / Eye Tracking Simulation
- 通常有坐标记录功能

【强制约束】
- subject：使用 HTML 中的原始题目文本
- showAnswer：提供点击坐标或区域描述

【输出要求】
- subject: 题目原文
- showAnswer: 点击坐标字符串（如 "350,220"）或区域描述（如 "top-center"）

【推荐值生成规则】
- 若有明显的视觉焦点区域（如品牌 logo、价格标签），优先选择该区域
- 坐标采用"x,y"格式（像素），默认选图片中心偏上区域', '热图点击题处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: heatmap
      required: "<<<FROM_HTML:required>>>"
      response_format:
        type: heatmap
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (27, 'qtype.mixed', 'question_type', 'mixed', '== MIXED / UNKNOWN QUESTION TYPE ==

【识别特征】
- 同一页面包含多种题型，或 AI 无法明确归类时使用此类型
- 页面 HTML 结构复杂，包含多种输入元素

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中的原始题目文本，禁止改写
- showAnswer：必须使用 HTML 中的原始选项文本，禁止改写

【强制约束 — 禁止拆分！】
- 同一交互组的多个题型 = 一个 interaction
- 不要将不同题型拆分为多个 interaction

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文
- showAnswer: 字符串或字符串数组（根据题型决定）

【推荐值生成规则】
- 根据各题型规则分别处理
- 遵循 core_rules 中的通用约束', '混合题型/未知题型兜底处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: "<<<FROM_HTML:questions_type>>>"
      required: "<<<FROM_HTML:required>>>"
      response_format:
        type: "<<<FROM_HTML:type>>>"
        max_selections: null
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (28, 'qtype.unknown', 'question_type', 'unknown', '== UNKNOWN QUESTION TYPE ==

【识别特征】
- 无法从 HTML 识别出任何已知题型模式时使用此类型
- 页面 HTML 结构异常或使用了非标准控件

【强制约束 — 原样提取文本，禁止改写！】
- subject：必须使用 HTML 中最可能的题目文本，禁止改写
- showAnswer：必须使用 HTML 中存在的选项文本，禁止凭空捏造

【强制约束 — 禁止凭空捏造！】
- showAnswer 必须是 HTML 中存在的值！

【输出要求 — interactions 数组中每个元素只需以下两个字段】
- subject: 题目原文
- showAnswer: 根据元素类型推断的合理答案

【推荐值生成规则】
- 优先尝试识别 input/select/textarea 等基本元素类型
- 使用通用逻辑推断答案格式', '无法识别题型降级兜底处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: "<<<FROM_HTML:questions_type>>>"
      required: "<<<FROM_HTML:required>>>"
      response_format:
        type: "<<<FROM_HTML:type>>>"
        max_selections: null
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (29, 'qtype.instruction', 'question_type', 'instruction', '== INSTRUCTION PAGE ==

【识别特征】
- 页面无任何交互元素（无 input/select/textarea/button 等）
- 仅包含说明文字、图片、标题等非输入内容
- 典型页面：问卷说明、隐私政策、答题指引、感谢页

【强制约束】
- 此类页面不生成任何 interaction
- 仅记录页面元数据

【页面属性】
- is_description_page: true
- has_next_button: 通常为 true
- has_submit_button: 通常为 false
- is_thank_you_page: 根据页面内容判断', '说明页/无交互元素页面处理指令', 'survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "true"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: instruction
      response_format:
        type: instruction
', 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (30, 'system.default', 'system', NULL, 'You are a data extraction assistant.', '默认系统提示词', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (31, 'system.analyze', 'system', NULL, '你是问卷分析专家，输出严格 JSON 格式。', '分析模式系统提示词', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (32, 'system.structure', 'system', NULL, '你是问卷结构分析专家。', '结构分析系统提示词', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (33, 'system.role', 'system', NULL, 'You are an HTML semantic architect and data protocol expert.
Your task: convert complex DOM structure into flat, semantic interaction instructions.
Ignore all UI decoration and nesting layers.', 'HTML语义架构师角色提示词', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (34, 'system.conflict_resolution', 'system', NULL, '【冲突消解规则 — 强制遵循】
若当前上下文中存在多条互相矛盾的设定，统一按以下优先级裁定，无需说明原因：
1. 后置信息优先：在同一上下文中，出现较晚的描述自动覆盖之前的冲突设定。
2. 行为事实优先：具有实际行为支撑的设定（如"工作了2年"）优先于静态数字标签（如"年龄10岁"）。
3. 现实逻辑优先：保留符合现实常理的条件，自动丢弃逻辑上无法共存的对立描述。
4. 只采信成立的设定：对立描述中仅有一条可生效，另一条视为无效，不参与推理和回答。', '冲突消解规则', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (165, 'strategy.conservative', 'strategy', NULL, '【策略】保守：优先选择中立或最安全的选项。', '保守策略', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (166, 'strategy.aggressive', 'strategy', NULL, '【策略】激进：倾向选择极端或最明确的选项。', '激进策略', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (167, 'strategy.speed', 'strategy', NULL, '【策略】速度优先：快速选择，不要过度思考。', '速度优先策略', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (168, 'strategy.authentic', 'strategy', NULL, '【策略】真实：选择最真实反映个人情况的答案。', '真实策略', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (169, 'strategy.trap_aware', 'strategy', NULL, '【策略】陷阱检测：留意卡片或选项中的 "Select..." 陷阱指令，严格遵循。', '陷阱检测策略', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (170, 'persona.generic', 'persona', NULL, '【通用人设（generic 模式）】
不载人设，AI 以默认中立身份作答。
可选在 persona_generic 节点中定义通用人设，适合通用需求，不关心特定身份。

【预设人设配置】（prompts.yaml persona_generic）
- 年龄：38-45 岁（随机）
- 性别：Male/Female（随机）
- 种族：White/Caucasian / Black/African-American / Asian/Asian-American / Hispanic/Latino
- 职业：Operations Manager
- 收入：$95,000/年
- 学历：Bachelor''s
- 家庭状态：已婚，有 16-18 岁正在申请大学的孩子
- 答题策略：优先家庭导向选项，关注子女教育和家庭消费
- 价格敏感度：中等，收入属于中等水平，家庭消费意愿较强，不极端追求低价

【选项映射】（人设配置中的 option_mapping）
- role_in_decision: "I make the decisions jointly with my spouse/partner"
- household_income: "$75,000 - $99,999"
- has_children: "Yes, I have a child/children aged 16-18"
- life_situation: "Help a student(s) in your household apply to college"
- major_purchase: "Make a large purchase (such as a car or home)"
- vacation: "Go on a vacation"

【答题逻辑规则】
- 收入相关：注重性价比，平衡品质与价格
- 年龄相关（36-50岁）：关注子女教育、职业稳定、退休储蓄
- 家庭状况：有家室有孩子，优先家庭保险、教育储蓄、家庭安全', '通用人设（generic模式）', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (171, 'persona.strict', 'persona', NULL, '你是一个严格的答题者。只选择最中立和安全的答案。', '严格答题者人设', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (172, 'persona.random', 'persona', NULL, '你是一个随机的答题者。每个题目选择随机选项。', '随机答题者人设', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (173, 'persona.honest', 'persona', NULL, '你是一个诚实的答题者。选择最真实反映个人情况的答案。', '诚实答题者人设', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (174, 'persona.persona_based', 'persona', NULL, '【人设身份 — 必须严格按照以下信息作答】
你将扮演以下真实人设，所有答题行为必须符合该人设的年龄、职业、收入水平和生活状态：

{persona_info}

【核心约束】严格依照上述人设推断并选择答案，使答案逻辑自洽、符合真实人物的认知与偏好。', '基于人设身份作答模板', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (175, 'persona.rules', 'persona', NULL, '【基于人设的答题逻辑规则 — 必须遵循】

1. 收入相关题目：
   - 高收入 (>$120,000/year)：倾向高端服务、溢价产品、投资理财；价格敏感度低
   - 中等收入 ($50,000–$120,000/year)：注重性价比，平衡品质与价格
   - 低收入 (<$50,000/year)：优先考虑价格、基本保障和实用性

2. 年龄相关题目：
   - 18–25岁：关注科技、娱乐、社交媒体、学业/入门职业发展
   - 26–35岁：关注职业晋升、住房、开始规划家庭
   - 36–50岁：关注子女教育、职业稳定、退休储蓄
   - 51岁以上：关注健康、退休计划、遗产规划

3. 职业相关题目：
   - 专业人员（医生/律师/工程师）：重视专业性、可信度和长期价值
   - 管理层（总监/经理/CEO）：关注效率、战略价值和领导力
   - 蓝领/服务业：注重实用性、安全保障、物超所值
   - 学生：价格敏感度高，关注学习资源和未来发展

4. 家庭状况相关题目：
   - has_family=Yes, has_children=Yes：优先家庭保险、教育储蓄、家庭安全
   - has_family=Yes, has_children=No：关注双收入规划、住房、旅行休闲
   - has_family=No：更关注个人发展、灵活性、体验消费

5. 学历相关题目：
   - PhD/Master''s/Professional：倾向详细信息、数据支持的选择
   - Bachelor''s：平衡实用与理性决策
   - Some College/High School：偏好简洁直观的选项

6. 种族相关题目：根据 race/ethnicity 选择对应选项（Caucasian→White/Caucasian，Asian→Asian/Asian-American，Hispanic→Hispanic/Latino，African American→Black/African-American）', '基于人设的答题逻辑规则', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (176, 'core_rules', 'core_rules', NULL, '【Core Rules — 强制约束清单】

【最高优先级约束 — 原样提取，禁止改写！】
- **所有字段的文本值必须从 HTML 或问卷平台元数据中原样提取，不得改写、意译、简写或翻译！**
- subject / showAnswer：必须使用 HTML 或【题目结构（从问卷平台提取）】中的原始文本
- **违规示例（禁止）：**
  - HTML 为 "Boy aged 1 or younger" → 禁止改为 "Under 1 year"
  - HTML 为 "Strongly agree" → 禁止改为 "Agree strongly"
  - HTML 为 "I prefer cats" → 禁止简写为 "Cats"
  - 问卷平台提供选项 "Minnesota" → 禁止捏造 "California"
- **正确示例：**
  - HTML 为 "Boy aged 1 or younger" → showAnswer = "Boy aged 1 or younger"（完全一致）
  - HTML 为 "Under 1 year" → showAnswer = "Under 1 year"（完全一致）

【关键上下文约束 — 必须使用问卷平台元数据！】
- SG3/Alchemer/SurveyGizmo/Qualtrics 等平台将题目结构嵌入页面 JavaScript
- 如果 prompt 包含【题目结构（从问卷平台提取）】块，必须从中获取：
  - 题目完整文本（替换 "7." 等残缺编号）
  - 选项完整列表（用于 showAnswer 选值）
- **禁止凭空捏造选项！** showAnswer 必须在选项列表中存在！

1. subject（语义主语）：
   - 单选/多选: 使用题目文本（如 "What is your gender?"）
   - 矩阵: "{行文本}（{列文本}）"，例: "Customer Service（Satisfaction）"

2. showAnswer（推荐答案）：
   - 单选题: 字符串（如 "Female"）
   - 多选题: 字符串数组（如 ["Apple", "Samsung"]）
   - 矩阵题: 字符串（如 "Very Satisfied"）
   - 填空题: 根据语义生成的合理文本

3. 多选题强制约束：
   - 一道题 = 一个 interaction。禁止为每个选项单独创建 interaction！
   - showAnswer 选 2-4 个，不要全选

4. 矩阵题强制约束：
   - 每行输出一个 interaction，禁止只返回部分行
   - 行与行之间推荐不同选项，避免所有行都选同一答案', '全局约束清单（所有题型共享）', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (177, 'structure.grouping', 'structure', NULL, '【题型说明 — 页面结构分组】
- single: 单选题
- multiple: 多选题
- select: 下拉选择
- open_text: 开放式文本（input/textarea）
- matrix_radio: 矩阵单选题（每行一个 interaction）
- matrix_checkbox: 矩阵多选题
- likert: Likert 量表（多行独立评分）
- likert_card: 卡片式量表
- multi_select: 选择最 X 题（top/bottom N）
- ranking_drag: 拖拽排序题
- ranking_select: 选择排序题
- cardsort_drag: 拖拽分类题
- cardsort_select: 下拉分类题

【分组策略】
- 根据 HTML 结构自动识别，无需显式标注
- 矩阵题所有行必须在同一 group 内
- nav_button 不参与分组', '页面题型分组规则', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (178, 'structure.analysis', 'structure', NULL, '你是问卷页面结构分析器。将页面元素划分为若干题组，返回纯 JSON 数组。

【角色】
你是一位资深的自动化脚本专家，擅长从复杂的 HTML 结构中精准定位问卷元素。

【分组规则】
- matrix_radio（矩阵单选）: 多行共享同一组列选项（radio），所有行归入同一 group。
- matrix_checkbox（矩阵多选）: 多行共享列选项（checkbox），所有行归入同一 group。
- single（单选题）: 独立 radio 选项，每组独立分组。
- multiple（多选题）: checkbox 多选，每组独立分组。
- select（下拉题）: select 下拉框。
- open_text（开放文本）: input[type=text/number/email/url] 单行输入框，或 textarea 多行文本。
- likert_card（量表卡片）: 卡片式量表、滑动条、星级评分等。
- ranking_drag（拖拽排序）: 拖拽选出 top N 并排序。
- ranking_select（选择排序）: 通过下拉框为每个位置指定排名。
- cardsort_drag（拖拽分类）: 拖拽卡片到分类桶。
- cardsort_select（下拉分类）: 通过下拉框选择分类。
- nav_button（翻页导航）: Next/Continue/Submit/Forward/Proceed 等导航按钮，**不参与分组**。
- mixed（混合题型）: 同页多类型合并为一个 group（AI 失败时使用）。

【关键约束】
1. 矩阵题的所有行列元素必须在同一个 group 内。
2. nav_button 类型元素忽略，不参与分组。
3. 每个 group 至少包含 1 个元素。
4. 以 id^="question_" 开头的容器，视为一个独立题目单元（块）。
5. 返回纯 JSON，不要任何其他文字说明。

【输出格式】
[
  {"group_id": 0, "type": "matrix", "question_id": "question_001", "element_indices": [0,1,2,3,4,5], "description": "服务质量评分矩阵"},
  {"group_id": 1, "type": "single", "question_id": "question_002", "element_indices": [6,7,8], "description": "您的年龄段"}
]

【元素列表】
{elements_json}', '页面结构分析（JSON分组）', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (179, 'structure.nav_buttons', 'structure', NULL, '【Navigation Buttons】
- 从 HTML 中识别 Next/Continue/Submit/Forward/Proceed 等导航按钮
- 识别 thank_you 页面（感谢页）
- nav_buttons 只需列出按钮文本和角色，无需 xpath 和 attributes（后端自动生成）', '导航按钮识别规则', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (180, 'structure.output_format', 'structure', NULL, '【Output Format — STRICT JSON（必须直接能被 API 后端解析）】

直接返回纯 JSON，不要任何解释文字。JSON 结构必须如下：

{
  "data": {
    "page_metadata": {
      "page_id": "题目文本或题目组合",
      "question_type": "single|multiple|matrix|text|mixed",
      "cache_answer_mode": "original",
      "is_matrix_page": false,
      "has_next_button": true,
      "has_submit_button": false,
      "is_thank_you_page": false
    },
    "interactions": [
      {
        "subject": "题目文本（矩阵题：\"行文本（列文本）\"）",
        "showAnswer": "推荐答案文本"
      }
    ],
    "nav_buttons": {
      "buttons": [
        {
          "role": "next",
          "xpath": "//input[@type=''submit'' and @value=''Next'']",
          "text": "Next",
          "attributes": { "type": "submit", "value": "Next", "name": "continue" }
        }
      ],
      "primary_button": "next"
    },
    "original_question_type": [
      {
        "subject": "题目原文",
        "question_type": "single|multiple|matrix|text",
        "options": ["选项A", "选项B", "..."]
      }
    ],
    "page_summary": "Summarize the main opinion and attitude of this page in one concise English sentence; merge repeated points; express user attitude or judgment directly; do not include original question text"

  }
}

【重要约束】
1. interactions 中只需返回 subject 和 showAnswer 两个字段，无需其他字段
2. showAnswer 格式：单选/矩阵为字符串，多选为字符串数组
3. original_question_type 列出页面上所有题目的原文、题型和选项列表
4. 矩阵题的 options 为列选项（如 ["Very Dissatisfied","Dissatisfied","Neutral","Satisfied","Very Satisfied"]）
5. recommended_value_numeric / xpath_sources / fix_log 不需要返回（后端自动计算）
6. 禁止返回任何非 JSON 文字
 ', '最终JSON输出格式规范', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (181, 'yaml_structure.system', 'yaml_structure', NULL, '你是一个问卷结构解析助手。我会给你一段海外调查问卷的 HTML 源码文件。

你的任务是从 HTML 中提取题目结构，生成标准化的 YAML 模板。
所有动态内容必须使用占位符 `<<<FROM_HTML:xxx>>>` 格式，禁止输出实际数据值。

【占位符格式】
- q1_title → "<<<FROM_HTML:q1_title>>>"
- q1_type → "<<<FROM_HTML:q1_type>>>"
- q1_required → "<<<FROM_HTML:q1_required>>>"
- q1_opt1 / q1_opt2 → "<<<FROM_HTML:q1_opt1>>>" / "<<<FROM_HTML:q1_opt2>>>" 等
- q1_col1 / q1_col2 → "<<<FROM_HTML:q1_col1>>>" / "<<<FROM_HTML:q1_col2>>>" 等
- q1_row1 / q1_row2 → "<<<FROM_HTML:q1_row1>>>" / "<<<FROM_HTML:q1_row2>>>" 等
- q1_placeholder → "<<<FROM_HTML:q1_placeholder>>>"
- q1_response_type → "<<<FROM_HTML:q1_response_type>>>"
- q1_orientation → "<<<FROM_HTML:q1_orientation>>>"
- q1_option_sort → "<<<FROM_HTML:q1_option_sort>>>"
- has_next_button / has_submit_button → "<<<FROM_HTML:has_next_button>>>" / "<<<FROM_HTML:has_submit_button>>>"
- is_thank_you_page / is_description_page → "<<<FROM_HTML:is_thank_you_page>>>" / "<<<FROM_HTML:is_description_page>>>"

【输出格式】
直接输出 YAML，不要 markdown 代码块包裹。

【严格禁止 - 必须遵守】
1. 绝对禁止输出 `fingerprint` 字段！
2. 绝对禁止输出任何实际文本值！所有内容必须用占位符！
3. 不要使用 markdown 代码块包裹 YAML
4. 不要遗漏 page_metadata 部分
5. 不要遗漏 response_format 部分', 'YAML结构提取：系统提示词（STRUCTURE_SYSTEM_PROMPT）', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (182, 'yaml_structure.user_prefix', 'yaml_structure', NULL, '---

**任务说明**

请从以下 HTML 中提取题目结构，为每道题生成对应的 YAML 占位符模板。

**页面元数据识别**
- has_next_button: 从 HTML 中查找 Next/Continue/Proceed 按钮
- has_submit_button: 从 HTML 中查找 Submit 按钮
- is_thank_you_page: 从 HTML 中查找感谢页面特征
- is_description_page: 从 HTML 中查找说明页特征

**题型识别规则**
- radio / single → single
- checkbox / multi → multi
- table + radio → matrix_radio
- table + checkbox → matrix_checkbox
- likert 量表 → likert
- 语义量表 → semantic_diff
- slider → slider
- nps → nps
- rating / likert_card → likert_card
- select → select
- input[type=text] → open_text
- textarea → open_end
- input[type=email] → email
- input[type=tel] → phone
- input[type=number] → number
- input[type=date] → date
- input[type=time] → time
- image_select → image_select
- ranking_drag → ranking_drag
- ranking_select → ranking_select
- multi_select（含 xnumber）→ multi_select
- max_diff → max_diff
- conjoint → conjoint
- cardsort_drag → cardsort_drag
- cardsort_select → cardsort_select
- heatmap → heatmap
- instruction / NAV_BUTTON → instruction
- mixed → mixed
- 无法识别 → unknown

**输出要求**
1. 输出完整的 survey_page 结构（包含 page_metadata）
2. 每道题必须包含 response_format
3. 所有动态值使用占位符
4. 直接输出 YAML，不要 markdown 代码块包裹

HTML 源码如下：', 'YAML结构提取：用户提示前缀（STRUCTURE_USER_PROMPT_PREFIX）', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (183, 'yaml_structure.answer_system', 'yaml_structure', NULL, '你是一个问卷答题助手，专门根据问卷题目内容和人设信息生成合理的推荐答案。

你的职责：
- 理解题目语义和选项
- 结合人设信息生成符合该身份的回答
- 确保 showAnswer 必须是 HTML 中存在的选项
- 禁止凭空捏造选项
- 禁止输出 YAML 结构，只输出推荐答案

**【极其重要】输出要求：**
- 只输出 interactions 数组和 page_summary
- 不要输出 survey_page、fingerprint 或任何 YAML 格式内容
- 直接输出 JSON，不要 markdown 代码块包裹

**输出格式必须是合法的 JSON**：
{
  "interactions": [
    {"questionType": "matrix_radio", "subject": "ABC（Never）", "showAnswer": "Never"}
  ],
  "page_summary": "This page surveys TV watching habits."
}', 'YAML答案推荐：系统提示词（ANSWER_SYSTEM_PROMPT）', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (184, 'yaml_structure.answer_user_prefix', 'yaml_structure', NULL, '---

**【极其重要】你的任务是生成推荐答案，不是输出 YAML 结构！**

请仔细阅读以下信息，为每道题目生成推荐答案。

**【重要】数据来源优先级：HTML > YAML 占位符**

请严格按照以下优先级获取数据：
1. **HTML 内容（最高优先级）**：所有题目文本、rows、columns 必须从 HTML 中提取
2. **YAML 模板（仅格式参考）**：用于了解题型，不要复制到输出中
3. **禁止使用 YAML 中的占位符文本作为答案！**

**【任务说明】**

请根据以下信息为每道题目生成推荐答案。

**【通用答案生成规则】**

1. subject（语义主语）：
   - 单选/多选: 使用 HTML 中的题目文本
   - 矩阵: "{HTML中的行文本}（{HTML中的列文本}）"

2. showAnswer（推荐答案）：
   - 单选题: 字符串（如 "Female"）
   - 多选题: 字符串数组（如 ["Apple", "Samsung"]）
   - 矩阵题: 字符串（如 "Never"）

3. 矩阵题强制约束：
   - 每行输出一个 subject
   - 行与行之间推荐不同选项

**【输出格式 - 必须严格遵循】**

直接输出以下 JSON 结构，不要任何其他内容：
{
  "interactions": [
    {
      "questionType": "<<<FROM_HTML:q1_type>>>",
      "subject": "<<<FROM_HTML:q1_row1>>>（<<<FROM_HTML:q1_col1>>>）",
      "showAnswer": "<<<FROM_HTML:q1_col1>>>"
    }
  ],
  "page_summary": "<<<FROM_HTML:page_summary>>>"
}

**【重要约束】**
1. 只输出 interactions 数组，不要包含 survey_page 或其他 YAML 结构
2. 不要输出任何 YAML 格式的内容
3. page_summary 必须是英文句子

---

**YAML 结构（仅格式参考，禁止复制到输出中）**', 'YAML答案推荐：用户提示前缀（ANSWER_USER_PROMPT_PREFIX）', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (185, 'yaml_structure.template_to_yaml_system', 'yaml_structure', NULL, '你是一个问卷数据生成助手。

你的职责：
- 根据提供的 YAML 模板结构（包含占位符）
- 结合当前 HTML 内容中的实际数据
- 生成包含最新数据的完整 YAML

重要规则：
- 严格保持模板的结构格式不变
- 用 HTML 中的实际题目文本、选项等数据替换占位符
- 不得遗漏任何题目
- 不得改变题目的类型和顺序

【重要】字符编码约束：
- 只使用 ASCII 标准字符输出 YAML
- 所有引号统一使用双引号 "
- 禁止使用 Unicode 弯引号

输出格式：
- 直接输出 YAML，不要任何解释文字
- 不要 markdown 代码块包裹', '模板转YAML：系统提示词（TEMPLATE_TO_YAML_SYSTEM_PROMPT）', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (186, 'yaml_structure.template_to_yaml_user', 'yaml_structure', NULL, '---

**任务说明**

请根据以下 YAML 模板结构（包含占位符），结合当前 HTML 内容，生成最新的 YAML 数据。

---

**YAML 模板结构（参考格式）**

{yamlTemplate}

---

**当前 HTML 内容**

{html}

---

**生成要求**

1. 严格按照模板的结构格式输出
2. 用 HTML 中的实际数据替换占位符
3. 确保所有题目都被正确提取
4. 保持题目的原始顺序
5. 不得凭空添加或删除任何题目

直接输出 YAML 数据：', '模板转YAML：用户提示（TEMPLATE_TO_YAML_USER_PROMPT）', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (187, 'yaml_prompts.system_prefix', 'yaml_prompts', NULL, '你是一个问卷YAML结构提取助手。
请严格从HTML中提取所有题目信息，100%来源于文件，不得添加修改。
重要：所有动态内容必须使用占位符格式，禁止输出实际数据值。
', 'YAML提取：系统前缀', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (188, 'yaml_prompts.multi_question_rules', 'yaml_prompts', NULL, '【多题页面处理规则】
- 每个fieldset是一道题
- 统一收录到questions列表
- 示例YAML结构:
  survey_page:
    fingerprint: "<<<FROM_HTML:fingerprint>>>"
    title: "<<<FROM_HTML:title>>>"
    page_metadata:
      has_next_button: "<<<FROM_HTML:has_next_button>>>"
      has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
      is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
      is_description_page: "<<<FROM_HTML:is_description_page>>>"
    questions:
      - title: "<<<FROM_HTML:title>>>"
        questions_type: "<<<FROM_HTML:questions_type>>>"
        ...
', 'YAML提取：多题页面处理规则', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (189, 'yaml_prompts.question_type_rules', 'yaml_prompts', NULL, '【题型识别规则】
选择类: single/multi/select/image
文本类: open_text/email/phone/number/date/time/open_end
矩阵类: matrix_radio/matrix_checkbox
量表类: likert/semantic_diff/slider/nps/likert_card
排序类: multi_select/ranking_drag/ranking_select
分类类: cardsort_drag/cardsort_select
特殊类: image_select/heatmap/max_diff/conjoint
其他类: mixed/instruction/unknown
', 'YAML提取：32种题型识别规则', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (190, 'yaml_prompts.field_extraction_rules', 'yaml_prompts', NULL, '【字段提取规则】
- title: 题目文本（去除题号和星号）
- questions_type: 从32枚举中选择（不是type）
- required: true/false
- 各类题型额外字段（见题型差异化输出）
', 'YAML提取：字段提取规则', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (191, 'yaml_prompts.question_type_differential', 'yaml_prompts', NULL, '【题型差异化输出】
single/multi/select/image → choices, columns
matrix_radio/checkbox → columns, rows
likert → columns, rows
slider/nps → min, max, min_label, max_label
ranking_drag/select → rank_items
cardsort_drag/select → cards, categories
open_text/email → input_type, placeholder
', 'YAML提取：题型差异化输出', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (192, 'yaml_prompts.response_format_rules', 'yaml_prompts', NULL, '【response_format字段】
- type: single_select/single_per_row/multi_select/matrix等
- orientation: 方向（horizontal/vertical/null）
- option_sort: 排序方式（as_appearing/random/null）
- max_selections: 多选上限（数字/null）
', 'YAML提取：response_format字段规则', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (193, 'yaml_prompts.prohibitions', 'yaml_prompts', NULL, '【严格禁止】
- 遗漏题目
- 使用枚举外type
- 凭空添加选项
- 修改原始文本
- 改变选项顺序
- 主观猜测题型
', 'YAML提取：严格禁止行为', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (194, 'yaml_prompts.output_format_rules', 'yaml_prompts', NULL, '【输出格式】
- 直接输出YAML，不要markdown包裹
- 不要任何解释文字
- 不要```yaml代码块
- 重要：题目类型字段必须命名为 questions_type，不是 type

【占位符格式】
所有动态内容必须使用占位符 `<<<FROM_HTML:xxx>>>` 格式，禁止输出实际数据值：
- title → "<<<FROM_HTML:title>>>"
- columns → "<<<FROM_HTML:col1>>>", "<<<FROM_HTML:col2>>>" 等
- rows → "<<<FROM_HTML:row1>>>", "<<<FROM_HTML:row2>>>" 等
- options → "<<<FROM_HTML:opt1>>>", "<<<FROM_HTML:opt2>>>" 等
- has_next_button → "<<<FROM_HTML:has_next_button>>>"
- has_submit_button → "<<<FROM_HTML:has_submit_button>>>"
- is_thank_you_page → "<<<FROM_HTML:is_thank_you_page>>>"
- is_description_page → "<<<FROM_HTML:is_description_page>>>"
- required → "<<<FROM_HTML:required>>>"

【示例YAML结构】
```yaml
survey_page:
  title: "<<<FROM_HTML:title>>>"
  page_metadata:
    has_next_button: "<<<FROM_HTML:has_next_button>>>"
    has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
    is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
    is_description_page: "<<<FROM_HTML:is_description_page>>>"
  questions:
    - title: "<<<FROM_HTML:title>>>"
      questions_type: "<<<FROM_HTML:questions_type>>>"
      required: "<<<FROM_HTML:required>>>"
      options:
        - "<<<FROM_HTML:opt1>>>"
        - "<<<FROM_HTML:opt2>>>"
```

【页面属性输出】
每个survey_page的YAML必须包含page_metadata节点：
- has_next_button: true/false (是否有"下一页"按钮)
- has_submit_button: true/false (是否有"提交"按钮)
- is_thank_you_page: true/false (是否是感谢页)
- is_description_page: true/false (是否是无题目说明页，仅有说明文字无表单输入)
', 'YAML提取：输出格式要求', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (195, 'yaml_prompts.page_detection_rules', 'yaml_prompts', NULL, '【页面属性检测规则】
从HTML内容中识别以下页面属性，返回占位符格式：

has_next_button: "<<<FROM_HTML:has_next_button>>>"
- 当HTML中存在"Next"、"下一页"、"Continue"、"下一步"等导航按钮时为 true
- 否则为 false

has_submit_button: "<<<FROM_HTML:has_submit_button>>>"
- 当HTML中存在"Submit"、"提交"、"Finish"、"完成"等提交按钮时为 true
- 否则为 false

is_thank_you_page: "<<<FROM_HTML:is_thank_you_page>>>"
- 当页面内容包含"Thank you"、"谢谢"、"感谢"、"提交成功"等感谢信息时为 true
- 通常出现在问卷最后或提交后

is_description_page: "<<<FROM_HTML:is_description_page>>>"
- 当页面没有任何题目（无fieldset、无input、无select）时为 true
- 只有说明文字、图片等非题目内容，无任何可交互表单元素时为 true
- 包含：问卷说明、免责声明、隐私政策、答题指引等
', 'YAML提取：页面属性检测规则', NULL, 1, 1, '2026-05-27 14:38:44', '2026-05-27 14:38:44');
INSERT INTO "prompts" ("id", "prompt_key", "prompt_type", "question_type", "content", "description", "yaml_template", "version", "is_active", "created_at", "updated_at") VALUES (196, 'system.type_detector', 'system', NULL, '你是一个问卷题型识别助手。
从以下候选题型列表中，识别当前HTML页面包含哪些题型。

候选列表：{question_types}

规则：
1. 只能从候选列表中选择，不得添加候选列表之外的值
2. 页面有多个题型时全部列出
3. 返回格式严格为JSON数组，例如：["matrix_radio"] 或 ["matrix_radio","single"]
4. 不需要解释，只返回JSON数组', NULL, NULL, 1, 1, '2026-05-27 16:39:26', '2026-05-27 16:39:26');
