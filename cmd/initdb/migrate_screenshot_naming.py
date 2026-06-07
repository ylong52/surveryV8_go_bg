#!/usr/bin/env python3
"""
迁移截图文件命名格式：旧格式 {question_text}.jpg → 新格式 {page_results_id}.jpg

匹配逻辑：
  1. 遍历所有 DB (survey202605.db)，构建 (session_id → survey_id) 和 (survey_id, page_id → page_results_id) 映射
  2. 对每个旧格式截图文件，通过 session_id + page_id 模糊匹配找到 page_results_id
  3. 找到则重命名，无法匹配则移入 _orphaned/

数据库 schema 要求：
  survey: id, session_name
  page_results: id, survey_id, page_id
"""

import os
import re
import sqlite3
import shutil
from pathlib import Path


# ========== 配置 ==========
WORKSPACE_ROOT = Path(__file__).parent.parent.parent.resolve()
STORAGE_PATH = WORKSPACE_ROOT / "storage"
DATA_DIR = WORKSPACE_ROOT / "data"
ORPHANED_DIR = STORAGE_PATH / "screenshots" / "_orphaned"

# 多数据库列表（按优先级依次尝试）
DB_FILES = [
    DATA_DIR / "survey202605.db",
    DATA_DIR / "survey_new.db",
    DATA_DIR / "survey.db",
]


def load_all_databases(db_files: list[Path]) -> tuple[dict, dict]:
    """
    从多个数据库合并加载数据。
    返回: (session_to_survey, page_lookup)
    session_to_survey: session_id → survey_id
    page_lookup: (survey_id, page_id) → page_results_id (取最小id)
    """
    session_to_survey: dict[str, int] = {}
    page_lookup: dict[tuple, int] = {}

    for db_path in db_files:
        if not db_path.exists():
            print(f"  跳过不存在的 DB: {db_path.name}")
            continue
        try:
            conn = sqlite3.connect(db_path)
            cursor = conn.cursor()

            cursor.execute("SELECT session_name, id FROM survey")
            for row in cursor.fetchall():
                sn, sid = row[0], row[1]
                if sn not in session_to_survey:
                    session_to_survey[sn] = sid

            cursor.execute("SELECT id, survey_id, page_id FROM page_results")
            for row in cursor.fetchall():
                pr_id, sid, pid = row[0], row[1], row[2]
                key = (sid, pid)
                if key not in page_lookup:
                    page_lookup[key] = pr_id
                else:
                    page_lookup[key] = min(page_lookup[key], pr_id)

            conn.close()
            print(f"  已加载: {db_path.name}")
        except Exception as e:
            print(f"  加载失败 {db_path.name}: {e}")

    return session_to_survey, page_lookup


def is_old_format_filename(filename: str) -> bool:
    """判断是否为旧格式文件名（纯数字ID格式为新格式）"""
    return not Path(filename).stem.isdigit()


def extract_page_id_from_filename(filename: str) -> str:
    """
    从旧格式文件名推断 page_id。
    例如：
      "what is your age what is your gender.jpg" → 推断 page_id 相关词
    由于无法 100% 还原 page_id，采用模糊匹配策略：
    1. 去掉 .jpg，用空格替换下划线/连字符
    2. 转为小写
    3. 返回清洗后的文本作为搜索关键词
    """
    name = Path(filename).stem
    # 将常见分隔符统一为空格
    name = re.sub(r"[-_]+", " ", name)
    name = name.lower().strip()
    return name


def find_matching_page_result(session_id: str, question_text: str,
                               session_to_survey: dict, page_lookup: dict) -> int | None:
    """
    在数据库中查找匹配的 page_results.id
    策略：
    1. 通过 session_id 找到 survey_id
    2. 在该 survey 下，用模糊匹配找 page_id
    """
    survey_id = session_to_survey.get(session_id)
    if survey_id is None:
        return None

    # 精确匹配
    if (survey_id, question_text) in page_lookup:
        return page_lookup[(survey_id, question_text)]

    # 模糊匹配：找 page_id 包含 question 关键词的
    clean_question = re.sub(r"[-_]+", " ", question_text.lower().strip())
    candidates = []

    for (sid, pid), pr_id in page_lookup.items():
        if sid != survey_id:
            continue
        clean_pid = re.sub(r"[-_]+", " ", pid.lower())

        # 词级别交集匹配
        q_words = {w for w in clean_question.split() if len(w) > 3}
        p_words = {w for w in clean_pid.split() if len(w) > 3}
        if q_words and p_words:
            overlap = len(q_words & p_words)
            total = len(q_words | p_words)
            score = overlap / total if total > 0 else 0
            if score >= 0.3:
                candidates.append((pr_id, score * 100))

    if not candidates:
        return None
    candidates.sort(key=lambda x: -x[1])
    return candidates[0][0]


