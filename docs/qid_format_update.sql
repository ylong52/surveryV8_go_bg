-- 更新 structure.output_format prompt，question_id 格式
UPDATE prompts SET content = '【Output Format — STRICT JSON（必须直接能被 API 后端解析）】

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
        "question_id": "q1",
        "question_type": "single",
        "subject": "题目文本（矩阵题："行文本（列文本）"）",
        "showAnswer": "推荐答案文本"
      }
    ],
    "questions": [
      {
        "question_id": "q1",
        "question_type": "single",
        "options": [
          { "opt_id": 1, "value": "a", "xpath": "//input[@id=''q1_opt1'']" },
          { "opt_id": 2, "value": "b", "xpath": "//input[@id=''q1_opt2'']" }
        ]
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
    "page_summary": "Summarize the main opinion and attitude of this page in one concise English sentence; merge repeated points; express user attitude or judgment directly; do not include original question text"
  }
}

【questions 字段规范 — XPath 配置下发（前端点击定位）】

★ 必须从 HTML 中提取每个选项的真实 XPath，不能硬编码！

【XPath 书写规则（XPath 2.0 兼容）】
1. 优先使用稳定属性定位（id > name > data-* > value）
2. 使用双斜杠 // 从文档任意位置匹配
3. 绝对不能在 xpath 中出现数字索引如 [1]、[2]（不稳定）
4. 选项 value 值必须从 HTML 表单元素的 value 属性提取
5. question_id 格式固定为 q1, q2, q3...（按 HTML 中题目出现顺序）
6. opt_id 格式为 1, 2, 3...（每个 question_id 内从 1 开始）
7. question_type 必须为标准题型之一：single / multi / dropdown / text / number / email / date / matrix_radio / matrix_checkbox / likert / likert_card / slider / nps / ranking_drag / ranking_select / multi_select / semantic_diff / open_text / open_end / instruction / mixed / unknown

【XPath 示例】
✓ 正确：
  //input[@id=''opt_male'']
  //input[@name=''q1'' and @value=''0'']
  //select[@id=''country'']/option[2]
  //textarea[@name=''feedback'']
  //div[@class=''star-rating'']/span[3]

✗ 错误：
  //div[1]/div[2]/span[1]    ← 禁止用数字索引
  //input[2]                   ← 禁止用数字索引

【选项提取规则】
- radio/checkbox: 遍历所有 input[type=radio] 或 input[type=checkbox]，提取每个的 @id/@name/@value/@data-* 生成 xpath
- select: 遍历 select[@name=xxx]/option，为每个 option 生成 xpath
- text/number/textarea: 无 options 数组（空数组 [] 或省略）
- value 值必须与 HTML 中表单元素的 value 属性一致，不能凭空生成

【questions 与 interactions 对应关系】
- questions[0] 对应 interactions[0]（按顺序一一对应）
- 每个 question 必须有对应的 interaction

【重要约束】
1. interactions 中只需返回 question_id、question_type、subject 和 showAnswer
2. question_id 格式必须为 q1, q2, q3...（与 questions 中的 question_id 完全一致）
3. questions 中的 xpath 必须与 interactions 中的题目一一对应
4. showAnswer 格式：单选/矩阵为字符串，多选为字符串数组
5. 禁止返回任何非 JSON 文字
6. questions 数组不能为空（即使是 text 题也要有 question 条目，options 为空数组）' WHERE id = 180;

-- 验证
SELECT prompt_key, length(content) as content_len FROM prompts WHERE id = 180;
