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
    from telethon import TelegramClient, errors
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
        entity_key = channel_input
        if entity_key.startswith("-100") or entity_key.isdigit() or (entity_key.startswith("-") and entity_key[1:].isdigit()):
            entity_key = int(entity_key)

        try:
            entity = await client.get_entity(entity_key)
        except Exception as e:
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

        if not msg.media:
            await client.disconnect()
            print(json.dumps({
                "type": "error",
                "message": f"消息 [ID: {msg_id}] 不包含视频或文件媒体"
            }), flush=True)
            sys.exit(1)

        caption = msg.message or msg.text or ""
        title = ""
        for line in caption.split("\n"):
            line = line.strip()
            if line:
                title = line
                break

        tags = []
        if caption:
            tags = re.findall(r'#([\w\u4e00-\u9fa5]+)', caption)

        file_name = None
        file_size = 0
        mime_type = "video/mp4"

        if hasattr(msg, "file") and msg.file:
            if msg.file.name:
                file_name = msg.file.name
            if msg.file.size:
                file_size = msg.file.size
            if msg.file.mime_type:
                mime_type = msg.file.mime_type

        if not file_name:
            ext = ".mp4"
            if mime_type:
                guessed_ext = mimetypes.guess_extension(mime_type)
                if guessed_ext:
                    ext = guessed_ext
            file_name = f"tg_{args.channel}_{args.msg_id}{ext}"

        if not title:
            title = f"Telegram-{args.channel}-{args.msg_id}"

        if args.max_size > 0 and file_size > args.max_size:
            await client.disconnect()
            print(json.dumps({
                "type": "error",
                "message": f"文件大小 ({file_size} 字节) 超过单文件上限 ({args.max_size} 字节)"
            }), flush=True)
            sys.exit(1)

        print(json.dumps({
            "type": "meta",
            "filename": file_name,
            "size": file_size,
            "mime": mime_type,
            "title": title,
            "tags": tags
        }), flush=True)

        target_path = os.path.join(out_dir, file_name)

        last_report_time = 0.0
        last_report_bytes = 0

        def progress_callback(current, total):
            nonlocal last_report_time, last_report_bytes
            now = time.time()
            if (total and current >= total) or (now - last_report_time >= 0.5) or (current - last_report_bytes >= 2 * 1024 * 1024):
                last_report_time = now
                last_report_bytes = current
                print(json.dumps({
                    "type": "progress",
                    "bytes": current,
                    "total": total or 0
                }), flush=True)

        downloaded_path = await client.download_media(
            msg,
            file=target_path,
            progress_callback=progress_callback
        )

        await client.disconnect()

        if not downloaded_path or not os.path.exists(downloaded_path):
            print(json.dumps({
                "type": "error",
                "message": "媒体下载完成但目标文件不存在"
            }), flush=True)
            sys.exit(1)

        final_size = os.path.getsize(downloaded_path)
        print(json.dumps({
            "type": "done",
            "path": os.path.abspath(downloaded_path),
            "size": final_size,
            "filename": os.path.basename(downloaded_path),
            "title": title,
            "tags": tags
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
