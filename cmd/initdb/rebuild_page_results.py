import sqlite3

conn = sqlite3.connect('data/survey202605.db')
cur = conn.cursor()

print("=== 重建 page_results 表 ===")

# 1. 查看当前数据
print("\n1. 当前数据:")
cur.execute("SELECT id, survey_id, page_id, yaml_data_md5, created_at FROM page_results")
rows = cur.fetchall()
print(f"   共 {len(rows)} 条记录")

# 2. 备份数据
print("\n2. 备份数据到临时表...")
cur.execute("ALTER TABLE page_results RENAME TO page_results_old")

# 3. 创建新表
print("\n3. 创建新表...")
cur.execute("""
CREATE TABLE page_results (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    survey_id           INTEGER     NOT NULL,
    page_id             VARCHAR(128) NOT NULL,
    yaml_data_md5       VARCHAR(32)  NOT NULL,
    yaml_data           TEXT,
    recommend_answer    TEXT,
    screenshot_path     TEXT,
    created_at          DATETIME    DEFAULT (datetime('now', 'localtime')),
    FOREIGN KEY (survey_id) REFERENCES survey(id),
    UNIQUE(survey_id, page_id, yaml_data_md5)
)
""")
print("   新表创建成功")

# 4. 迁移数据
print("\n4. 迁移数据...")
cur.execute("""
INSERT INTO page_results (id, survey_id, page_id, yaml_data_md5, yaml_data, recommend_answer, screenshot_path, created_at)
SELECT id, survey_id, page_id, yaml_data_md5, yaml_data, recommend_answer, screenshot_path, created_at FROM page_results_old
""")
print(f"   迁移了 {cur.rowcount} 条记录")

# 5. 删除旧表
print("\n5. 删除旧表...")
cur.execute("DROP TABLE page_results_old")
print("   删除成功")

# 6. 重建索引
print("\n6. 重建索引...")
cur.execute("CREATE INDEX idx_page_results_survey_id ON page_results(survey_id)")
cur.execute("CREATE INDEX idx_page_results_page_id ON page_results(page_id)")
cur.execute("CREATE INDEX idx_page_results_yaml_md5 ON page_results(yaml_data_md5)")
print("   索引创建成功")

conn.commit()

# 验证
print("\n=== 验证 ===")
cur.execute("SELECT sql FROM sqlite_master WHERE type='table' AND name='page_results'")
print(f"表结构:\n   {cur.fetchone()[0]}")

cur.execute("SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name='page_results'")
for r in cur.fetchall():
    if r[0]:
        print(f"索引: {r[0]}")

cur.execute("SELECT COUNT(*) FROM page_results")
print(f"\n记录数: {cur.fetchone()[0]}")

conn.close()
print("\n=== 完成 ===")
