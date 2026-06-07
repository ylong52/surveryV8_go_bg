import sqlite3

conn = sqlite3.connect(r'E:\case2025\adspower\surveyV8_go\surveryV8_go_bg\data\survey202605.db')
cur = conn.cursor()
cur.execute("SELECT id, prompt_key, prompt_type, version, is_active, LENGTH(content) FROM prompts WHERE prompt_key='session.persona_analysis'")
r = cur.fetchone()
print(f'id={r[0]}, key={r[1]}, type={r[2]}, version={r[3]}, active={r[4]}, content长度={r[5]}字符')
conn.close()
