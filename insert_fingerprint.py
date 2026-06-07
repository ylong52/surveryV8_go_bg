import sqlite3

conn = sqlite3.connect('data/survey202605.db')
cursor = conn.cursor()

# Insert fingerprint for site 1 (Macalester-2026-prospective-families, survey_id=8809865)
cursor.execute('''INSERT INTO site_page_fingerprints 
    (site_id, page_id_pattern_regex, next_pattern, wait_after_click, created_at) 
    VALUES (?, ?, ?, ?, datetime('now'))''',
    (1, '["/<legend[^>]*>([\\\\s\\\\S]*?)<\\\\/legend>/gi","/<label[^>]*>([\\\\s\\\\S]*?)<\\\\/label>/gi","/<div[^>]*>([\\\\s\\\\S]*?)<\\\\/div>/gi"]', 
     '/<input[^>]*id="sg_NextButton"/', 1500))

conn.commit()
print('Inserted ID:', cursor.lastrowid)
conn.close()
