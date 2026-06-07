# OCR 多模型投票升级 — 预期结果文档

## 一、涉及改动

**文件**: `core/extractor/screenshot_ocr.go`

**改动性质**: 在现有 OCR 流程中增加多模型投票层，保留分析比对步骤，`ws_server/main.go` 无需改动。

---

## 二、新流程全貌

```
截图上传
    │
    ▼
┌─────────────────────────────────────────────────┐
│  步骤1：多模型投票 OCR                           │
│                                                 │
│  imageBase64 ──┬── Qwen VL (qwen-vl-plus) ──┐   │
│                │                              │   │
│                └── Claude (claude-sonnet-4-6) ─┤   │
│                                               │   │
│         (goroutine 并行，等待两者完成)           │   │
│                                               │   │
│  ┌─ 逐题对齐 ─────────────────────────────────┐│   │
│  │  Qwen  →  第1题: "Strongly Agree"         ││   │
│  │  Claude →  第1题: "Strongly Agree"  ✓ 全票 ││   │
│  │  Qwen  →  第2题: "Somewhat Agree"         ││   │
│  │  Claude →  第2题: "Agree"          ✗ 需裁决 ││   │
│  └────────────────────────────────────────────┘│   │
│                                               │   │
│  ┌─ 投票裁决（最多5轮重试）───────────────────┐│   │
│  │  全票一致  → confirmed（无需重试）           ││   │
│  │  多数（>50%）→ majority → 重试存疑题        ││   │
│  │  平票        → uncertain → 重试             ││   │
│  │  重试后仍uncertain → human_review_required  ││   │
│  └────────────────────────────────────────────┘│   │
│                                                 │
│  输出: VotedItem[]（每题含 FinalAnswer + Status）│   │
│  日志: 投票过程写入 storage/logs/events/sys_*.log │
└─────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────┐
│  步骤2：推荐答案比对（使用投票后的 FinalAnswer）  │
│                                                 │
│  对每道题：                                      │
│    single/multi/dropdown → 代码归一化精确匹配     │
│    open_text              → DeepSeek 语义比对     │
│    likert_card            → 跳过（不比对）       │
│                                                 │
│  输出: analysisStep（overall: match/mismatch/unclear）│
└─────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────┐
│  数据库写入                                      │
│                                                 │
│  page_results.ocr_json  = 完整 OCRFullResult JSON │
│  page_results.ocr_verified = overall 汇总值      │
│  page_results.ocr_model    = "Qwen+Claude"       │
└─────────────────────────────────────────────────┘
```

---

## 三、预期输出示例

### 场景 A：普通问卷页（单选）

**输入**: 截图含 3 道题，用户分别选了 "Male"、"1985"、"Yes"

**OCR 步骤1 输出（每题投票结果）**:

| 题号 | 题干 | Qwen | Claude | 投票结果 | 最终答案 |
|------|------|------|--------|----------|---------|
| 1 | What is your gender? | "Male" | "Male" | ✅ confirmed | "Male" |
| 2 | What year were you born? | "1985" | "1985" | ✅ confirmed | "1985" |
| 3 | Are you Hispanic or Latino? | "Yes" | "No" | 🔶 majority | "Yes" |

**分析步骤2（与推荐答案比对）**:

假设推荐答案均为 "Male" / "1985" / "Yes"：

| 题号 | 推荐答案 | 用户选择 | 比对结果 |
|------|---------|---------|---------|
| 1 | "Male" | "Male" | ✅ match |
| 2 | "1985" | "1985" | ✅ match |
| 3 | "Yes" | "Yes" | ✅ match |

**最终 `ocrVerified`**: `"match"`

---

### 场景 B：矩阵问卷页

**输入**: 5 行 × 4 列矩阵（Strongly Disagree / Somewhat Disagree / Somewhat Agree / Strongly Agree），部分行两模型结果不同

**OCR 步骤1 输出**:

| 题号 | 题干 | Qwen | Claude | 投票结果 | 最终列号 | 最终列标签 |
|------|------|------|--------|----------|---------|-----------|
| 1 | "I feel safe..." | col 4 | col 4 | ✅ confirmed | 4 | Strongly Agree |
| 2 | "This excites me..." | col 3 | col 4 | 🔶 majority (重试解决) | 3 | Somewhat Agree |
| 3 | "This bores me..." | col 2 | col 2 | ✅ confirmed | 2 | Somewhat Disagree |
| 4 | "This upsets me..." | col 4 | col 3 | 🔶 majority (重试解决) | 4 | Strongly Agree |
| 5 | "For data quality..." | col 1 | col 1 | ✅ confirmed（陷阱题忠实记录） | 1 | Strongly Disagree |

