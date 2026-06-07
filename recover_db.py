"""
recover_db.py — 从备份 SQL 重建 data/survey202605.db
运行: python recover_db.py
"""
import sqlite3
import os
import re
import shutil
import datetime

DB_PATH        = "data/survey202605.db"
DB_NEW         = "data/survey_new.db"   # 先写入临时路径
MAIN_SQL       = "data/bak/main.sql"
PROMPTS_SQL    = "data/prompts.sql"
OPENTEXT_SQL   = "docs/opentext_branch_prompts.sql"

def log(msg):
    print(f"[recover] {msg}")

# ─── 1. 清理上次可能残留的临时文件 ──────────────────────────────────────────
if os.path.exists(DB_NEW):
    os.remove(DB_NEW)
    log(f"清理残留临时文件: {DB_NEW}")

# ─── 2. 从 main.sql 重建主库 ──────────────────────────────────────────────────
log(f"从 {MAIN_SQL} 重建数据库（写入 {DB_NEW}）...")
with open(MAIN_SQL, encoding="utf-8") as f:
    main_sql = f.read()

conn = sqlite3.connect(DB_NEW)
conn.row_factory = sqlite3.Row
try:
    conn.executescript(main_sql)
    conn.commit()
    log("main.sql 导入完成")
except Exception as e:
    log(f"main.sql 导入出错（尝试继续）: {e}")
    conn.commit()

# ─── 3. 用 prompts.sql 覆盖更新 prompts 表 ──────────────────────────────────
log(f"用 {PROMPTS_SQL} 更新 prompts 表...")
with open(PROMPTS_SQL, encoding="utf-8") as f:
    prompts_sql = f.read()

# 将 INSERT INTO 替换为 INSERT OR REPLACE INTO 避免主键冲突
prompts_sql_replace = prompts_sql.replace(
    'INSERT INTO "prompts"', 'INSERT OR REPLACE INTO "prompts"'
)
try:
    conn.executescript(prompts_sql_replace)
    conn.commit()
    max_id = conn.execute("SELECT MAX(id) FROM prompts").fetchone()[0]
    count  = conn.execute("SELECT COUNT(*) FROM prompts").fetchone()[0]
    log(f"prompts.sql 导入完成，共 {count} 条，当前最大 ID={max_id}")
except Exception as e:
    log(f"prompts.sql 导入出错: {e}")
    conn.commit()

# ─── 4. 插入 open_text.* 提示词 ──────────────────────────────────────────────
log(f"从 {OPENTEXT_SQL} 插入 open_text.* 提示词...")
with open(OPENTEXT_SQL, encoding="utf-8") as f:
    ot_content = f.read()

# 提取三段 INSERT 语句
inserts = re.findall(r'INSERT INTO.*?;', ot_content, re.DOTALL)
log(f"找到 {len(inserts)} 条 INSERT")

# 取当前最大 ID，从 max_id+1 开始重新分配
try:
    base_id = conn.execute("SELECT MAX(id) FROM prompts").fetchone()[0] or 0
except Exception:
    base_id = 200

for i, stmt in enumerate(inserts):
    new_id = base_id + i + 1
    # 替换第一个 VALUES ( 后面的 ID 数字
    stmt_fixed = re.sub(
        r'(VALUES\s*\(\s*)\d+',
        rf'\g<1>{new_id}',
        stmt,
        count=1
    )
    # 使用 INSERT OR IGNORE 避免 prompt_key 重复冲突
    stmt_fixed = stmt_fixed.replace(
        'INSERT INTO "prompts"', 'INSERT OR IGNORE INTO "prompts"'
    )
    try:
        conn.executescript(stmt_fixed)
        conn.commit()
        # 提取 prompt_key 用于日志
        key_match = re.search(r"'open_text\.[^']*'", stmt_fixed)
        key = key_match.group(0) if key_match else f"unknown_{i}"
        log(f"  插入 id={new_id} key={key}")
    except Exception as e:
        log(f"  插入第 {i+1} 条出错: {e}")

# ─── 5. 验证结果 ──────────────────────────────────────────────────────────────
log("\n─── 验证 ───")
tables = conn.execute(
    "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name"
).fetchall()
log(f"表数量: {len(tables)}")

prompt_count = conn.execute("SELECT COUNT(*) FROM prompts").fetchone()[0]
log(f"prompts 行数: {prompt_count}")

opentext_keys = conn.execute(
    "SELECT id, prompt_key FROM prompts WHERE prompt_key LIKE 'open_text%' OR prompt_key LIKE 'system.type%'"
).fetchall()
log("open_text.* / system.type* 提示词:")
for row in opentext_keys:
    log(f"  id={row[0]}  key={row[1]}")

site_count = conn.execute("SELECT COUNT(*) FROM sites").fetchone()[0]
log(f"sites 行数: {site_count}")

conn.close()
log(f"\n新数据库已写入: {DB_NEW}")

# ─── 6. 替换旧库 ──────────────────────────────────────────────────────────────
ts = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
corrupt_bak = f"{DB_PATH}.corrupted_{ts}"
try:
    if os.path.exists(DB_PATH):
        shutil.move(DB_PATH, corrupt_bak)
        log(f"损坏文件已备份到: {corrupt_bak}")
    shutil.move(DB_NEW, DB_PATH)
    log(f"✅ 数据库重建完成: {DB_PATH}")
except PermissionError as e:
    log(f"\n⚠️  文件被占用，无法自动替换: {e}")
    log(f"   请手动操作：")
    log(f"   1. 关闭所有占用 survey.db 的进程（ws_server、IDE 等）")
    log(f"   2. 把  {DB_PATH}  改名为  {DB_PATH}.corrupted")
    log(f"   3. 把  {DB_NEW}  改名为  {DB_PATH}")
