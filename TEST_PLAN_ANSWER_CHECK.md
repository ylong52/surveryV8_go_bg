# Answer Check 工具 - 测试方案与预期效果

## 📋 工具概述
- **工具名称**: Answer Check Simulator
- **入口文件**: `cmd/answer-check/main.go`
- **功能**: 从问卷 HTML 中自动提取问题，调用 AI 获取推荐答案，并通过交互式 CLI 进行模拟答题和答案验证
- **输出**: 将答案持久化到数据库 `user_submitted_answers` 表

---

## 🎯 核心功能清单

| # | 功能模块 | 实现文件 | 状态 |
|---|---------|---------|------|
| 1 | 参数解析 + HTML 文件输入 | `cmd/answer-check/main.go` | ✅ |
| 2 | HTML 清洗 + PageID 提取 | 复用 `core/cleaner/` | ✅ |
| 3 | 数据库初始化 | 复用 `core/database/` | ✅ |
| 4 | AI 推荐答案生成 | 复用 `core/extractor/GenerateRealtimeAnswerOnlyWithMeta` | ✅ |
| 5 | YAML options[] 解析 | `core/simulator/yaml_parser.go` | ✅ |
| 6 | 交互式答题引擎 | `core/simulator/interactive_simulator.go` | ✅ |
| 7 | 答案持久化 | 复用 `core/database/UserSubmittedAnswerRepo` | ✅ |
| 8 | 结果摘要输出 | `cmd/answer-check/main.go` | ✅ |

---

## 🔧 使用方式

### 基础命令

```bash
# 方式1: 最简单，使用默认参数
cd e:\case2025\adspower\surveyV8_go\surveryV8_go_bg
go run cmd/answer-check/main.go
# → 提示拖入 HTML 文件

# 方式2: 指定 HTML 文件
go run cmd/answer-check/main.go -html="path\to\survey.html"

# 方式3: 自定义 survey_id 和 session_id
go run cmd/answer-check/main.go \
  -html="path\to\survey.html" \
  -survey_id="my_survey_001" \
  -session="my_session_001"
```

### 参数说明

| 参数 | 默认值 | 说明 |
|-----|--------|------|
| `-html` | (提示输入) | HTML/TXT 文件路径 |
| `-survey_id` | `test_0002` | 问卷标识 |
| `-session` | `session_id_test0002` | 会话 ID |

---

## 📊 执行流程与预期输出

### 流程步骤

```
1️⃣  参数验证
    ↓ 检查 HTML 文件存在性和格式
    
2️⃣  数据库初始化
    ↓ 连接 SQLite，验证 user_submitted_answers 表
    
3️⃣  HTML 加载 & 清洗
    ↓ 读取原始 HTML，移除样式/脚本，保留结构
    
4️⃣  PageID 提取
    ↓ 从清洗后 HTML 中识别页面标识符
    
5️⃣  AI 推荐答案生成
    ↓ 调用 GenerateRealtimeAnswerOnlyWithMeta()
    ↓ 返回 RealtimeAnswerOnlyResult（含 interactions + YamlStructure）
    
6️⃣  YAML options[] 解析
    ↓ ParseYamlOptions() 从 YamlStructure 提取选项
    ↓ 构建 label → index 映射
    
7️⃣  交互式答题
    ↓ 逐题显示问题 + AI 推荐答案
    ↓ 用户确认或修改答案
    
8️⃣  答案持久化
    ↓ 将答案插入 user_submitted_answers 表
    
9️⃣  结果摘要
    ↓ 显示完成统计、所有答案总结
```

### 预期输出示例

