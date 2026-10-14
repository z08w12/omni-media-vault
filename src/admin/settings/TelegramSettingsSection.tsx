import { useState } from "react";
import { TelegramIcon } from "@/components/icons/TelegramIcon";
import { SettingsSection } from "./SettingsSection";
import type { SettingsDraft, VisualField } from "./configYaml";
import "@/styles/telegram.css";

type Props = {
  active: boolean;
  disabled: boolean;
  draft: SettingsDraft;
  onChange: <Field extends VisualField>(
    field: Field,
    value: SettingsDraft[Field],
  ) => void;
};

export function TelegramSettingsSection({
  active,
  disabled,
  draft,
  onChange,
}: Props) {
  const [testingClient, setTestingClient] = useState(false);
  const [testResult, setTestResult] = useState<{ ok: boolean; msg: string } | null>(null);

  const [showLoginAssistant, setShowLoginAssistant] = useState(false);
  const [loginPhone, setLoginPhone] = useState("");
  const [phoneCodeHash, setPhoneCodeHash] = useState("");
  const [loginSession, setLoginSession] = useState("");
  const [loginCode, setLoginCode] = useState("");
  const [loginPassword, setLoginPassword] = useState("");
  const [needsPassword, setNeedsPassword] = useState(false);
  const [sendingCode, setSendingCode] = useState(false);
  const [signingIn, setSigningIn] = useState(false);
  const [loginStatus, setLoginStatus] = useState<{ ok: boolean; msg: string } | null>(null);

  const handleTestClient = async () => {
    setTestingClient(true);
    setTestResult(null);
    try {
      const resp = await fetch("/admin/api/telegram/user-client/test", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          apiId: draft.telegramApiId,
          apiHash: draft.telegramApiHash,
          session: draft.telegramSessionString,
        }),
      });
      const data = await resp.json();
      if (!resp.ok || !data.ok) {
        setTestResult({ ok: false, msg: `连接失败: ${data.error || "凭据无效"}` });
      } else {
        const u = data.user;
        const nameStr = [u?.firstName, u?.lastName].filter(Boolean).join(" ");
        const userDisplay = u?.username ? `@${u.username}` : nameStr || `ID: ${u?.id || ""}`;
        setTestResult({ ok: true, msg: `连接成功：已登录为 ${userDisplay} (ID: ${u?.id || ""})` });
      }
    } catch (err) {
      setTestResult({ ok: false, msg: `请求失败: ${err instanceof Error ? err.message : String(err)}` });
    } finally {
      setTestingClient(false);
    }
  };

  const handleSendCode = async () => {
    setSendingCode(true);
    setLoginStatus(null);
    try {
      const resp = await fetch("/admin/api/telegram/user-client/send-code", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          apiId: draft.telegramApiId,
          apiHash: draft.telegramApiHash,
          phone: loginPhone.trim(),
        }),
      });
      const data = await resp.json();
      if (!resp.ok || !data.ok) {
        setLoginStatus({ ok: false, msg: `发送失败: ${data.error || "未知错误"}` });
      } else {
        setPhoneCodeHash(data.phoneCodeHash);
        setLoginSession(data.session);
        setLoginStatus({ ok: true, msg: "验证码已发送！请查收 Telegram 消息或手机短信" });
      }
    } catch (err) {
      setLoginStatus({ ok: false, msg: `发送失败: ${err instanceof Error ? err.message : String(err)}` });
    } finally {
      setSendingCode(false);
    }
  };

  const handleSignIn = async () => {
    setSigningIn(true);
    setLoginStatus(null);
    try {
      const resp = await fetch("/admin/api/telegram/user-client/sign-in", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          apiId: draft.telegramApiId,
          apiHash: draft.telegramApiHash,
          phone: loginPhone.trim(),
          code: loginCode.trim(),
          phoneCodeHash,
          session: loginSession,
          password: loginPassword.trim(),
        }),
      });
      const data = await resp.json();
      if (!resp.ok) {
        setLoginStatus({ ok: false, msg: `登录失败: ${data.error || "未知错误"}` });
      } else if (data.needsPassword) {
        setNeedsPassword(true);
        if (data.session) setLoginSession(data.session);
        setLoginStatus({ ok: false, msg: "该账号启用了两步验证 (2FA)，请输入两步验证密码后重试" });
      } else if (data.ok) {
        onChange("telegramSessionString", data.session);
        onChange("telegramUserClientEnabled", true);
        const u = data.user;
        const userDisplay = u?.username ? `@${u.username}` : `ID: ${u?.id || ""}`;
        setLoginStatus({
          ok: true,
          msg: `登录成功！已成功登录为 ${userDisplay} 并自动填入 Session 凭证，请点击页面右上角保存配置。`,
        });
      }
    } catch (err) {
      setLoginStatus({ ok: false, msg: `登录失败: ${err instanceof Error ? err.message : String(err)}` });
    } finally {
      setSigningIn(false);
    }
  };

  const handleResetLogin = () => {
    setPhoneCodeHash("");
    setLoginSession("");
    setLoginCode("");
    setLoginPassword("");
    setNeedsPassword(false);
    setLoginStatus(null);
  };

  return (
    <div className="admin-telegram-settings" hidden={!active}>
      <SettingsSection
        id="config-telegram"
        index="04"
        icon={<TelegramIcon size={16} />}
        title="Telegram"
        description="管理机器人连接与视频接收"
      >
        <fieldset disabled={disabled || !active}>
          <div className="tg-enable-row">
            <div>
              <label
                id="telegram-enabled-label"
                htmlFor="telegram-enabled-toggle"
              >
                启用 Telegram 机器人
              </label>
            </div>
            <button
              id="telegram-enabled-toggle"
              type="button"
              className={`toggle-switch ${draft.telegramEnabled ? "is-on" : ""}`}
              role="switch"
              aria-checked={draft.telegramEnabled}
              aria-labelledby="telegram-enabled-label"
              disabled={disabled || !active}
              onClick={() =>
                onChange("telegramEnabled", !draft.telegramEnabled)
              }
            >
              <span className="toggle-switch__dot" />
            </button>
          </div>
          <section
            className="tg-setting-group"
            aria-labelledby="tg-credentials-title"
          >
            <h3 id="tg-credentials-title">机器人凭据</h3>
            <div className="tg-fields tg-fields--thirds">
              <label>
                Bot Token
                <input
                  type="text"
                  autoComplete="off"
                  spellCheck={false}
                  value={draft.telegramBotToken}
                  onChange={(e) => onChange("telegramBotToken", e.target.value)}
                  placeholder="填写 BotFather 提供的 Token"
                />
              </label>
            </div>
          </section>

          <section
            className="tg-setting-group"
            aria-labelledby="tg-mtproto-title"
          >
            <div className="tg-enable-row" style={{ paddingBottom: 12 }}>
              <div>
                <h3 id="tg-mtproto-title">Telegram 客户端协议 (MTProto User API / 频道链接解析)</h3>
                <p className="tg-note">
                  开启后支持直接向机器人发送或转存 Telegram 公开/私密频道内的视频链接（如 <code>https://t.me/channel/12345</code>），绕过网页游客拦截限制。
                </p>
              </div>
              <button
                id="telegram-user-client-toggle"
                type="button"
                className={`toggle-switch ${draft.telegramUserClientEnabled ? "is-on" : ""}`}
                role="switch"
                aria-checked={draft.telegramUserClientEnabled}
                aria-labelledby="tg-mtproto-title"
                disabled={disabled || !active}
                onClick={() =>
                  onChange("telegramUserClientEnabled", !draft.telegramUserClientEnabled)
                }
              >
                <span className="toggle-switch__dot" />
              </button>
            </div>

            <div className="tg-fields">
              <label>
                API ID
                <input
                  type="text"
                  autoComplete="off"
                  spellCheck={false}
                  value={draft.telegramApiId}
                  onChange={(e) => onChange("telegramApiId", e.target.value)}
                  placeholder="从 my.telegram.org 申请获取"
                />
              </label>
              <label>
                API Hash
                <input
                  type="text"
                  autoComplete="off"
                  spellCheck={false}
                  value={draft.telegramApiHash}
                  onChange={(e) => onChange("telegramApiHash", e.target.value)}
                  placeholder="32位十六进制字符串"
                />
              </label>
              <label className="tg-field-wide">
                <span>Session 字符串 (StringSession)</span>
                <input
                  type="password"
                  autoComplete="off"
                  spellCheck={false}
                  value={draft.telegramSessionString}
                  onChange={(e) => onChange("telegramSessionString", e.target.value)}
                  placeholder="填入已有的 StringSession，或点击下方「辅助登录」生成"
                />
                <small>该凭据以加密字符串形式保存在本地配置中，用于以客户端身份与 Telegram MTProto 服务器安全通信。</small>
              </label>
            </div>

            <div className="tg-actions" style={{ marginTop: 16 }}>
              <button
                type="button"
                className="admin-btn"
                disabled={disabled || !active || testingClient}
                onClick={() => void handleTestClient()}
              >
                {testingClient ? "正在测试连接…" : "测试客户端连接"}
              </button>
              <button
                type="button"
                className="admin-btn is-primary"
                disabled={disabled || !active}
                onClick={() => setShowLoginAssistant((prev) => !prev)}
              >
                {showLoginAssistant ? "收起辅助登录" : "辅助登录生成 Session"}
              </button>

              {testResult && (
                <span
                  style={{
                    fontSize: 13,
                    marginLeft: 8,
                    color: testResult.ok ? "var(--color-success, #22c55e)" : "var(--color-danger, #ef4444)",
                  }}
                >
                  {testResult.msg}
                </span>
              )}
            </div>

            {showLoginAssistant && (
              <div
                style={{
                  marginTop: 16,
                  padding: "16px 20px",
                  background: "var(--bg-sunken)",
                  borderRadius: 8,
                  border: "1px solid var(--border-default)",
                }}
              >
                <h4 style={{ margin: "0 0 10px", fontSize: 14, fontWeight: 600 }}>
                  MTProto 客户端登录助手
                </h4>
                <p className="tg-note" style={{ margin: "0 0 14px", fontSize: 13 }}>
                  通过输入手机号及验证码，系统将通过 MTProto 官方协议直接与 Telegram 通信并生成 Session 字符串，完成后自动填入上方输入框。请先确保已填写上方的 API ID 与 API Hash。
                </p>

                <div style={{ display: "grid", gap: 12, maxWidth: 540 }}>
                  <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
                    <input
                      type="text"
                      placeholder="手机号 (含国家代码，如 +8613800000000)"
                      value={loginPhone}
                      onChange={(e) => setLoginPhone(e.target.value)}
                      disabled={disabled || sendingCode || Boolean(phoneCodeHash)}
                      style={{
                        flex: 1,
                        padding: "8px 10px",
                        borderRadius: 6,
                        border: "1px solid var(--border-default)",
                        background: "var(--bg-surface)",
                        color: "inherit",
                      }}
                    />
                    {!phoneCodeHash ? (
                      <button
                        type="button"
                        className="admin-btn"
                        disabled={disabled || sendingCode || !loginPhone.trim()}
                        onClick={() => void handleSendCode()}
                      >
                        {sendingCode ? "正在发送…" : "发送验证码"}
                      </button>
                    ) : (
                      <button
                        type="button"
                        className="admin-btn"
                        onClick={handleResetLogin}
                      >
                        重新发送
                      </button>
                    )}
                  </div>

                  {phoneCodeHash && (
                    <>
                      <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
                        <input
                          type="text"
                          placeholder="输入收到的 Telegram 验证码"
                          value={loginCode}
                          onChange={(e) => setLoginCode(e.target.value)}
                          disabled={disabled || signingIn}
                          style={{
                            flex: 1,
                            padding: "8px 10px",
                            borderRadius: 6,
                            border: "1px solid var(--border-default)",
                            background: "var(--bg-surface)",
                            color: "inherit",
                          }}
                        />
                      </div>

                      {needsPassword && (
                        <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
                          <input
                            type="password"
                            placeholder="两步验证密码 (2FA)"
                            value={loginPassword}
                            onChange={(e) => setLoginPassword(e.target.value)}
                            disabled={disabled || signingIn}
                            style={{
                              flex: 1,
                              padding: "8px 10px",
                              borderRadius: 6,
                              border: "1px solid var(--border-default)",
                              background: "var(--bg-surface)",
                              color: "inherit",
                            }}
                          />
                        </div>
                      )}

                      <div>
                        <button
                          type="button"
                          className="admin-btn is-primary"
                          disabled={disabled || signingIn || !loginCode.trim()}
                          onClick={() => void handleSignIn()}
                        >
                          {signingIn ? "正在登录…" : "完成登录并获取 Session"}
                        </button>
                      </div>
                    </>
                  )}

                  {loginStatus && (
                    <div
                      style={{
                        fontSize: 13,
                        marginTop: 4,
                        color: loginStatus.ok ? "var(--color-success, #22c55e)" : "var(--color-danger, #ef4444)",
                      }}
                    >
                      {loginStatus.msg}
                    </div>
                  )}
                </div>
              </div>
            )}
          </section>

          <section
            className="tg-setting-group"
            aria-labelledby="tg-access-title"
          >
            <h3 id="tg-access-title">接收设置</h3>
            <div className="tg-fields tg-fields--thirds">
              <label className="tg-field--single-line">
                <span>允许的用户 ID（机器人仅接收指定用户发送的视频）</span>
                <input
                  value={draft.telegramAllowedUserIds}
                  onChange={(e) =>
                    onChange("telegramAllowedUserIds", e.target.value)
                  }
                  placeholder="123456789, 987654321"
                />
              </label>
              <label>
                站点地址（选填）
                <input
                  type="url"
                  value={draft.telegramSiteBaseUrl}
                  onChange={(e) =>
                    onChange("telegramSiteBaseUrl", e.target.value)
                  }
                  placeholder="https://video.example.com"
                />
              </label>
            </div>
          </section>
          <section
            className="tg-setting-group"
            aria-labelledby="tg-download-title"
          >
            <h3 id="tg-download-title">下载与存储设置</h3>
            <div className="tg-fields tg-fields--thirds">
              <label>
                文件上限（GiB）
                <input
                  type="number"
                  min="0.01"
                  max="16"
                  step="0.01"
                  value={draft.telegramMaxFileSizeBytes / 1024 ** 3}
                  onChange={(e) =>
                    onChange(
                      "telegramMaxFileSizeBytes",
                      Math.round(Number(e.target.value) * 1024 ** 3),
                    )
                  }
                />
              </label>
              <label>
                预留空间阈值（GiB）
                <input
                  type="number"
                  min="0.1"
                  max="1024"
                  step="0.5"
                  value={
                    Math.round(
                      (draft.remoteUploadDiskReserveBytes / 1024 ** 3) * 100,
                    ) / 100
                  }
                  onChange={(e) => {
                    const val = Number(e.target.value);
                    if (val > 0) {
                      onChange(
                        "remoteUploadDiskReserveBytes",
                        Math.round(val * 1024 ** 3),
                      );
                    }
                  }}
                />
              </label>
              <label>
                待处理任务上限
                <input
                  type="number"
                  min="1"
                  max="10000"
                  value={draft.telegramMaxPendingJobs}
                  onChange={(e) =>
                    onChange("telegramMaxPendingJobs", Number(e.target.value))
                  }
                />
              </label>
              <label>
                获取超时（秒）
                <input
                  type="number"
                  min="30"
                  max="86400"
                  value={draft.telegramFetchTimeoutSeconds}
                  onChange={(e) =>
                    onChange(
                      "telegramFetchTimeoutSeconds",
                      Number(e.target.value),
                    )
                  }
                />
              </label>
            </div>
          </section>
        </fieldset>
      </SettingsSection>
    </div>
  );
}
