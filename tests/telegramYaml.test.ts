import assert from "node:assert/strict";
import test from "node:test";
import { applyVisualFields, changedVisualFields, configDocument, parseConfig } from "../src/admin/settings/configYaml";

test("Telegram settings and credentials round-trip through the shared YAML draft", () => {
  for (const source of ["# keep\npreview: { enabled: true }\n", "{}\n", "telegram:\n", "telegram: null\n", "telegram: {}\n"]) {
    const before = parseConfig(source).draft;
    const draft = { ...before, telegramEnabled: true, telegramBotToken: "123:test_token", telegramAllowedUserIds: "123, 456",
      telegramMaxPendingJobs: 25 };
    const output = applyVisualFields(source, draft, changedVisualFields(before, draft));
    assert.deepEqual(parseConfig(output).draft, draft);
    assert.equal(configDocument(output).getIn(["telegram", "bot_token"]), draft.telegramBotToken);
    assert.equal(configDocument(output).hasIn(["telegram", "api_base_url"]), false);
    assert.equal(configDocument(output).hasIn(["telegram", "api_id"]), false);
    assert.equal(configDocument(output).hasIn(["telegram", "api_hash"]), false);
    assert.equal(configDocument(output).hasIn(["telegram", "api_files_root"]), false);
    assert.equal(configDocument(output).hasIn(["telegram", "local_files_root"]), false);
    if (source.startsWith("# keep")) assert.ok(output.startsWith("# keep\npreview: { enabled: true }\n"));
  }
});

test("Telegram edits preserve unrelated comments, keys, and concurrent field changes", () => {
  const source = "# top\ntelegram:\n  bot_token: '123:old' # bot\n  api_hash: '0123456789abcdef0123456789abcdef'\n  max_pending_jobs: 10 # jobs\n  allowed_user_ids: [123] # users\n  unknown_key: keep\npreview:\n  width: 480\n";
  const before = parseConfig(source).draft;
  const draft = { ...before, telegramBotToken: "123:new" };
  const latest = source.replace("max_pending_jobs: 10", "max_pending_jobs: 25");
  const output = applyVisualFields(latest, draft, changedVisualFields(before, draft));
  assert.equal(output, latest.replace("'123:old'", "'123:new'"));
});

test("Telegram lists can be edited in block and flow YAML without damaging the next field", () => {
  for (const source of [
    "telegram:\n  allowed_user_ids:\n    - 123\n    - 456\n  site_base_url: https://example.com\n",
    "telegram: { allowed_user_ids: [123], site_base_url: https://example.com }\n",
    "telegram:\n  allowed_user_ids: # empty\n  site_base_url: https://example.com\n",
  ]) {
    const before = parseConfig(source).draft;
    const draft = { ...before, telegramAllowedUserIds: "789, 123" };
    const output = applyVisualFields(source, draft, new Set(["telegramAllowedUserIds"]));
    assert.deepEqual(parseConfig(output).draft, draft);
  }
});

test("clearing the Telegram Bot Token writes empty values instead of retaining old secrets", () => {
  const source = "telegram: { bot_token: '123:old', api_hash: '0123456789abcdef0123456789abcdef' }\n";
  const before = parseConfig(source).draft;
  const draft = { ...before, telegramBotToken: "" };
  assert.deepEqual(parseConfig(applyVisualFields(source, draft, changedVisualFields(before, draft))).draft, draft);
});

test("Telegram YAML rejects mismatched types and invalid user ID lists", () => {
  for (const source of ["telegram: []", "telegram: { enabled: yes }", "telegram: { bot_token: 123 }", "telegram: { allowed_user_ids: [-1] }", "telegram: { max_pending_jobs: 'lots' }"]) {
    assert.throws(() => parseConfig(source), /telegram/);
  }
});
