import sqlite3, os

dbs = [
    r"E:\case2025\adspower\surveyV8_go\surveryV8_go_bg\cmd\initdb\data\survey.db",
    r"E:\case2025\adspower\surveyV8_go\surveryV8_go_bg\data\survey.db",
    r"E:\case2025\adspower\surveyV8_go\surveryV8_go_bg\data\survey_new.db",
    r"E:\case2025\adspower\surveyV8_go\surveryV8_go_bg\data\survey202605.db",
]

for db_path in dbs:
    if not os.path.exists(db_path):
        print(f"[跳过] {db_path} 不存在")
        continue
    try:
        conn = sqlite3.connect(db_path)
        cur = conn.cursor()
        cur.execute("SELECT name FROM sqlite_master WHERE type='table' AND name LIKE '%prompt%'")
        tables = [r[0] for r in cur.fetchall()]
        print(f"[{os.path.basename(db_path)}] prompt相关表: {tables}")
        if tables:
            cur.execute(f"SELECT COUNT(*) FROM \"{tables[0]}\"")
            print(f"  行数: {cur.fetchone()[0]}")
        conn.close()
    except Exception as e:
        print(f"[{os.path.basename(db_path)}] 错误: {e}")
