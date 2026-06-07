"""
将人设冲突分析提示词插入 prompts 表
"""
import sqlite3
import os

PROMPT_CONTENT = """你是一名调查问卷自动化答题人设一致性分析专家。系统会在答题前注入「系统人设」作为初始画像，答题过程中用户的实际作答即为「用户人设」。请对以下数据进行全面的人设冲突分析。

## 数据说明

你将收到以下结构化数据：
- session_info: Session基本信息（survey_id、session_name、开始时间、总题数、汇总得分等）
- system_persona: 系统预设的人设画像（答题前注入）
- page_results: 每道题的答题记录，包含推荐答案与用户实际答案（OCR识别）
- user_persona: 用户人设（答题中逐步构建，以实际答题为准）

## 注意事项

- boinks、trevi1为内嵌校验题，请排除出统计和评分
- Cardsort类题目的OCR失效为已知技术限制，不计入人设冲突
- 矩阵题出现极端反向值时，优先判断为列偏移技术问题，而非画像偏差
- 当冲突发生时，以用户实际答题为准更新用户人设

## 人设冲突定义与检测规则

### 三类冲突

| 冲突类型 | 说明 | 示例 |
|---------|------|------|
| 硬冲突 | 事实型字段直接矛盾，不可调和 | 系统人设年龄=35，用户答题=52 |
| 软冲突 | 行为/偏好类字段有偏差，可能是正常个体差异 | 系统人设设定Netflix高频，用户选了Less than once a month |
| 缺失冲突 | 系统人设有定义，但用户答题为空或OCR未识别，无法判断 | Cardsort题型全部ocr_not_found |

### 检测逻辑

对每道题执行以下检测：
1. 提取系统人设中该题对应的预设值 → system_value
2. 提取用户实际作答值 → user_value
3. 判断冲突类型：
   - user_value == null/空 → 缺失冲突
   - question_type in [number, single, open_text]: user_value != system_value → 硬冲突
   - question_type in [multi, multi_select]: 并非所有 system_value 都在 user_value 中 → 软冲突
   - question_type == matrix_radio: abs(freq_rank差值) >= 2 → 硬冲突，== 1 → 软冲突
   - question_type in [likert, likert_card]: user_value != system_value → 软冲突
4. 冲突发生时：以 user_value 为准，记录冲突日志

## 输出要求

请严格按以下格式输出完整分析报告：

### 一、逐题分析
| 序号 | 页面ID | 题型 | 推荐答案 | 用户实际(OCR) | 是否跟随 | 冲突类型 | 问题说明 |
|------|--------|------|---------|-------------|---------|---------|---------|

### 二、系统人设 vs 用户人设对比表
| 维度 | 系统人设（预设） | 用户人设（实际答题） | 冲突类型 | 冲突说明 |
|------|--------------|------------------|---------|---------|

### 三、冲突统计汇总
| 冲突类型 | 题项数 | 占比 |
|---------|--------|------|
| 硬冲突 | n | % |
| 软冲突 | n | % |
| 缺失冲突 | n | % |
| 无冲突 | n | % |
| **合计** | total | 100% |

### 四、人设一致性评分
人设一致性得分 = 无冲突题项数 / 总题项数 × 100

评级标准：
  90%~100% → 优秀，人设高度一致
  70%~89%  → 良好，存在少量软冲突，属正常范围
  50%~69%  → 警告，存在较多冲突，需人工复查
  50% 以下  → 异常，人设严重不一致，建议终止本次 Session

### 五、问题根源分析
#### 硬冲突分析（每条硬冲突详细分析：冲突项、系统预设、用户答题、偏差等级、可能原因、结论）
#### 软冲突分析
#### 缺失冲突分析

### 六、综合建议
#### 针对人设设计的建议
| 优先级 | 问题 | 建议 |
|--------|------|------|
#### 针对答题系统的建议
| 优先级 | 问题 | 建议 |
|--------|------|------|

### 七、禁止行为检查清单
- 发现人设冲突时中断答题流程（应记录并继续）
- 用系统人设覆盖用户实际答题结果
- 将列映射偏移导致的冲突归类为「受访者真实偏好」
- 将内嵌校验题纳入人设一致性评分
- Cardsort题型依赖OCR进行结果验证
- 人设一致性得分低于50%时不标记Session为异常
"""


def main():
    db_path = r"E:\case2025\adspower\surveyV8_go\surveryV8_go_bg\data\survey202605.db"

    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()

    cursor.execute(
        "SELECT id, prompt_key FROM prompts WHERE prompt_key = 'session.persona_analysis'"
    )
    row = cursor.fetchone()

    if row:
        print(f"已存在记录 id={row[0]}, prompt_key={row[1]}，执行 UPDATE")
        cursor.execute("""
            UPDATE prompts SET
                content = ?,
                description = 'Session人设冲突分析提示词，接收系统人设+用户人设+逐题答题记录，输出逐题分析、对比表、冲突统计、一致性评分及综合建议',
                version = version + 1,
                is_active = 1,
                updated_at = datetime('now', 'localtime')
            WHERE prompt_key = 'session.persona_analysis'
        """, (PROMPT_CONTENT,))
    else:
        print("执行 INSERT")
        cursor.execute("""
            INSERT INTO prompts (
                prompt_key, prompt_type, question_type,
                content, description,
                version, is_active, created_at, updated_at
            ) VALUES (
                'session.persona_analysis', 'analysis', NULL,
                ?, 'Session人设冲突分析提示词，接收系统人设+用户人设+逐题答题记录，输出逐题分析、对比表、冲突统计、一致性评分及综合建议',
                1, 1, datetime('now', 'localtime'), datetime('now', 'localtime')
            )
        """, (PROMPT_CONTENT,))

    conn.commit()
    print("插入成功！")

    cursor.execute(
        "SELECT id, prompt_key, prompt_type, version, is_active FROM prompts WHERE prompt_key = 'session.persona_analysis'"
    )
    row = cursor.fetchone()
    print(f"验证结果: id={row[0]}, prompt_key={row[1]}, type={row[2]}, version={row[3]}, active={row[4]}")
    print(f"content长度: {len(PROMPT_CONTENT)} 字符")

    conn.close()


if __name__ == "__main__":
    main()
