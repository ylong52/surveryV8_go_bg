"""
将 data/prompts.sql 中的 content 同步到数据库 prompts 表。
用法: python sync_prompts_to_db.py
"""
import re
import sqlite3
import os

PROJ_ROOT = r"E:\case2025\adspower\surveyV8_go\surveryV8_go_bg"
DB_PATH = os.path.join(PROJ_ROOT, "data", "survey202605.db")
SQL_PATH = os.path.join(PROJ_ROOT, "data", "prompts.sql")


def parse_prompts_sql(path):
    """从 prompts.sql 解析 id, prompt_key, content"""
    with open(path, encoding="utf-8") as f:
        content = f.read()

    # 匹配每个 INSERT 语句
    pattern = re.compile(
        r'INSERT\s+INTO\s+"prompts"\s*\([^)]+\)\s*VALUES\s*\('
        r'(\d+)'
        r',\s*\'([^\']+)\''
        r',\s*\'[^\']*\''  # prompt_type
        r',\s*(?:NULL|\'[^\']*\')'  # question_type
        r',\s*\'(.*?)\'',  # content
        re.DOTALL,
    )

    records = []
    for m in pattern.finditer(content):
        pid = int(m.group(1))
        pkey = m.group(2)
        pcontent = m.group(3)
        # 去除末尾的 ')'
        pcontent = pcontent.rstrip("'")
        # 转义单引号: '' -> '
        pcontent = pcontent.replace("''", "'")
        records.append((pid, pkey, pcontent))
    return records


def main():
    db_path = os.path.normpath(DB_PATH)
    if not os.path.exists(db_path):
        print(f"数据库不存在: {db_path}")
        return

    records = parse_prompts_sql(os.path.normpath(SQL_PATH))
    print(f"从 prompts.sql 解析了 {len(records)} 条记录")

    conn = sqlite3.connect(db_path)
    cur = conn.cursor()

    updated = 0
    for pid, pkey, pcontent in records:
        cur.execute(
            "UPDATE prompts SET content=?, updated_at=strftime('%Y-%m-%d %H:%M:%S','now','localtime') WHERE id=? AND prompt_key=?",
            (pcontent, pid, pkey),
        )
        if cur.rowcount > 0:
            updated += 1

    conn.commit()
    print(f"已更新 {updated} 条记录")
    conn.close()


if __name__ == "__main__":
    main()
