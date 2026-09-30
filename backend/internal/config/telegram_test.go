package config

import (
	"encoding/json"
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
