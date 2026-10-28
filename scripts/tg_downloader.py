#!/usr/bin/env python3
"""
Telegram MTProto Downloader and Client Helper
Uses Telethon to interact with Telegram's MTProto User API.
Supports testing connection, logging in / generating session,
and downloading media files from channels/messages with progress reporting.
"""

import argparse
import asyncio
import json
import mimetypes
import os
import re
import sys
import time

try:
    from telethon import TelegramClient, errors, functions
    from telethon.sessions import StringSession
except ImportError:
    print(json.dumps({
        "ok": False,
        "type": "error",
        "message": "Telethon 未安装，请在 Python 环境中执行 pip install telethon"
    }), flush=True)
    sys.exit(1)


def parse_args():
    parser = argparse.ArgumentParser(description="Telegram MTProto CLI Helper")
    subparsers = parser.add_subparsers(dest="command", required=True)

    # Subcommand: test
    test_p = subparsers.add_parser("test", help="Test MTProto User Client credentials")
    test_p.add_argument("--api-id", required=True, help="Telegram API ID")
    test_p.add_argument("--api-hash", required=True, help="Telegram API Hash")
    test_p.add_argument("--session", required=True, help="StringSession string")

    # Subcommand: send-code
    send_p = subparsers.add_parser("send-code", help="Send authentication code to phone")
    send_p.add_argument("--api-id", required=True, help="Telegram API ID")
    send_p.add_argument("--api-hash", required=True, help="Telegram API Hash")
    send_p.add_argument("--phone", required=True, help="Phone number with country code")

    # Subcommand: sign-in
    sign_p = subparsers.add_parser("sign-in", help="Sign in with code and optional 2FA password")
    sign_p.add_argument("--api-id", required=True, help="Telegram API ID")
    sign_p.add_argument("--api-hash", required=True, help="Telegram API Hash")
    sign_p.add_argument("--phone", required=True, help="Phone number")
    sign_p.add_argument("--code", required=True, help="Verification code")
    sign_p.add_argument("--phone-code-hash", required=True, help="Phone code hash from send-code")
    sign_p.add_argument("--session", required=True, help="Session string from send-code")
    sign_p.add_argument("--password", default=None, help="2FA cloud password if enabled")

    # Subcommand: download
    dl_p = subparsers.add_parser("download", help="Download media from a channel message")
    dl_p.add_argument("--api-id", required=True, help="Telegram API ID")
    dl_p.add_argument("--api-hash", required=True, help="Telegram API Hash")
    dl_p.add_argument("--session", required=True, help="StringSession string")
    dl_p.add_argument("--channel", required=True, help="Channel username or -100 ID")
    dl_p.add_argument("--msg-id", type=int, required=True, help="Message ID")
    dl_p.add_argument("--out-dir", required=True, help="Destination directory for download")
    dl_p.add_argument("--max-size", type=int, default=0, help="Maximum allowed file size in bytes")

    return parser.parse_args()


async def do_test(args):
    try:
        api_id = int(str(args.api_id).strip())
        api_hash = str(args.api_hash).strip()
        session_str = str(args.session).strip()

        client = TelegramClient(StringSession(session_str), api_id, api_hash)
        await client.connect()
        if not await client.is_user_authorized():
            await client.disconnect()
            print(json.dumps({
                "ok": False,
                "error": "Session 已失效或未完成账号授权"
            }), flush=True)
            sys.exit(1)

        me = await client.get_me()
        await client.disconnect()
        print(json.dumps({
            "ok": True,
            "user": {
                "id": me.id,
                "first_name": me.first_name or "",
                "last_name": me.last_name or "",
                "username": me.username or "",
                "phone": me.phone or ""
            }
        }), flush=True)
    except Exception as e:
        print(json.dumps({
            "ok": False,
            "error": f"测试连接失败: {str(e)}"
        }), flush=True)
        sys.exit(1)


