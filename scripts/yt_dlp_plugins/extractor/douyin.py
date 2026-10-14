# coding: utf-8
import hashlib
import time
from urllib.parse import quote
from yt_dlp.extractor.tiktok import DouyinIE
from yt_dlp.utils import ExtractorError, traverse_obj


class DouyinFixIE(DouyinIE, plugin_name="douyin_fix"):
    """yt-dlp extractor plugin overriding Douyin extraction with ArgusSecurityPlugin signature."""

    SALT = "A96D855A08C0A9707F8BEF0D9A527E4E"

    def _real_extract(self, url):
        video_id = self._match_id(url)
        cookies = self._get_cookies(self._WEBPAGE_HOST)

        uifid = None
        for k in ["UIFID", "UIFID_TEMP", "uifid", "uifid_temp"]:
            c = cookies.get(k)
            if c and c.value:
                uifid = c.value
                break

        if not uifid:
            self._download_webpage(
                self._WEBPAGE_HOST, video_id, "Acquiring visitor tokens", fatal=False
            )
            cookies = self._get_cookies(self._WEBPAGE_HOST)
            for k in ["UIFID", "UIFID_TEMP", "uifid", "uifid_temp"]:
                c = cookies.get(k)
                if c and c.value:
                    uifid = c.value
                    break

        if not uifid:
            raise ExtractorError("缺少 UIFID 凭证，请配置有效的 Douyin Cookies")

        verify_fp_cookie = cookies.get("s_v_web_id")
        verify_fp = verify_fp_cookie.value if verify_fp_cookie else ""

        pairs = [
            ("device_platform", "webapp"),
            ("aid", "6383"),
            ("channel", "channel_pc_web"),
            ("aweme_id", video_id),
        ]
        if verify_fp:
            pairs.append(("verifyFp", verify_fp))
            pairs.append(("fp", verify_fp))

        stamp = str(int(time.time()))
        covered = list(pairs)
        covered.append(("uifid", uifid))
        covered.append(("timestamp", stamp))
        query = "&".join(f"{quote(k, safe='*-._')}={quote(v, safe='*-._')}" for k, v in covered)
        sig = hashlib.md5(f"{uifid}_{stamp}_{self.SALT}_{query}".encode()).hexdigest()

        full_url = f"https://www.douyin.com/aweme/v1/web/aweme/detail/?{query}&x-secsdk-web-signature={sig}"
        headers = {
            "Referer": "https://www.douyin.com/",
            "uifid": uifid,
            "x-secsdk-web-signature": sig,
            "x-secsdk-web-expire": stamp,
        }

        res = self._download_json(
            full_url,
            video_id,
            headers=headers,
            note="Downloading signed web detail JSON",
            errnote="Failed to download web detail JSON",
            fatal=False,
        )

        detail = traverse_obj(res, ("aweme_detail", {dict}))
        if not detail:
            raise ExtractorError("无法获取作品详情，可能该视频已被删除、设为私密或需要更新 Cookies")

        return self._parse_aweme_video_app(detail)
