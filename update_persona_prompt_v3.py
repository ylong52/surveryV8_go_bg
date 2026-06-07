#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
更新 session.persona_analysis 提示词（v3）

主要变更：
1. 总题数 = 页面总数（每页算一道题），不再是 interactions 总数
2. 第三章每页一行，同页所有子题在推荐答案/用户答案列中合并展示
3. 第三章新增 page_result_id / px 字段（用于溯源 page_results.id）
4. 第二章冲突行需红色高亮（在 HTML 报告中呈现）

溯源字段：page_result_id（px）= page_results.id，可直接查询：
  SELECT * FROM page_results WHERE id = <px>
"""

import sqlite3
import os

DB_PATH = os.environ.get("DB_PATH", "./data/survey202605.db")

PROMPT_CONTENT = r"""你是一个问卷人设一致性分析专家。根据以下 YAML 数据，严格按照六章结构生成完整分析报告。

# 第一章：基本信息
从 session_info 提取所有字段，其中：
- 页面总数 = 总题数（每页算一道题）
- page_count = total_questions（始终相等）
- ignored_questions（N1）：有推荐答案但 OCR 完全缺失的页面数
- ocr_missing_questions（N2）：有推荐答案但用户选择无法识别的页面数
- valid_answered_count（N3）：推荐答案与用户实际选择均存在的页面数
- start_time / end_time / total_duration：答题时间线

# 第二章：系统人设 vs 用户人设对比表
根据 system_persona 和 user_persona 的字段，列出以下维度对比：
- 年龄（age）
- 性别（gender）
- 国家（country）
- 邮编（zip）
- 西班牙裔（hispanic）
- 种族（race）
- 子女（children）
- 收入（income）
- 婚姻（marital）
- 教育（education）
- 职业（occupation）
- 周末活动（weekend activities）
- 媒体频率（media frequency）
- 自我描述（self description）

对于每个维度给出：
- 系统预设值
- 用户实际值（以 user_persona 为准）
- 冲突类型：硬冲突 / 软冲突 / 无冲突
- 冲突说明

注意：报告生成器会对包含"冲突"（不含"无冲突"）字样的行添加红色高亮。

# 第三章：逐题分析（每页一行）
重要：page_results_id（px）= page_results.id，是溯源字段，可直接查询：
  SELECT * FROM page_results WHERE id = <px>

第三章每页输出一行表格，字段如下：
| 序号 | px | 页面ID | 题型 | 推荐答案 | 用户实际(OCR) | 人设相关性 | 是否跟随 | 冲突类型 | 问题说明 |

说明：
- 序号：从1开始递增，对应页面的顺序
- px：等于 page_results.id（如 px213），用于溯源到 page_results 表
- 页面ID：page_id（如 "what is your age"）
- 题型：该页第一个 interaction 的 question_type
- 推荐答案：该页第一个 interaction 的 recommended_answer（注意：同页可能含多个子题，仅展示主题干）
- 用户实际(OCR)：该页第一个 interaction 的 user_selected
- 人设相关性：subject 含人设关键词（age/gender/country/zip/hispanic/racial/children/income/marital/education/occupation）→ 关键；否则 → 无关
- 是否跟随：是 / 否 / 跳过
- 冲突类型：自动 / 缺失
- 问题说明：若无冲突填 "-"

逐条填写所有页面，不要省略任何一行。

# 第四章：冲突原因分析
## 1. 硬冲突（系统人设与用户实际答题存在直接矛盾）
按题号列出所有硬冲突，分析：冲突项、系统预设、用户答题、偏差程度、可能原因。

## 2. 软冲突（存在不一致但程度较轻）
总结软冲突分布，重点关注媒体频率类题目（matrix_row）的等级偏差。

## 3. 缺失冲突（OCR 未识别或推荐答案缺失）
遍历第三章中冲突类型为"缺失"的条目，逐条说明：题目、推荐答案、用户实际（无）、原因、处理建议。

# 第五章：综合建议
## 1. 针对人设设计的建议
| 优先级 | 问题 | 建议 |
|--------|------|------|

## 2. 针对 AI 推荐答题系统的建议
| 优先级 | 问题 | 建议 |
|--------|------|------|

# 第六章：用户执行力评估
## 1. 评分标准
- 整体平均分 = N3 × 10 / (page_count - N1)
- 纯有效平均分 = N3 × 10 / (page_count - N1 - N2)

## 2. 统计概览
| 指标 | 值 |
|------|-----|
| 页面总数（=总题数） | page_count |
| 忽略题数 (N1) | ignored_questions |
| OCR缺失题数 (N2) | ocr_missing_questions |
| 有效已答题数 (N3) | valid_answered_count |

## 3. 评估结论
根据得分给出结论描述：
- 90%~100%：优秀，人设高度一致
- 70%~89%：良好，存在少量软冲突，属正常范围
- 50%~69%：警告，存在较多冲突，需人工复查
- 50% 以下：异常，人设严重不一致，建议终止本次 Session
"""

DESCRIPTION = (
    "人设一致性分析报告提示词 v3：每页一行（page_result_id溯源），"
    "总题数=页面总数，冲突行红色高亮"
)


def main():
    if not os.path.exists(DB_PATH):
        print(f"[ERROR] 数据库文件不存在: {DB_PATH}")
        return

    conn = sqlite3.connect(DB_PATH)
    cur = conn.cursor()

    cur.execute(
        "SELECT id, prompt_key, description, version FROM prompts WHERE prompt_key = 'session.persona_analysis'"
    )
    row = cur.fetchone()
    if row:
        print(f"[INFO] 现有记录: id={row[0]}, prompt_key={row[1]}, description={row[2]}, version={row[3]}")
    else:
        print("[WARN] 未找到记录，将执行 INSERT")

    cur.execute("""
        INSERT INTO prompts (prompt_key, prompt_type, question_type, content, description,
                             yaml_template, version, is_active, created_at, updated_at)
        VALUES (
            'session.persona_analysis', 'analysis', NULL, ?, ?,
            NULL,
            COALESCE((SELECT MAX(version) FROM prompts WHERE prompt_key = 'session.persona_analysis'), 0) + 1,
            1,
            datetime('now', 'localtime'), datetime('now', 'localtime')
        )
        ON CONFLICT(prompt_key) DO UPDATE SET
            content = excluded.content,
            description = excluded.description,
            version = prompts.version + 1,
            updated_at = datetime('now', 'localtime')
    """, (PROMPT_CONTENT, DESCRIPTION))

    conn.commit()
    print("[OK] upsert 成功")

    cur.execute(
        "SELECT id, prompt_key, description, version FROM prompts WHERE prompt_key = 'session.persona_analysis'"
    )
    updated = cur.fetchone()
    print(f"[VERIFIED] id={updated[0]}, prompt_key={updated[1]}, description={updated[2]}, version={updated[3]}")
    print(f"[INFO] content 长度: {len(PROMPT_CONTENT)} 字符")

    conn.close()


if __name__ == "__main__":
    main()
