#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""更新 session.persona_analysis 提示词到新版六章结构"""

import sqlite3
import os

# 读取环境变量或使用默认数据库路径
DB_PATH = os.environ.get("DB_PATH", "./data/survey202605.db")

NEW_PROMPT_CONTENT = r"""你是一个问卷人设一致性分析专家。根据以下 YAML 数据，严格按照六章结构生成完整分析报告。

# 第一章：基本信息
从 session_info 提取所有字段填写。

# 第二章：系统人设 vs 用户人设对比表
根据 system_persona 和 user_persona 的字段，列出各维度对比：
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

对于每个维度，给出：系统预设值、用户实际值、冲突类型（硬冲突/软冲突/无冲突）、冲突说明。

# 第三章：逐题分析
逐条填写 page_results 中的每一题，使用以下字段：
| 序号 | 页面ID | 题型 | 推荐答案 | 用户实际(OCR) | 人设相关性 | 是否跟随 | 冲突类型 | 问题说明 |
按顺序填写所有题目。

# 第四章：详细冲突原因分析
## 1. 硬冲突分析
总结硬冲突情况（系统人设与用户实际答题存在直接矛盾）。

## 2. 软冲突分析
总结软冲突情况（存在不一致但程度较轻）。

## 3. 缺失冲突分析
遍历 page_results 中冲突类型为"缺失"的条目，逐条说明：
- 题目名称和推荐答案
- 用户实际答案（无）
- 可能原因：截图路径存在但文件缺失
- 处理建议：人工复核或重新截图

# 第五章：综合建议
## 1. 针对人设设计的建议
| 优先级 | 问题 | 建议 |

## 2. 针对AI推荐答题系统的建议
| 优先级 | 问题 | 建议 |

# 第六章：用户执行力评估
## 1. 评分标准
- 匹配得分：匹配题数 × 10分
- 整体平均分：N3 × 10 / (N4 - N1)
- 纯有效平均分：N3 × 10 / (N4 - N1 - N2)

## 2. 统计概览
从 session_info 读取：
- 忽略题数 (N1)
- OCR缺失题数 (N2)
- 有效已答题数 (N3)
- 总题数 (N4)

## 3. 评估结论
填写整体平均分、纯有效平均分，并根据得分给出结论描述。"""

NEW_DESCRIPTION = "人设一致性分析报告生成提示词（新版六章结构）"


def main():
    if not os.path.exists(DB_PATH):
        print(f"[ERROR] 数据库文件不存在: {DB_PATH}")
        return

    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()

    # 先查看当前记录
    cursor.execute(
        "SELECT id, prompt_key, description, version FROM prompts WHERE prompt_key = 'session.persona_analysis'"
    )
    row = cursor.fetchone()
    if row:
        print(f"[INFO] 找到记录: id={row[0]}, prompt_key={row[1]}, description={row[2]}, version={row[3]}")
    else:
        print("[WARN] 未找到 prompt_key='session.persona_analysis'，将尝试插入新记录")
        # 尝试插入
        cursor.execute("""
            INSERT INTO prompts (prompt_key, prompt_type, question_type, content, description, yaml_template, version, is_active, created_at, updated_at)
            VALUES ('session.persona_analysis', 'session', NULL, ?, ?, NULL, 1, 1, datetime('now', 'localtime'), datetime('now', 'localtime'))
        """, (NEW_PROMPT_CONTENT, NEW_DESCRIPTION))
        conn.commit()
        print(f"[OK] 新记录已插入，id={cursor.lastrowid}")
        conn.close()
        return

    # 执行更新
    cursor.execute("""
        UPDATE prompts
        SET content = ?,
            description = ?,
            version = version + 1,
            updated_at = datetime('now', 'localtime')
        WHERE prompt_key = 'session.persona_analysis' AND is_active = 1
    """, (NEW_PROMPT_CONTENT, NEW_DESCRIPTION))

    if cursor.rowcount > 0:
        conn.commit()
        print("[OK] 更新成功")
        # 验证
        cursor.execute("SELECT id, prompt_key, description, version FROM prompts WHERE prompt_key = 'session.persona_analysis'")
        updated = cursor.fetchone()
        print(f"[VERIFIED] id={updated[0]}, prompt_key={updated[1]}, description={updated[2]}, version={updated[3]}")
    else:
        print("[ERROR] 更新失败，可能记录不存在或 is_active=0")

    conn.close()


if __name__ == "__main__":
    main()
