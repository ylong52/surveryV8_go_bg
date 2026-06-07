import sqlite3
import json

conn = sqlite3.connect('data/survey202605.db')
cursor = conn.cursor()

# Delete old record first
cursor.execute("DELETE FROM site_page_fingerprints WHERE site_id = 1")

# Insert fingerprint for site 1 (Macalester-2026-prospective-families, survey_id=8809865)
patterns = [
    r"/<legend[^>]*>([\s\S]*?)<\/legend>/gi",
    r"/<label[^>]*>([\s\S]*?)<\/label>/gi",
    r"/<div[^>]*>([\s\S]*?)<\/div>/gi"
]

cursor.execute('''INSERT INTO site_page_fingerprints 
    (site_id, page_id_pattern_regex, next_pattern, wait_after_click, created_at) 
    VALUES (?, ?, ?, ?, datetime('now'))''',
    (1, json.dumps(patterns), '/<input[^>]*id="sg_NextButton"/', 1500))

conn.commit()
print('Inserted ID:', cursor.lastrowid)
print('Pattern:', json.dumps(patterns))
conn.close()
