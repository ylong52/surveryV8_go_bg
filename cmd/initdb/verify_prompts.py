import os
import sqlite3

_root = os.path.dirname(os.path.dirname(os.path.dirname(__file__)))
conn = sqlite3.connect(os.path.join(_root, 'data', 'survey.db'))
cur = conn.cursor()

# 1. 总记录
print('=== 1. 总记录数 ===')
r = cur.execute('SELECT COUNT(*) FROM prompts').fetchone()
print(f'  总记录: {r[0]}')

# 2. yaml_template 为空
print()
print('=== 2. yaml_template 为空 ===')
rows = cur.execute(
    "SELECT prompt_key, prompt_type, LENGTH(yaml_template) "
    "FROM prompts WHERE yaml_template IS NULL OR yaml_template = '' "
    "ORDER BY prompt_type"
).fetchall()
if rows:
    for r2 in rows:
        print(f'  {r2[0]} | {r2[1]} | len={r2[2]}')
else:
    print('  (无)')

# 3. content 为空
print()
print('=== 3. content 为空 ===')
rows = cur.execute(
    "SELECT prompt_key, prompt_type, LENGTH(content) "
    "FROM prompts WHERE content IS NULL OR content = '' "
    "ORDER BY prompt_type"
).fetchall()
if rows:
    for r2 in rows:
        print(f'  {r2[0]} | {r2[1]} | len={r2[2]}')
else:
    print('  (无)')

# 4. 按类型统计
print()
print('=== 4. 按类型分布 ===')
rows = cur.execute(
    'SELECT prompt_type, COUNT(*) as cnt FROM prompts GROUP BY prompt_type ORDER BY prompt_type'
).fetchall()
for r2 in rows:
    print(f'  {r2[0]}: {r2[1]}')

# 5. yaml_template 抽样
print()
print('=== 5. yaml_template 抽样 ===')
rows = cur.execute(
    "SELECT prompt_key, SUBSTR(yaml_template,1,70) "
    "FROM prompts "
    "WHERE prompt_type='question_type' "
    "AND prompt_key IN ('qtype.single','qtype.multiple','qtype.matrix_radio')"
).fetchall()
for r2 in rows:
    print(f'  {r2[0]}: {r2[1]}')

# 6. persona.generic content
print()
print('=== 6. persona.generic content 首行 ===')
r2 = cur.execute(
    "SELECT prompt_key, SUBSTR(content,1,80) FROM prompts WHERE prompt_key='persona.generic'"
).fetchone()
if r2:
    print(f'  {r2[0]}: {r2[1]}')

# 7. 只查 question_type 类型的 yaml_template
print()
print('=== 7. question_type 记录 yaml_template 全部非空 ===')
rows = cur.execute(
    "SELECT prompt_key, LENGTH(yaml_template) as ylen "
    "FROM prompts WHERE prompt_type='question_type' ORDER BY prompt_key"
).fetchall()
all_filled = all(r[1] > 0 for r in rows)
empty = [r[0] for r in rows if r[1] == 0 or r[1] is None]
print(f'  总数: {len(rows)}, 全部有值: {all_filled}')
if empty:
    print(f'  空值: {empty}')
else:
    print('  (全部有值)')

conn.close()
