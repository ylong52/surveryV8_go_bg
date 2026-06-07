#!/usr/bin/env python3
"""
数据库迁移脚本：将 page_results 表从旧结构迁移到新结构

旧结构：
  - yaml_data_md5 (删除)
  - yaml_data (重命名为 yaml_template)
  - recommend_answer (保留)
  - 无 interactions 字段 (新增)
  - 无 interactions_md5 字段 (新增)

新结构：
  - yaml_template (原 yaml_data)
  - recommend_answer (保留)
  - interactions (从 recommend_answer 提取)
  - interactions_md5 (基于 interactions 计算)
"""

import json
import hashlib
import sqlite3
import sys
import os

DB_PATH = "data/survey202605.db"


def calc_md5(data: str) -> str:
    """计算字符串的 MD5 哈希值"""
    return hashlib.md5(data.encode('utf-8')).hexdigest()


def migrate_page_results_table(db_path: str):
    """执行 page_results 表迁移"""
    print(f"连接数据库: {db_path}")
    conn = sqlite3.connect(db_path)
    cursor = conn.cursor()

    # 1. 检查当前表结构
    cursor.execute("PRAGMA table_info(page_results)")
    columns = {row[1]: row for row in cursor.fetchall()}
    print(f"\n当前 page_results 表字段: {list(columns.keys())}")

    # 2. 检查是否需要迁移
    needs_migration = False

    if "yaml_data_md5" in columns:
        print("检测到旧字段 yaml_data_md5，需要迁移")
        needs_migration = True

    if "yaml_data" in columns and "yaml_template" not in columns:
        print("检测到旧字段 yaml_data (需要重命名为 yaml_template)")
        needs_migration = True

    if "interactions" not in columns:
        print("检测到缺少新字段 interactions")
        needs_migration = True

    if "interactions_md5" not in columns:
        print("检测到缺少新字段 interactions_md5")
        needs_migration = True

    if not needs_migration:
        print("\n表结构已经是最新，无需迁移")
        conn.close()
        return

    # 3. 获取现有数据
    print("\n读取现有数据...")
    cursor.execute("""
        SELECT id, survey_id, page_id, yaml_data_md5, yaml_data, recommend_answer, screenshot_path, created_at
        FROM page_results
    """)
    rows = cursor.fetchall()
    print(f"共有 {len(rows)} 条记录需要迁移")

    # 4. 创建新表
    print("\n创建新表结构...")
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS page_results_new (
            id                  INTEGER PRIMARY KEY AUTOINCREMENT,
            survey_id           INTEGER     NOT NULL,
            page_id             VARCHAR(128) NOT NULL,
            yaml_template       TEXT,
            recommend_answer    TEXT,
            interactions        TEXT,
            interactions_md5   VARCHAR(32)  NOT NULL,
            screenshot_path     TEXT,
            created_at          DATETIME    DEFAULT (datetime('now', 'localtime')),
            FOREIGN KEY (survey_id) REFERENCES survey(id),
            UNIQUE(survey_id, page_id, interactions_md5)
        )
    """)

    # 5. 迁移数据
    print("\n迁移数据...")
    migrated = 0
    skipped = 0

    for row in rows:
        old_id, survey_id, page_id, old_yaml_md5, yaml_data, recommend_answer, screenshot_path, created_at = row

        # 从 recommend_answer 提取 interactions
        interactions = "[]"
        interactions_md5 = ""

        if recommend_answer:
            try:
                data = json.loads(recommend_answer)
                if "data" in data and "interactions" in data["data"]:
                    interactions = json.dumps(data["data"]["interactions"], sort_keys=True)
                    interactions_md5 = calc_md5(interactions)
            except (json.JSONDecodeError, KeyError) as e:
                print(f"  警告: id={old_id} 解析 recommend_answer 失败: {e}")
                # 使用空数组的 MD5
                interactions = "[]"
                interactions_md5 = calc_md5("[]")

        # 插入新表（yaml_template 使用空字符串）
        try:
            cursor.execute("""
                INSERT INTO page_results_new
                    (survey_id, page_id, yaml_template, recommend_answer, interactions, interactions_md5, screenshot_path, created_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?)
            """, (survey_id, page_id, "", recommend_answer, interactions, interactions_md5, screenshot_path, created_at))
            migrated += 1
        except sqlite3.IntegrityError as e:
            print(f"  警告: id={old_id} 插入失败 (可能重复): {e}")
            skipped += 1

    print(f"迁移完成: 成功 {migrated} 条, 跳过 {skipped} 条")

    # 6. 删除旧表，重命名新表
    print("\n替换旧表...")
    cursor.execute("DROP TABLE page_results")
    cursor.execute("ALTER TABLE page_results_new RENAME TO page_results")

    # 7. 重建索引
    print("重建索引...")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_page_results_survey_id ON page_results(survey_id)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_page_results_page_id ON page_results(page_id)")
    cursor.execute("CREATE INDEX IF NOT EXISTS idx_page_results_interactions_md5 ON page_results(interactions_md5)")

    conn.commit()

    # 8. 验证新结构
    print("\n验证新表结构...")
    cursor.execute("PRAGMA table_info(page_results)")
    new_columns = [row[1] for row in cursor.fetchall()]
    print(f"新字段: {new_columns}")

    conn.close()
    print("\n迁移完成!")


if __name__ == "__main__":
    # 确保数据库文件存在
    if not os.path.exists(DB_PATH):
        print(f"错误: 数据库文件不存在: {DB_PATH}")
        sys.exit(1)

    migrate_page_results_table(DB_PATH)