def migrate_screenshots(dry_run: bool = True):
    """执行截图文件迁移"""
    print(f"存储路径: {STORAGE_PATH}")
    print(f"演练模式: {'是 (dry-run，不实际修改)' if dry_run else '否 (正式执行)'}")
    print()

    # 构建查找表（多 DB 合并）
    print("加载数据库查找表...")
    session_to_survey, page_lookup = load_all_databases(DB_FILES)
    print(f"  survey 记录: {len(session_to_survey)} 条")
    print(f"  page_results 记录: {len(page_lookup)} 条")
    print()

    screenshots_dir = STORAGE_PATH / "screenshots"
    if not screenshots_dir.exists():
        print("截图目录不存在，退出。")
        return

    # 统计
    stats = {
        "total": 0,
        "new_format": 0,
        "old_matched": 0,
        "old_orphaned": 0,
    }

    # 遍历所有 session 子目录
    for session_dir in sorted(screenshots_dir.iterdir()):
        if not session_dir.is_dir():
            continue
        if session_dir.name == "_orphaned":
            continue

        session_id = session_dir.name

        for old_file in sorted(session_dir.iterdir()):
            if not old_file.is_file() or old_file.suffix.lower() not in (".jpg", ".jpeg", ".png"):
                continue

            stats["total"] += 1

            if not is_old_format_filename(old_file.name):
                stats["new_format"] += 1
                continue

            # 旧格式文件，尝试匹配
            question_text = old_file.stem
            pr_id = find_matching_page_result(
                session_id, question_text, session_to_survey, page_lookup
            )

            if pr_id is not None:
                new_name = f"{pr_id}{old_file.suffix}"
                new_file = old_file.parent / new_name
                action = f"MATCH → {new_name}"
                stats["old_matched"] += 1

                if not dry_run:
                    # 如果新文件名已存在，先删除旧的（已被更新的截图覆盖）
                    if new_file.exists() and new_file.resolve() != old_file.resolve():
                        print(f"  覆盖已有文件: {new_file} (删除旧同名文件)")
                        new_file.unlink()
                    old_file.rename(new_file)

                    # 更新数据库
                    conn = sqlite3.connect(db_path)
                    cursor = conn.cursor()
                    cursor.execute(
                        "UPDATE page_results SET screenshot_path = ? WHERE id = ?",
                        (str(new_file), pr_id)
                    )
                    conn.commit()
                    conn.close()
            else:
                # 无法匹配，移到孤立目录
                action = "ORPHANED"
                stats["old_orphaned"] += 1

                if not dry_run:
                    ORPHANED_DIR.mkdir(parents=True, exist_ok=True)
                    orphaned_file = ORPHANED_DIR / f"{session_id}_{old_file.name}"
                    counter = 1
                    while orphaned_file.exists():
                        orphaned_file = ORPHANED_DIR / f"{session_id}_{old_file.stem}_{counter}{old_file.suffix}"
                        counter += 1
                    old_file.rename(orphaned_file)

            if dry_run:
                print(f"  {action}: {old_file.name}")

    print()
    print("========== 统计 ==========")
    print(f"  总文件数        : {stats['total']}")
    print(f"  新格式(已合规)  : {stats['new_format']}")
    print(f"  旧格式→已匹配   : {stats['old_matched']}")
    print(f"  旧格式→孤立     : {stats['old_orphaned']}")
    print()

    if dry_run:
        print("演练完成。若确认无误，重新运行并设置 dry_run=False 执行实际迁移。")
    else:
        print("迁移完成！")
        if stats["old_orphaned"] > 0:
            print(f"孤立文件已移至: {ORPHANED_DIR}")


if __name__ == "__main__":
    import sys
    dry_run = "--execute" not in sys.argv
    migrate_screenshots(dry_run=dry_run)
