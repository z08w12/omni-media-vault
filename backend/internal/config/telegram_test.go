package config

import (
	"os"
	"strings"
	"testing"
)

func TestTelegramConfigValidation(t *testing.T) {
	for _, input := range []string{
		"telegram:\n  bot_token: secret\n",
		"telegram:\n  enabled: true\n",
		"telegram:\n  max_pending_jobs: -1\n",
		"telegram:\n  allowed_user_ids: [-1]\n",
		"telegram:\n  fetch_timeout_seconds: 1\n",
	} {
		if _, err := Parse([]byte(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	cfg, err := Parse([]byte("telegram:\n  enabled: true\n  bot_token: 123:test_token\n  allowed_user_ids: []\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Telegram.MaxFileSizeBytes != 4<<30 {
		t.Fatalf("defaults: %+v", cfg.Telegram)
	}
}

func TestUpdateTelegramUserClientSession(t *testing.T) {
	manager, path := newManagerForTest(t, `# header comment
telegram:
  enabled: true
  bot_token: 123:test_token
  allowed_user_ids: []
  user_client_enabled: false
  api_id: "11111"
  api_hash: "oldhash"
  session_string: ""
`)

	err := manager.UpdateTelegramUserClientSession("1BXYZ_session_token", "22222", "newhash")
	if err != nil {
		t.Fatalf("UpdateTelegramUserClientSession failed: %v", err)
	}

	settings := manager.TelegramSettings()
	if !settings.UserClientEnabled {
		t.Fatalf("expected UserClientEnabled to be true")
	}
	if settings.SessionString != "1BXYZ_session_token" {
		t.Fatalf("expected session string to be updated, got %q", settings.SessionString)
	}
	if settings.APIID != "22222" {
		t.Fatalf("expected api_id to be updated, got %q", settings.APIID)
	}
	if settings.APIHash != "newhash" {
		t.Fatalf("expected api_hash to be updated, got %q", settings.APIHash)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "# header comment") {
		t.Fatalf("comments were discarded:\n%s", text)
	}
	if !strings.Contains(text, "1BXYZ_session_token") {
		t.Fatalf("session_string not found in file:\n%s", text)
	}
}

