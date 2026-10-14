package cookiemanager

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCookieManagerLifecycle(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	legDir := t.TempDir()

	mgr, err := New(Config{
		DataDir:    dataDir,
		LegacyDirs: []string{legDir},
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// 1. Initial list should have 6 platforms, none configured
	list, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 6 {
		t.Fatalf("expected 6 platforms, got %d", len(list))
	}
	for _, p := range list {
		if p.Configured {
			t.Fatalf("platform %s should not be configured initially", p.Platform)
		}
	}

	// 2. Validate format rejects HTML
	htmlContent := `<!DOCTYPE html><html><body>Error page</body></html>`
	if err := ValidateNetscapeFormat(htmlContent); err == nil {
		t.Fatalf("expected error for HTML content")
	}

	// 3. Save valid Netscape cookie for youtube
	validCookie := `# Netscape HTTP Cookie File
.youtube.com	TRUE	/	TRUE	1791881939	VISITOR_INFO1_LIVE	sample_val
.youtube.com	TRUE	/	TRUE	1791881939	LOGIN_INFO	sample_token
`
	if err := mgr.SaveContent(ctx, "youtube", validCookie); err != nil {
		t.Fatalf("SaveContent youtube failed: %v", err)
	}

	// Verify status updated
	list, err = mgr.List(ctx)
	if err != nil {
		t.Fatalf("List after save failed: %v", err)
	}
	var ytStatus PlatformCookieStatus
	for _, p := range list {
		if p.Platform == "youtube" {
			ytStatus = p
			break
		}
	}
	if !ytStatus.Configured || ytStatus.LineCount != 2 {
		t.Fatalf("unexpected youtube status: %+v", ytStatus)
	}
	if ytStatus.SHA256 == "" {
		t.Fatalf("SHA256 should not be empty")
	}

	// 4. Verify mirrored to legacy dir
	legFile := filepath.Join(legDir, "cookies_youtube.txt")
	if data, err := os.ReadFile(legFile); err != nil || len(data) == 0 {
		t.Fatalf("expected legacy mirror file to exist: %v", err)
	}

	// 5. Update content and verify backup created
	updatedCookie := `# Netscape HTTP Cookie File
.youtube.com	TRUE	/	TRUE	1891881939	VISITOR_INFO1_LIVE	updated_val
`
	if err := mgr.SaveContent(ctx, "youtube", updatedCookie); err != nil {
		t.Fatalf("SaveContent update failed: %v", err)
	}

	// Verify HasBackup is true
	list, _ = mgr.List(ctx)
	for _, p := range list {
		if p.Platform == "youtube" {
			if !p.HasBackup {
				t.Fatalf("expected HasBackup to be true after update")
			}
			break
		}
	}

	// 6. Test rollback
	if err := mgr.Restore(ctx, "youtube"); err != nil {
		t.Fatalf("Restore failed: %v", err)
	}
	restored, err := mgr.GetContent(ctx, "youtube")
	if err != nil {
		t.Fatalf("GetContent after restore failed: %v", err)
	}
	if !stringContains(restored, "sample_token") {
		t.Fatalf("restored content does not match original backup: %s", restored)
	}

	// 7. Delete cookie
	if err := mgr.Delete(ctx, "youtube"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	list, _ = mgr.List(ctx)
	for _, p := range list {
		if p.Platform == "youtube" && p.Configured {
			t.Fatalf("youtube should not be configured after delete")
		}
	}
}

func stringContains(s, sub string) bool {
	return filepath.Clean(s) != "" && (len(s) >= len(sub)) && (s == sub || len(sub) == 0 || (len(s) > 0 && len(sub) > 0 && containsSub(s, sub)))
}

func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