```
═══════════════════════════════════════════════════════════════════
📋 问卷答案检查工具 (Answer Check Simulator)
═══════════════════════════════════════════════════════════════════

📄 请拖入 HTML 或 TXT 文件，然后按回车:
> e:\case2025\adspower\surveyV8_go\surveryV8_go_bg\docs\htmls\test_single.html

✅ 使用文件: e:\case2025\adspower\surveyV8_go\surveryV8_go_bg\docs\htmls\test_single.html
   Survey ID: test_0002
   Session ID: session_id_test0002

📄 读取 HTML 文件... ✅ (45.2 KB)
   清洗后: 38.5 KB

🔍 提取 PageID... ✅ page_12345

🤖 调用 AI 获取推荐答案... ✅ (3 题)

───────────────────────────────────────────────────────────────────
📌 AI 推荐答案摘要
───────────────────────────────────────────────────────────────────
[Q1] What is your current country of residence?
     推荐: 男 (类型: single)

[Q2] Are you Spanish, Hispanic, or Latinx?
     推荐: ['Yes', 'No'] (类型: multi)

[Q3] Please enter your zip code
     推荐: "12345" (类型: text)


📋 开始交互式答题模拟 (3 题)
─────────────────────────────────────────────────────────────────

[Q1] What is your current country of residence?
推荐答案: 男 (题型: single)
  → [0] 男
  [1] 女
  [2] 不知道

请输入选项序号 [默认: 0] 或按回车确认: 
> [用户按回车或输入选项号]

[Q2] Are you Spanish, Hispanic, or Latinx?
推荐答案: ['Yes', 'No'] (题型: multi)
  [0] Yes
  [1] No
  [2] Maybe

请输入选项序号，多个用逗号分隔 (例: 0,2,3)，或按回车使用推荐答案:
推荐: ['Yes', 'No']
>

[Q3] Please enter your zip code
推荐答案: "12345"
请输入或按回车使用推荐答案: 
>

─────────────────────────────────────────────────────────────────
✅ 共收集 3 题答案

✅ 答案持久化: 已保存 3 题到 user_submitted_answers 表

═══════════════════════════════════════════════════════════════════
✅ 答题完成
═══════════════════════════════════════════════════════════════════
会话 ID: session_id_test0002
完成度: 3/3 题

📊 交互结果摘要
─────────────────────────────────────────────────────────────────
[Q1] What is your current country of residence?
     答案: 男
     选项ID: 0

[Q2] Are you Spanish, Hispanic, or Latinx?
     答案: 0,1

[Q3] Please enter your zip code
     答案: 12345
─────────────────────────────────────────────────────────────────

✨ 所有答案已保存到数据库
```

---

## 🧪 测试用例

### 测试用例1: 单选题 (Single Choice)

**测试文件**: 任何包含单选题的 HTML

**预期效果**:
- ✅ 正确解析单选选项
- ✅ 显示 AI 推荐选项（用 `→` 标记）
- ✅ 用户输入选项号时正确映射
- ✅ 用户按回车时使用推荐答案
- ✅ 答案持久化: `answer=标签文本, answer_id=选项索引`

**测试步骤**:
```bash
go run cmd/answer-check/main.go -html="<single_choice_html>"
# 输入: [按回车] 使用推荐答案
# 或输入: 1, 2, 3 等选择不同选项
```

---

### 测试用例2: 多选题 (Multi Choice)

**测试文件**: 任何包含多选题的 HTML

**预期效果**:
- ✅ 解析多个选项
- ✅ 显示选项列表（[0], [1], [2] ...）
- ✅ 用户可输入多个序号 (用逗号分隔)
- ✅ 答案持久化为逗号分隔的序号列表

**测试步骤**:
```bash
# 输入推荐答案
go run cmd/answer-check/main.go -html="<multi_choice_html>"
# 在交互中输入: 0,2 (选择第0和第2选项)
```

---

### 测试用例3: 填空题 (Text Input)

**测试文件**: 任何包含文本输入的 HTML

**预期效果**:
- ✅ 显示 AI 推荐文本
- ✅ 用户输入自定义答案或按回车使用推荐
- ✅ 答案持久化为原始文本

**测试步骤**:
```bash
go run cmd/answer-check/main.go -html="<text_input_html>"
# 输入推荐答案或自定义输入
```

---

### 测试用例4: 矩阵题 (Matrix)

**测试文件**: 包含矩阵表格的 HTML

**预期效果**:
- ✅ 识别矩阵结构（行 × 列）
- ✅ 显示推荐答案
- ✅ 答案持久化: `row_label=行标签, col_label=列标签`

**测试步骤**:
```bash
go run cmd/answer-check/main.go -html="<matrix_html>"
# 确认或修改推荐答案
```

---

## ✨ 高级特性

### 1. YAML Options 解析

从 `YamlStructure` 中自动提取 options[] 数组:

