import sqlite3  
conn = sqlite3.connect('data/survey202605.db')  
cur = conn.cursor()  
cur.execute('''  
    SELECT id, survey_id, page_id, page_fingerprint_md5, LENGTH(yaml_template) as yaml_len  
    FROM page_results  
    WHERE yaml_template IS NOT NULL AND yaml_template != ''  
    ORDER BY survey_id, id  
''')  
rows = cur.fetchall()  
print(f'Total: {len(rows)} records hit db_cache yaml_template')  
print()  
for r in rows:  
    print(f'  id={r[0]}  survey_id={r[1]}  page_id={r[2]}  fingerprint_md5={r[3]}  yaml_len={r[4]}')  
conn.close() 
