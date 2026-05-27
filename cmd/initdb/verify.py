import sqlite3

conn = sqlite3.connect('data/survey.db')
cur = conn.cursor()

# 列出所有表
cur.execute("SELECT name FROM sqlite_master WHERE type='table'")
tables = [r[0] for r in cur.fetchall()]
print('Tables:', tables)

# 查看 page_results 表结构
cur.execute("PRAGMA table_info(page_results)")
print('\npage_results columns:')
for r in cur.fetchall():
    print(f'  {r[1]:20} {r[2]}')

conn.close()
