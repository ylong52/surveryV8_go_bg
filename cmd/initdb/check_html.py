import sqlite3

conn = sqlite3.connect('data/survey.db')
cur = conn.cursor()

# 查看表结构
print("=== 表结构 ===")
cur.execute("PRAGMA table_info(page_html_source)")
for r in cur.fetchall():
    print(f"  {r[1]:20} {r[2]}")

# 查看记录数
print("\n=== 记录数 ===")
cur.execute("SELECT COUNT(*) FROM page_html_source")
count = cur.fetchone()[0]
print(f"  page_html_source: {count} 条")

# 查看所有记录
print("\n=== 所有记录 ===")
cur.execute("SELECT id, page_results_id, raw_html_md5, length(raw_html) as html_len, created_at FROM page_html_source")
rows = cur.fetchall()
if rows:
    for r in rows:
        print(f"  id={r[0]}, page_results_id={r[1]}, md5={r[2][:8]}..., html_len={r[3]}, created={r[4]}")
else:
    print("  (空)")

conn.close()
