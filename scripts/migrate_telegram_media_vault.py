#!/usr/bin/env python3
"""
Migrate media and metadata from telegram-media-vault into 91.

Usage:
  python3 scripts/migrate_telegram_media_vault.py \
    --old-data /var/lib/telegram-media \
    --91-data /opt/video-site-91/data
"""

import argparse
import json
import os
import shutil
import sqlite3
import sys
import time
from pathlib import Path


def is_image_ext(ext: str) -> bool:
    return ext.lower().lstrip(".") in {"jpg", "jpeg", "png", "webp", "gif", "bmp"}


def main():
    parser = argparse.ArgumentParser(description="Migrate telegram-media-vault to 91")
    parser.add_argument(
        "--old-data",
        default="/var/lib/telegram-media",
        help="Path to telegram-media data dir (contains media.db and media/)",
    )
    parser.add_argument(
        "--new-data",
        dest="new_data",
        default="/opt/video-site-91/data",
        help="Path to 91 data dir (contains video-site.db and telegram/library/)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Simulate migration without modifying destination",
    )
    args = parser.parse_args()

    old_db_path = Path(args.old_data) / "media.db"
    old_media_dir = Path(args.old_data) / "media"
    new_db_path = Path(args.new_data) / "video-site.db"
    new_library_dir = Path(args.new_data) / "telegram" / "library"

    if not old_db_path.exists():
        print(f"[-] 旧数据库文件不存在：{old_db_path}", file=sys.stderr)
        sys.exit(1)
    if not new_db_path.exists():
        print(f"[-] 91 数据库文件不存在：{new_db_path}", file=sys.stderr)
        sys.exit(1)

    new_library_dir.mkdir(parents=True, exist_ok=True)

    print(f"[*] 正在连接旧数据库：{old_db_path}")
    old_conn = sqlite3.connect(old_db_path)
    old_conn.row_factory = sqlite3.Row

    print(f"[*] 正在连接 91 数据库：{new_db_path}")
    new_conn = sqlite3.connect(new_db_path)
    new_conn.row_factory = sqlite3.Row

    # Fetch all tags from old DB
    tag_map = {}
    try:
        for row in old_conn.execute("SELECT media_id, tags.name FROM media_tags JOIN tags ON media_tags.tag_id = tags.id"):
            tag_map.setdefault(row["media_id"], []).append(row["name"])
    except Exception as e:
        print(f"[!] 读取旧标签警告：{e}")

    # Query ready media from old DB
    cursor = old_conn.execute(
        "SELECT * FROM media WHERE status = 'ready' AND deleted_at IS NULL ORDER BY id ASC"
    )
    rows = cursor.fetchall()
    print(f"[*] 共发现 {len(rows)} 条就绪媒体记录，准备迁移...")

    migrated_count = 0
    skipped_count = 0
    error_count = 0

    now_ms = int(time.time() * 1000)

    for item in rows:
        old_id = item["id"]
        rel_path = item["relative_path"] or item["stored_name"]
        if not rel_path:
            skipped_count += 1
            continue

        src_file = old_media_dir / rel_path
        if not src_file.exists():
            print(f"[!] 文件不存在，跳过 #{old_id}: {src_file}")
            skipped_count += 1
            continue

        ext = src_file.suffix.lstrip(".").lower() or "mp4"
        job_id = f"tg-migrated-{old_id}"
        file_id = f"{job_id}.media"
        video_id = f"telegram-{job_id}"
        dest_file = new_library_dir / file_id

        # Check if already imported
        existing = new_conn.execute("SELECT id FROM videos WHERE id = ?", (video_id,)).fetchone()
        if existing and dest_file.exists():
            skipped_count += 1
            continue

        # Copy or hardlink file to 91 library
        if not args.dry_run and not dest_file.exists():
            try:
                os.link(src_file, dest_file)
            except Exception:
                shutil.copy2(src_file, dest_file)

        if existing:
            migrated_count += 1
            continue

        # Build metadata
        raw_caption = item["caption"] or ""
        title = item["original_name"] or f"Media-{old_id}"
        if raw_caption:
            first_line = raw_caption.strip().split("\n")[0].strip()
            if first_line:
                title = first_line[:120]

        uploader = item["source_name"] or ""
        is_image = is_image_ext(ext) or item["kind"] == "image"
        is_x = "X ·" in uploader or (item["source_url"] and "x.com" in item["source_url"])

        tags = ["TG"]
        if is_x:
            tags.append("X")
        if is_image:
            tags.append("图片")
        for custom_tag in tag_map.get(old_id, []):
            if custom_tag not in tags:
                tags.append(custom_tag)

        tags_json = json.dumps(tags, ensure_ascii=False)
        duration_sec = int(item["duration_seconds"] or 0)
        file_size = src_file.stat().st_size
        created_at_ms = int((item["created_at"] or time.time()) * 1000)

        preview_status = "disabled" if is_image else "pending"

        if args.dry_run:
            migrated_count += 1
            if migrated_count <= 5 or migrated_count % 20 == 0:
                print(f"[dry-run] 模拟迁移 #{old_id}: {title} ({ext}) tags={tags}")
            continue

        try:
            with new_conn:
                # 1. Insert video record
                new_conn.execute(
                    """
                    INSERT OR REPLACE INTO videos (
                        id, drive_id, file_id, file_name, title, author,
                        tags, duration_seconds, size_bytes, ext, thumbnail_status,
                        preview_status, views, description, published_at, created_at, updated_at
                    ) VALUES (?, 'telegram', ?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?, 0, ?, ?, ?, ?)
                    """,
                    (
                        video_id,
                        file_id,
                        item["original_name"] or dest_file.name,
                        title,
                        uploader,
                        tags_json,
                        duration_sec,
                        file_size,
                        ext,
                        preview_status,
                        raw_caption,
                        created_at_ms,
                        created_at_ms,
                        now_ms,
                    ),
                )

                # 2. Insert telegram_local_files record
                new_conn.execute(
                    "INSERT OR IGNORE INTO telegram_local_files (file_id, job_id) VALUES (?, ?)",
                    (file_id, job_id),
                )

                # 3. Insert telegram_files record (for dedup)
                if item["telegram_file_id"]:
                    new_conn.execute(
                        """
                        INSERT OR REPLACE INTO telegram_files (
                            bot_id, file_unique_id, file_id, job_id, video_id
                        ) VALUES (0, ?, ?, ?, ?)
                        """,
                        (item["telegram_file_id"], item["telegram_file_id"], job_id, video_id),
                    )

            migrated_count += 1
            if migrated_count % 50 == 0:
                print(f"[+] 已成功迁移 {migrated_count} 个媒体...")

        except Exception as e:
            print(f"[-] 迁移 #{old_id} 失败：{e}", file=sys.stderr)
            error_count += 1

    old_conn.close()
    new_conn.close()

    print("\n================== 迁移完成 ==================")
    print(f"成功迁移: {migrated_count} 个")
    print(f"跳过已有: {skipped_count} 个")
    print(f"迁移失败: {error_count} 个")
    print("媒体文件已映射至 91 库目录，启动 91 服务后将自动开始生成封面。")


if __name__ == "__main__":
    main()
