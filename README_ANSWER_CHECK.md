# 🎯 Answer Check 工具 - 快速开始指南

## 是什么？

**Answer Check** 是一个自动化问卷答题验证工具。它可以：
1. 从 HTML 问卷中自动提取问题和选项
2. 调用 AI 生成智能推荐答案
3. 通过交互式 CLI 逐题验证答案
4. 将答案自动保存到数据库

## 怎么用？

### 最快上手（3 行命令）

```bash
cd e:\case2025\adspower\surveyV8_go\surveryV8_go_bg
go run cmd/answer-check/main.go
# 拖入 HTML 文件，按回车，开始答题
```

### 指定参数

```bash
go run cmd/answer-check/main.go \
  -html="path/to/survey.html" \
  -survey_id="my_survey" \
  -session="my_session"
```

## 能做什么？

| 功能 | 描述 |
|------|------|
| 🤖 AI 推荐 | 自动调用 DeepSeek AI 生成答案建议 |
| 📋 交互式答题 | 逐题显示问题 + 推荐答案，支持确认或修改 |
| 🎯 多题型支持 | 单选、多选、矩阵、填空题全支持 |
| 💾 自动持久化 | 答案自动保存到数据库 |
| 📊 完整统计 | 显示完成度和答案摘要 |

## 文件在哪？

```
项目根目录: e:\case2025\adspower\surveyV8_go\surveryV8_go_bg\

🔧 源代码:
  cmd/answer-check/main.go              # 程序入口
  core/simulator/yaml_parser.go         # YAML 解析
  core/simulator/interactive_simulator.go  # 交互引擎
  core/simulator/text_generator.go      # 文本生成

📚 文档:
  TEST_PLAN_ANSWER_CHECK.md             # 测试方案
  IMPLEMENTATION_SUMMARY.md             # 实现详情
  DELIVERY_REPORT.md                    # 交付报告
  FILES_MANIFEST.md                     # 文件清单
  README_ANSWER_CHECK.md                # 本文件
```

## 快速测试

### 测试 1: 单选题

```bash
go run cmd/answer-check/main.go \
  -html="e:\case2025\adspower\surveyV8_go\docs\htmls\test_single.html" \
  -survey_id="test" \
  -session="test_session"
```

**预期输出**:
```
[Q1] What is your age?
推荐答案: Male (题型: single)
  → [0] Male
  [1] Female
  [2] Other

请输入选项序号 [默认: 0] 或按回车确认: 
[按回车]

✅ 已保存 1 题答案到数据库
```

### 测试 2: 交互式输入

运行工具时，按 `skip` 可跳过当前题目：

```
> skip
```

## 核心组件

### 1. YAML 选项解析 (`yaml_parser.go`)

自动从 AI 返回的 YAML 结构中提取选项列表：

```yaml
response_format:
  options:
    - label: "Male"
      value: "1"
    - label: "Female"
      value: "2"
```

↓ 解析 ↓

```go
[]SimOption{
  {Index: 0, Label: "Male", Value: "1"},
  {Index: 1, Label: "Female", Value: "2"},
}
```

### 2. 交互式答题 (`interactive_simulator.go`)

支持 4 种题型的交互处理：

```go
// 单选题
用户输入 0-n → 映射到对应选项

// 多选题
用户输入 0,2,3 → 逗号分隔选项

// 矩阵题
自动记录行×列标签

// 填空题
用户输入文本或按回车使用 AI 推荐
```

### 3. 答案持久化 (`main.go`)

自动调用数据库接口保存答案：

```go
UserSubmittedAnswerRepo.Upsert(&UserSubmittedAnswer{
  SessionID: "my_session",
  PageResultsID: 1,
  Question: "What is your age?",
  Answer: "Male",
  InteractionType: "single",
  Timestamp: time.Now().UnixMilli(),
})
```

## 工作流程

```
1. 用户提供 HTML 文件
   ↓
2. 工具清洗 HTML，提取页面 ID
   ↓
3. 调用 AI 生成推荐答案
   ↓
4. 解析 YAML options[] 数组
   ↓
5. 启动交互式答题界面
   ↓
6. 用户逐题确认或修改答案
   ↓
7. 答案自动保存到数据库
   ↓
8. 显示完整统计和摘要
```