async def do_send_code(args):
    try:
        api_id = int(str(args.api_id).strip())
        api_hash = str(args.api_hash).strip()
        phone = str(args.phone).strip()

        client = TelegramClient(StringSession(""), api_id, api_hash)
        await client.connect()
        result = await client.send_code_request(phone)
        session_str = client.session.save()
        await client.disconnect()

        print(json.dumps({
            "ok": True,
            "phone_code_hash": result.phone_code_hash,
            "session": session_str
        }), flush=True)
    except Exception as e:
        print(json.dumps({
            "ok": False,
            "error": f"发送验证码失败: {str(e)}"
        }), flush=True)
        sys.exit(1)


async def do_sign_in(args):
    try:
        api_id = int(str(args.api_id).strip())
        api_hash = str(args.api_hash).strip()
        phone = str(args.phone).strip()
        code = str(args.code).strip()
        phone_code_hash = str(args.phone_code_hash).strip()
        session_str = str(args.session).strip()
        password = args.password

        client = TelegramClient(StringSession(session_str), api_id, api_hash)
        await client.connect()

        try:
            await client.sign_in(phone=phone, code=code, phone_code_hash=phone_code_hash)
        except errors.SessionPasswordNeededError:
            if password:
                await client.sign_in(password=password)
            else:
                updated_session = client.session.save()
                await client.disconnect()
                print(json.dumps({
                    "ok": False,
                    "needs_password": True,
                    "session": updated_session,
                    "error": "该账号开启了两步验证，请输入两步验证密码 (2FA)"
                }), flush=True)
                return

        me = await client.get_me()
        final_session = client.session.save()
        await client.disconnect()

        print(json.dumps({
            "ok": True,
            "session": final_session,
            "user": {
                "id": me.id,
                "first_name": me.first_name or "",
                "last_name": me.last_name or "",
                "username": me.username or "",
                "phone": me.phone or ""
            }
        }), flush=True)
    except Exception as e:
        print(json.dumps({
            "ok": False,
            "error": f"登录失败: {str(e)}"
        }), flush=True)
        sys.exit(1)


def is_video_msg(m):
    if not m or not getattr(m, "media", None):
        return False
    if getattr(m, "video", None):
        return True
    if getattr(m, "document", None):
        mime = (getattr(getattr(m, "file", None), "mime_type", "") or "").lower()
        name = (getattr(getattr(m, "file", None), "name", "") or "").lower()
        if mime.startswith("video/") or name.endswith((".mp4", ".mkv", ".mov", ".webm", ".avi", ".ts", ".flv")):
            return True
    return False


def is_photo_msg(m):
    if not m or not getattr(m, "media", None):
        return False
    if getattr(m, "photo", None):
        return True
    if getattr(m, "document", None):
        mime = (getattr(getattr(m, "file", None), "mime_type", "") or "").lower()
        name = (getattr(getattr(m, "file", None), "name", "") or "").lower()
        if mime.startswith("image/") or name.endswith((".jpg", ".jpeg", ".png", ".webp", ".bmp", ".gif", ".heic")):
            return True
    return False