```go
// yaml_parser.go: ParseYamlOptions()
// 输入:
//   response_format:
//     options:
//       - label: "男"
//         value: "1"
//       - label: "女"
//         value: "2"
// 输出: []SimOption{
//   {Index: 0, Label: "男", Value: "1"},
//   {Index: 1, Label: "女", Value: "2"},
// }
```

### 2. label → index 映射

通过 `FindOptionByLabel()` 快速定位推荐答案对应的选项索引

### 3. 交互式 CLI

- 纯标准库实现 (`bufio`, `fmt`)
- 无第三方依赖
- 支持 skip 命令跳过题目
- 即时显示验证结果

### 4. 答案持久化

自动调用 `UserSubmittedAnswerRepo.Upsert()`:
- 自动生成 `answer_id`
- 记录 `timestamp` (Unix millisecond)
- 支持 `row_label`, `col_label` (矩阵题)
- 唯一约束: `session_id + page_results_id + question + row_label + col_label`

---

## 🛠️ 代码结构

```
cmd/answer-check/
└── main.go
    ├── promptForHTMLFile()       # 交互式文件输入
    ├── validateHTMLFile()        # 文件验证
    ├── persistAnswers()          # 答案持久化
    └── main()                    # 流程编排

core/simulator/
├── yaml_parser.go
│   ├── ParseYamlOptions()        # YAML 选项解析
│   ├── FindOptionByLabel()       # label → index 查询
│   └── FormatOptionDisplay()     # 选项格式化输出
│
├── interactive_simulator.go
│   ├── RunSimulation()           # 启动交互式模拟
│   ├── handleSingleChoice()      # 单选题处理
│   ├── handleMultiChoice()       # 多选题处理
│   ├── handleMatrixChoice()      # 矩阵题处理
│   ├── handleTextInput()         # 填空题处理
│   └── PrintSummary()            # 结果摘要输出
│
└── text_generator.go
    ├── GenerateText()            # 文本生成（未来用）
    ├── ValidateTextLength()      # 长度验证
    └── SuggestTextLength()       # 长度推荐

复用模块:
├── core/cleaner/
│   ├── StripForDirectSurvey()   # HTML 清洗
│   └── ExtractPageIDWithPatterns() # PageID 提取
│
├── core/extractor/
│   └── GenerateRealtimeAnswerOnlyWithMeta() # AI 推荐答案
│
└── core/database/
    ├── Init()                   # 数据库初始化
    ├── InitSiteCache()          # Site 缓存初始化
    └── UserSubmittedAnswerRepo.Upsert() # 答案持久化
```

---

## 🐛 错误处理

| 场景 | 处理方式 |
|------|---------|
| 文件不存在 | 显示错误并退出 (EXIT 1) |
| 数据库初始化失败 | 显示错误并退出 (EXIT 1) |
| PageID 提取失败 | ⚠️ 警告，使用默认 ID 继续 |
| AI 调用失败 | 显示错误并退出 (EXIT 1) |
| 答案持久化失败 | ⚠️ 警告，显示摘要但标记失败 |
| 无效用户输入 | 显示提示，使用推荐答案重试 |

---

## 📈 后续优化方向

1. **批量处理**: 支持一次性处理多个 HTML 文件
2. **答案验证**: 整合 `core/validator/` 对答案进行 humanize 检查
3. **缓存加速**: 复用已生成的推荐答案，减少 AI 调用
4. **交互改进**: 支持更多快捷键 (n=next, p=previous, e=edit last)
5. **数据导出**: 支持将答案导出为 CSV/JSON
6. **WebUI**: 基于 Web 的交互界面（可选）

---

## ✅ 验收标准

- [x] 代码编译无误
- [x] 默认参数正确
- [x] HTML 文件验证正确
- [x] 数据库初始化正确
- [x] HTML 清洗 + PageID 提取正确
- [x] AI 推荐答案获取正确
- [x] YAML options[] 解析正确
- [x] 交互式答题流程正确
- [x] 答案持久化正确
- [x] 结果摘要输出正确
- [x] 错误处理合理

---

## 📝 变更日志

| 版本 | 日期 | 内容 |
|------|------|------|
| 1.0 | 2026-06-07 | 初始版本完成 |

