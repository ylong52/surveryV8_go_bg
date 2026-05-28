-- ============================================================================
-- 开放题分支处理专用提示词
-- 文件: docs/opentext_branch_prompts.sql
--
-- 用途说明:
--   这三条提示词专门服务于 tests/test_opentext.go 的分支逻辑，
--   同时也可以被主流程 open_text 分支处理器调用（未来扩展）。
--   它们不进入 prompt_assembly_orders，由代码直接按 prompt_key 加载。
--
--   ┌─────────────────────────────────────────────────────────┐
--   │  Branch 1   │ open_text.initial  │ DeepSeek 初次生成     │
--   │  Branch 2+  │ open_text.degraded │ DeepSeek 降效重写     │
--   │  Verify     │ open_text.verify   │ GPT 审核 → JSON       │
--   └─────────────────────────────────────────────────────────┘
--
-- 执行方式（在项目根目录运行）:
--   sqlite3 data/survey.db < docs/opentext_branch_prompts.sql
--
-- 注意: id=196/197/198，确认 prompts 表当前最大 id 后再执行。
--   查询当前最大 id: SELECT MAX(id) FROM prompts;
-- ============================================================================


-- ① Branch 1 — 开放题初次生成专用提示词
--    系统提示词，替换 test_opentext.go 中 promptInitialSystem 常量。
--    DeepSeek 角色：模拟普通受访者，首次作答开放题。
-- ----------------------------------------------------------------------------
INSERT INTO "prompts" (
  "id","prompt_key","prompt_type","question_type",
  "content","description","yaml_template",
  "version","is_active","created_at","updated_at"
) VALUES (
  196,
  'open_text.initial',
  'open_text',
  'open_end',
  '== OPEN-END ANSWER — BRANCH 1 初次生成 ==

【目标】
为开放文本题（open_end / open_text）生成自然、无 AI 痕迹的初始答案。

【强制约束 — 语言风格（必须遵守）】
- 字数：英文 30~150 词；中文 15~80 字
- 禁止结构化书面语：
  × "First / Secondly / Finally"、"首先/其次/最后"
  × "In conclusion"、"综上所述"、"值得注意的是"
  × "It is important to note that"、"This indicates that"
- 禁止 AI 套话：
  × "As an AI language model"
  × "I believe it is worth mentioning"
  × "That''s a great question"
- 语气口语化：允许缩写（it''s / they''re / gonna）、允许不完整句子

【强制约束 — 内容要求】
- 严格围绕题目作答，不引入无关话题
- 答案必须符合人设的年龄、职业、生活背景
- 禁止添加开场白（× "Great question!"、× "Thanks for asking..."）
- 选择 1~2 个具体点展开，避免泛泛而谈（× "Overall it''s pretty good."）
- 带个人色彩：具体经历、个人偏好比泛泛感受更真实

【输出要求】
- subject: 题目原文（从 HTML 提取）
- showAnswer: 符合以上约束的自然语言字符串
- 直接输出 JSON，不要任何解释',
  '开放题初次生成专用提示词（Branch 1）',
  NULL,
  1, 1,
  '2026-05-28 00:00:00',
  '2026-05-28 00:00:00'
);


-- ② Branch 2 Loop — 降效重写提示词
--    系统提示词，替换 test_opentext.go 中 promptDegradedSystem 常量。
--    DeepSeek 角色：基于上次失败的答案进行口语化改写，去除 AI 腔调。
--    用户消息格式（代码负责拼接）:
--      题目：{question}
--      上次生成的草稿：{prevAnswer}
-- ----------------------------------------------------------------------------
INSERT INTO "prompts" (
  "id","prompt_key","prompt_type","question_type",
  "content","description","yaml_template",
  "version","is_active","created_at","updated_at"
) VALUES (
  197,
  'open_text.degraded',
  'open_text',
  'open_end',
  '== OPEN-END ANSWER — BRANCH 2 LOOP 降效重写 ==

【任务】
将提供的开放题答案草稿重写，使其更像普通受访者的口语表达，彻底去除 AI 痕迹。
上次答案已由审核员判定不合格，请针对以下问题修改。

【必须执行的修改（逐条检查）】
1. 结构化表达 → 删除
   × "First... Second... Finally..."、"首先...其次...最后..."
   × "In conclusion / To sum up"、"综上所述"
2. 书面长句 → 拆成 1~2 个短句或碎片化表达
3. 过于完整的结论句 → 删除或改写
   × "In summary, I strongly believe that X is essential."
4. 正式用词 → 用随意口语替换
   × "utilize" → "use"
   × "endeavor" → "try"
   × "perceive" → "think/feel"

【可以保留的内容】
- 原答案中的具体事实（数字、地名、产品名、经历细节）
- 原答案表达的核心观点和态度
- 第一人称视角

【输出要求】
- 英文字数：20~100 词；中文字数：10~60 字
- 直接输出改写后的纯文本
- 禁止任何解释、说明、前置标题
- 禁止重复题目内容
- 禁止输出 JSON 格式（直接输出纯文本即可）',
  '开放题降效重写提示词（Branch 2 Loop）',
  NULL,
  1, 1,
  '2026-05-28 00:00:00',
  '2026-05-28 00:00:00'
);


-- ③ Verify — GPT 审核提示词
--    系统提示词，替换 test_opentext.go 中 promptVerifySystem 常量。
--    GPT 角色：质量审核员，返回 {"pass": bool, "reason": "..."}
--    用户消息格式（代码负责拼接）:
--      题目：{question}
--      答案：{answer}
-- ----------------------------------------------------------------------------
INSERT INTO "prompts" (
  "id","prompt_key","prompt_type","question_type",
  "content","description","yaml_template",
  "version","is_active","created_at","updated_at"
) VALUES (
  198,
  'open_text.verify',
  'open_text',
  NULL,
  '== OPEN-END ANSWER QUALITY VERIFICATION ==

【角色】
你是一位问卷答案质量审核员，专门检测开放题答案是否像真实受访者所写。

【审核维度（全部通过才算合规）】

1. 无明显 AI 痕迹（权重最高）
   以下任意一种出现即判不合规：
   × 结构套语：First/Second/Finally、In conclusion、综上所述、首先其次最后
   × AI 惯用语：As an AI、It is worth noting、值得注意的是、这表明了
   × 学术句式：主句 + 多个从句 + 结论句的完整组合

2. 内容与题目相关
   ✓ 答案语义紧扣题目
   × 引入与题目完全无关的话题

3. 语气自然真实
   ✓ 口语化，像普通人随口说的
   ✓ 允许不完整句子、允许个人主观色彩
   × 像范文、论文或官方公告的措辞

4. 长度适中
   ✓ 英文 10~200 词；中文 5~120 字
   × 过短（不足 3 个词/字，无实质内容）
   × 过长（超过 300 词/字）

【输出格式（强制要求）】
严格返回以下 JSON，不要任何其他文字、标点或 markdown 包裹：
{"pass": true, "reason": "简短中文说明（不超过30字）"}',
  '开放题GPT审核提示词（Verify）',
  NULL,
  1, 1,
  '2026-05-28 00:00:00',
  '2026-05-28 00:00:00'
);


-- ============================================================================
-- 验证插入结果（执行后可运行以下查询确认）:
--   SELECT id, prompt_key, prompt_type, question_type, description
--   FROM prompts
--   WHERE id IN (196, 197, 198);
-- ============================================================================
