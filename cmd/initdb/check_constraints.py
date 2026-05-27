import sqlite3

conn = sqlite3.connect('data/survey.db')
cur = conn.cursor()

# 检查 page_results 表约束
print("=== page_results 索引和约束 ===")
cur.execute("SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name='page_results'")
for r in cur.fetchall():
    print(f"  {r[0]}")

# 检查 page_html_source 索引
print("\n=== page_html_source 索引 ===")
cur.execute("SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name='page_html_source'")
for r in cur.fetchall():
    print(f"  {r[0]}")

conn.close()
