import sqlite3

conn = sqlite3.connect('data/survey.db')
cur = conn.cursor()

print("=== 清理和验证 ===\n")

# 清理临时表
print("1. 清理临时表...")
cur.execute("SELECT name FROM sqlite_master WHERE type='table' AND name LIKE '%_old%'")
old_tables = cur.fetchall()
for t in old_tables:
    print(f"   删除: {t[0]}")
    cur.execute(f"DROP TABLE IF EXISTS {t[0]}")
conn.commit()

# 验证 page_results 约束
print("\n2. 验证 page_results 表约束...")
cur.execute("SELECT sql FROM sqlite_master WHERE type='table' AND name='page_results'")
table_sql = cur.fetchone()[0]
if "UNIQUE(survey_id, page_id, yaml_data_md5)" in table_sql:
    print("   [OK] 约束正确: UNIQUE(survey_id, page_id, yaml_data_md5)")
else:
    print("   [X] 约束错误")

# 验证索引
print("\n3. 验证索引...")
cur.execute("SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name='page_results'")
for r in cur.fetchall():
    if r[0]:
        print(f"   {r[0]}")

# 验证数据
print("\n4. 验证数据...")
cur.execute("SELECT COUNT(*) FROM page_results")
print(f"   page_results: {cur.fetchone()[0]} 条")
cur.execute("SELECT COUNT(*) FROM survey")
print(f"   survey: {cur.fetchone()[0]} 条")
cur.execute("SELECT COUNT(*) FROM page_html_source")
print(f"   page_html_source: {cur.fetchone()[0]} 条")

# 最终表列表
print("\n5. 所有表...")
cur.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
for r in cur.fetchall():
    print(f"   {r[0]}")

conn.close()
print("\n=== 完成 ===")