**分析步骤2**:

| 题号 | 推荐列标签 | 用户列标签 | 比对结果 |
|------|---------|---------|---------|
| 1 | Strongly Agree | Strongly Agree | ✅ match |
| 2 | Somewhat Agree | Somewhat Agree | ✅ match |
| 3 | Somewhat Disagree | Somewhat Disagree | ✅ match |
| 4 | Somewhat Agree | Strongly Agree | ❌ mismatch |
| 5 | Strongly Disagree | Strongly Disagree | ✅ match |

**最终 `ocrVerified`**: `"mismatch"`（因为第4题不符）

---

### 场景 C：OCR 全模型失败

**输入**: 截图损坏或无法读取

**预期**:
- `ocrJSON` = `""`
- `ocrVerified` = `"unclear"`
- 数据库 `ocr_error` 字段记录错误信息
- 事件日志仍写入，标注 `ocr_type: "multi_model_voting"`

---

### 场景 D：部分模型失败（容错）

**输入**: Qwen 调用成功，Claude 调用失败（401/超时）

**预期**:
- 仍有 `VotedItem` 结果（单模型直接 confirmed）
- 日志标注 Claude 失败原因
- `ocrVerified` 仍基于可用模型出结果

---

## 四、`ocr_json` 字段内容结构

```json
{
  "ocr_step": {
    "model": "Qwen+Claude",
    "interactions": [
      {
        "subject": "What is your gender?",
        "question_type": "single",
        "user_selected": "Male",
        "note": ""
      },
      {
        "subject": "matrix_page",
        "question_type": "matrix_page",
        "user_selected": {
          "total_rows": 5,
          "total_cols": 4,
          "col_labels": ["Strongly Disagree", "Somewhat Disagree", "Somewhat Agree", "Strongly Agree"],
          "rows": [
            {"row_index": 1, "row_text": "I feel safe...", "selected_col": 4},
            ...
          ]
        },
        "note": ""
      }
    ]
  },
  "analysis_step": {
    "interactions": [
      {
        "subject": "What is your gender?",
        "question_type": "single",
        "recommended_answers": "Male",
        "user_selected": "Male",
        "followed_recommendation": true,
        "match_method": "code_exact"
      }
    ],
    "overall": "match"
  }
}
```

---

## 五、事件日志格式

文件: `storage/logs/events/sys_YYYYMMDDHHMMSS.log`

```
2026-05-31 21:30:00.000|screenshot_ocr_voting|sess_12345|{"ocr_type":"multi_model_voting","models":[{"model_name":"Qwen","elapsed_ms":1234,"parsed":{"page_type":"normal","items":[...]}},{"model_name":"Claude","elapsed_ms":2345,"parsed":{"page_type":"normal","items":[...]}}],"voting_summary":{"total":3,"status_counts":{"confirmed":2,"majority":1},"items":[{"index":1,"subject":"What is your gender?","final_answer":"Male","status":"confirmed","model_answers":{"Qwen":"Male","Claude":"Male"}},...]}}
```

每条日志为单行 JSON，包含：
- `models`: 各模型原始识别结果（含耗时）
- `voting_summary.items`: 每题的投票详情（含各模型原始答案、最终答案、状态）

---

## 六、数据库字段变化

| 字段 | 变化 |
|------|------|
| `page_results.ocr_json` | 新增 `ocr_step.interactions[].user_selected` 现在是投票后的最终答案 |
| `page_results.ocr_verified` | 值域不变：`match` / `mismatch` / `unclear` / `human_review_required` |
| `page_results.ocr_model` | 从 `"qwen-vl-plus"` 改为实际调用的模型名，如 `"Qwen+Claude"` |
| `page_results.ocr_error` | 不变 |

---

## 七、向后兼容性

- **`ws_server/main.go` 无需改动**，`OCRAndAnalyze` 函数签名保持不变
- `ocr_json` 字段结构向下兼容：`ocr_step.interactions` 格式不变（只是答案来源从单模型变为多模型投票结果）
- 数据库表结构不变