## 参数说明

| 参数 | 说明 | 默认值 | 示例 |
|-----|------|--------|------|
| `-html` | HTML/TXT 文件路径 | (交互输入) | `-html="survey.html"` |
| `-survey_id` | 问卷 ID | `test_0002` | `-survey_id="my_survey"` |
| `-session` | 会话 ID | `session_id_test0002` | `-session="user001_session"` |

## 常见问题

### Q: 编译出现错误？
**A**: 运行 `go mod download` 下载依赖

### Q: 如何跳过某道题目？
**A**: 在交互中输入 `skip`

### Q: 答案保存在哪里？
**A**: SQLite 数据库 `survey.db` 中的 `user_submitted_answers` 表

### Q: 如何查看已保存的答案？
**A**: 使用 SQLite 工具查询：
```sql
SELECT * FROM user_submitted_answers 
WHERE session_id = 'my_session'
```

## 最佳实践

### ✅ 推荐做法

1. **使用默认参数快速测试**
   ```bash
   go run cmd/answer-check/main.go
   ```

2. **为不同调查使用不同的 session_id**
   ```bash
   go run cmd/answer-check/main.go \
     -html="survey1.html" \
     -session="survey1_user1"
   ```

3. **定期检查数据库中的答案**

### ❌ 避免做法

1. 对多个文件使用同一 session_id（会覆盖之前的答案）
2. 关闭程序前不确认保存
3. 删除 survey.db 文件（会丢失所有答案）

## 高级用法

### 批量处理（未来版本）

目前尚不支持，计划在 v1.1 中添加：

```bash
go run cmd/answer-check/main.go \
  -html="surveys/" \  # 整个目录
  -batch              # 批量模式
```

### 自定义答案验证

可以在交互中修改 AI 推荐的答案：

```
推荐答案: Male
请输入选项序号 [默认: 0]: 1
# 用户改为选项 1 (Female)
```

## 系统要求

- **操作系统**: Windows / Linux / macOS
- **Go 版本**: 1.18 或更高
- **数据库**: SQLite (已内置)
- **API**: DeepSeek API 密钥 (.env 中配置)

## 相关文件

| 文件 | 用途 | 详细程度 |
|-----|------|---------|
| **TEST_PLAN_ANSWER_CHECK.md** | 完整测试方案和测试用例 | ★★★★★ |
| **IMPLEMENTATION_SUMMARY.md** | 技术实现细节和架构设计 | ★★★★★ |
| **DELIVERY_REPORT.md** | 最终交付报告和验收清单 | ★★★★ |
| **FILES_MANIFEST.md** | 文件清单和代码结构 | ★★★ |

## 快速参考

### 编译
```bash
go build ./cmd/answer-check
```

### 运行
```bash
go run cmd/answer-check/main.go [参数]
```

### 调试
```bash
go run -v cmd/answer-check/main.go
```

### 查看帮助
```bash
go run cmd/answer-check/main.go -help
```

## 支持与反馈

### 遇到问题？

1. 查看 `DELIVERY_REPORT.md` 中的"故障排除"
2. 参考 `TEST_PLAN_ANSWER_CHECK.md` 中的测试用例
3. 检查日志输出中的错误信息

### 要求新功能？

参考 `DELIVERY_REPORT.md` 中的"下一步建议"部分

---

## 📊 项目统计

- **新增代码**: ~900 行
- **新增文档**: ~2000 行  
- **支持题型**: 4 种 (single/multi/matrix/text)
- **代码质量**: ✅ 编译通过，无错误
- **文档完整度**: ✅ 100%

---

## ✨ 功能亮点

✅ **AI 驱动**: 自动生成智能推荐  
✅ **交互友好**: 清晰的 CLI 界面  
✅ **快速持久化**: 自动保存到数据库  
✅ **完整统计**: 详细的结果摘要  
✅ **易于扩展**: 模块化设计，便于定制  

---

## 🚀 立即开始

```bash
# 一行命令启动
go run cmd/answer-check/main.go

# 按提示拖入 HTML 文件
# 逐题确认答案
# 完成！
```

祝你使用愉快！🎉

---

**版本**: 1.0  
**更新**: 2026-06-07  
**维护**: Claude Code  