async def do_download(args):
    try:
        api_id = int(str(args.api_id).strip())
        api_hash = str(args.api_hash).strip()
        session_str = str(args.session).strip()
        channel_input = str(args.channel).strip()
        msg_id = int(args.msg_id)
        out_dir = os.path.abspath(args.out_dir)
        os.makedirs(out_dir, exist_ok=True)

        client = TelegramClient(StringSession(session_str), api_id, api_hash)
        await client.connect()
        if not await client.is_user_authorized():
            await client.disconnect()
            print(json.dumps({
                "type": "error",
                "message": "MTProto 客户端 Session 已失效或未授权"
            }), flush=True)
            sys.exit(1)

        # Parse channel / entity
        entity = None
        try:
            entity_key = channel_input
            if entity_key.startswith("-100") or entity_key.isdigit() or (entity_key.startswith("-") and entity_key[1:].isdigit()):
                entity_key = int(entity_key)
            entity = await client.get_entity(entity_key)
        except Exception as e:
            if str(channel_input).isdigit() and not str(channel_input).startswith("-"):
                try:
                    entity = await client.get_entity(int(f"-100{channel_input}"))
                except Exception:
                    pass
            if not entity:
                await client.disconnect()
                print(json.dumps({
                    "type": "error",
                    "message": f"无法获取频道/群组 [{channel_input}]: {str(e)}"
                }), flush=True)
                sys.exit(1)

        try:
            msg = await client.get_messages(entity, ids=msg_id)
        except Exception as e:
            await client.disconnect()
            print(json.dumps({
                "type": "error",
                "message": f"无法读取消息 [ID: {msg_id}]: {str(e)}"
            }), flush=True)
            sys.exit(1)

        if not msg:
            await client.disconnect()
            print(json.dumps({
                "type": "error",
                "message": f"消息不存在或已被删除 [ID: {msg_id}]"
            }), flush=True)
            sys.exit(1)

        # 1. 检查帖子本身是否为媒体或相册 (Media Group / Album)
        post_media_msgs = []
        grouped_id = getattr(msg, "grouped_id", None)
        if grouped_id:
            try:
                candidate_ids = list(range(max(1, msg_id - 10), msg_id + 11))
                surrounding = await client.get_messages(entity, ids=candidate_ids)
                post_media_msgs = [
                    m for m in surrounding
                    if m and getattr(m, "grouped_id", None) == grouped_id and getattr(m, "media", None)
                ]
            except Exception:
                post_media_msgs = []

        if not post_media_msgs and getattr(msg, "media", None):
            post_media_msgs = [msg]

        post_media_msgs.sort(key=lambda x: x.id)

        # 2. 检查关联讨论组 / 评论区 (Discussion Group / Comments) 中的媒体（视频与图片）
        comment_video_msgs = []
        comment_photo_msgs = []
        try:
            discussion = await client(functions.messages.GetDiscussionMessageRequest(
                peer=entity,
                msg_id=msg_id
            ))
            if discussion and discussion.messages and discussion.chats:
                top_msg = discussion.messages[0]
                discussion_group = discussion.chats[0]
                # 遍历评论区回复（通常资源在最新的若干条回复中，取前100条）
                replies = await client.get_messages(discussion_group, reply_to=top_msg.id, limit=100)
                for r in replies:
                    if not r or not getattr(r, "media", None):
                        continue
                    if is_video_msg(r):
                        comment_video_msgs.append(r)
                    elif is_photo_msg(r):
                        comment_photo_msgs.append(r)
        except Exception:
            comment_video_msgs = []
            comment_photo_msgs = []

        # 按消息编号正序排列（保证多集视频/多张图片按自然发送顺序排列）
        comment_video_msgs.sort(key=lambda x: x.id)
        comment_photo_msgs.sort(key=lambda x: x.id)

        # 3. 确定最终下载的目标消息列表 targets
        post_videos = [m for m in post_media_msgs if is_video_msg(m)]
        post_photos = [m for m in post_media_msgs if is_photo_msg(m)]

        targets = []
        if comment_video_msgs or comment_photo_msgs:
            # 优先将所有视频排在前面，图片排在后面（保证存在视频时，首个文件必定为视频，作为主视频）
            all_videos = post_videos + comment_video_msgs
            all_photos = post_photos + comment_photo_msgs
            targets = all_videos + all_photos
        else:
            targets = list(post_media_msgs)

        if not targets:
            await client.disconnect()
            print(json.dumps({
                "type": "error",
                "message": f"消息 [ID: {msg_id}] 及其留言评论区均不包含视频或图片等媒体内容"
            }), flush=True)
            sys.exit(1)

        # 提取标题和标签（优先从主消息提取，主消息通常包含完整的标题介绍和 #标签）
        caption = ""
        for m in [msg] + post_media_msgs + comment_video_msgs + comment_photo_msgs:
            if m:
                c = getattr(m, "message", None) or getattr(m, "text", None) or ""
                if c.strip():
                    caption = c.strip()
                    break

        title = ""
        for line in caption.split("\n"):
            line = line.strip()
            if line:
                title = line
                break

        tags = []
        if caption:
            tags = re.findall(r'#([\w\u4e00-\u9fa5]+)', caption)

        if not title:
            title = f"Telegram-{args.channel}-{args.msg_id}"

        downloaded_files = []
        total_items = len(targets)

        for idx, m in enumerate(targets):
            is_photo = False
            if getattr(m, "photo", None):
                is_photo = True

            file_name = None
            file_size = 0
            mime_type = "image/jpeg" if is_photo else "video/mp4"

            if hasattr(m, "file") and m.file:
                if m.file.name:
                    file_name = m.file.name
                if m.file.size:
                    file_size = m.file.size
                if m.file.mime_type:
                    mime_type = m.file.mime_type
                    if mime_type.startswith("image/"):
                        is_photo = True

            if not file_name:
                ext = ".jpg" if is_photo else ".mp4"
                if mime_type:
                    guessed_ext = mimetypes.guess_extension(mime_type)
                    if guessed_ext:
                        ext = guessed_ext
                        if ext == ".jpe":
                            ext = ".jpg"
                file_name = f"tg_{args.channel}_{m.id}{ext}"

            if args.max_size > 0 and file_size > args.max_size:
                continue

            clean_name = re.sub(r'[\\/*?:"<>|]', '_', file_name).strip()
            if not clean_name:
                clean_name = f"tg_{args.channel}_{m.id}.mp4"
            target_path = os.path.join(out_dir, f"{m.id}_{clean_name}")

            last_report_time = 0.0
            last_report_bytes = 0

            def make_progress_cb(item_idx):
                def cb(current, total):
                    nonlocal last_report_time, last_report_bytes
                    now = time.time()
                    if (total and current >= total) or (now - last_report_time >= 0.5) or (current - last_report_bytes >= 2 * 1024 * 1024):
                        last_report_time = now
                        last_report_bytes = current
                        print(json.dumps({
                            "type": "progress",
                            "bytes": current,
                            "total": total or 0,
                            "item_index": item_idx,
                            "total_items": total_items
                        }), flush=True)
                return cb

            downloaded_path = await client.download_media(
                m,
                file=target_path,
                progress_callback=make_progress_cb(idx + 1)
            )

            if downloaded_path and os.path.exists(downloaded_path):
                actual_size = os.path.getsize(downloaded_path)
                downloaded_files.append({
                    "id": m.id,
                    "path": os.path.abspath(downloaded_path),
                    "filename": clean_name,
                    "size": actual_size,
                    "mime": mime_type,
                    "is_photo": is_photo
                })

        await client.disconnect()

        if not downloaded_files:
            print(json.dumps({
                "type": "error",
                "message": "媒体下载完成但未生成任何有效文件"
            }), flush=True)
            sys.exit(1)

        # 优先选择视频作为 primary；若无视频才选 msg_id 对应文件；最后选第一个
        primary = None
        for f in downloaded_files:
            if not f["is_photo"]:
                primary = f
                break
        if not primary:
            for f in downloaded_files:
                if f["id"] == msg_id:
                    primary = f
                    break
        if not primary:
            primary = downloaded_files[0]

        print(json.dumps({
            "type": "done",
            "path": primary["path"],
            "size": primary["size"],
            "filename": primary["filename"],
            "mime": primary["mime"],
            "is_photo": primary["is_photo"],
            "title": title,
            "tags": tags,
            "files": downloaded_files
        }), flush=True)

    except Exception as e:
        print(json.dumps({
            "type": "error",
            "message": f"下载失败: {str(e)}"
        }), flush=True)
        sys.exit(1)


def main():
    args = parse_args()
    if args.command == "test":
        asyncio.run(do_test(args))
    elif args.command == "send-code":
        asyncio.run(do_send_code(args))
    elif args.command == "sign-in":
        asyncio.run(do_sign_in(args))
    elif args.command == "download":
        asyncio.run(do_download(args))


if __name__ == "__main__":
    main()
