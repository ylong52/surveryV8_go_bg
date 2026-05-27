import sqlite3

conn = sqlite3.connect('data/survey.db')
cur = conn.cursor()

# 查看所有表
print("=== 所有表 ===")
cur.execute("SELECT name FROM sqlite_master WHERE type='table'")
print(f"  {[r[0] for r in cur.fetchall()]}")

# 查看 survey 表
print("\n=== survey 表 ===")
cur.execute("SELECT COUNT(*) FROM survey")
print(f"  记录数: {cur.fetchone()[0]}")
cur.execute("SELECT id, session_id, survey_id, created_at FROM survey")
for r in cur.fetchall():
    print(f"  id={r[0]}, session_id={r[1]}, survey_id={r[2]}, created={r[3]}")

# 查看 page_results 表
print("\n=== page_results 表 ===")
cur.execute("SELECT COUNT(*) FROM page_results")
print(f"  记录数: {cur.fetchone()[0]}")
cur.execute("SELECT id, survey_id, page_id, yaml_data_md5, created_at FROM page_results")
for r in cur.fetchall():
    print(f"  id={r[0]}, survey_id={r[1]}, page_id={r[2]}, md5={r[3][:8] if r[3] else 'None'}..., created={r[4]}")

# 查看 page_html_source 表
print("\n=== page_html_source 表 ===")
cur.execute("SELECT COUNT(*) FROM page_html_source")
print(f"  记录数: {cur.fetchone()[0]}")

conn.close()
