package export

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sessionJSONL = `{"type":"custom-title","customTitle":"Test session","sessionId":"sess-1234-abcd"}
{"type":"user","sessionId":"sess-1234-abcd","timestamp":"2026-05-01T12:00:00Z","gitBranch":"main","cwd":"/repo","message":{"role":"user","content":"Investigate the widget. It is broken."}}
{"type":"assistant","sessionId":"sess-1234-abcd","timestamp":"2026-05-01T12:01:00Z","gitBranch":"main","message":{"role":"assistant","content":[{"type":"text","text":"Looking into it."}]}}
`

func setupProject(t *testing.T) string {
	t.Helper()
	cfg := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)
	encoded := "-Users-me-proj-widgetapp"
	dir := filepath.Join(cfg, "projects", encoded)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sess-1234-abcd.jsonl"), []byte(sessionJSONL), 0o644); err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestExportEndToEnd(t *testing.T) {
	encoded := setupProject(t)
	target := t.TempDir()

	cfg := Config{
		ProjectArg: encoded,
		TargetDir:  target,
		RootName:   "transcripts",
		Location:   time.UTC,
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	res, err := cfg.Run()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// root README + month README + 1 session doc.
	if len(res.Created) != 3 {
		t.Fatalf("created = %v, want 3 docs", res.Created)
	}

	// Session at 2026-05-01T12:00:00Z (UTC) → month folder + date-time filename.
	sessionDoc := filepath.Join(target, "2026-05", "2026-05-01 1200 test-session.md")
	body := readFile(t, sessionDoc)
	for _, want := range []string{
		`title: "Test session"`,
		`document_kind: "session_transcript"`,
		`generated_by: "session-transcript-exporter"`,
		"conversation:sess-1234-abcd",
		"branch:main",
		`managed_root: "transcripts"`,
		"# Test session",
		"Investigate the widget.", // synopsis lead paragraph (indexed summary)
		"Looking into it.",        // assistant prose
		"> ⎇ On branch `main`",    // inline branch marker
	} {
		if !strings.Contains(body, want) {
			t.Errorf("session doc missing %q", want)
		}
	}

	overview := readFile(t, filepath.Join(target, "README.md"))
	if !strings.Contains(overview, `document_kind: "transcript_root_overview"`) {
		t.Error("root overview missing kind")
	}
	if !strings.Contains(overview, "2026-05/README.md") {
		t.Error("root overview should link the month index")
	}

	// Idempotency: a second run changes nothing.
	res2, err := cfg.Run()
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Created) != 0 || len(res2.Updated) != 0 || len(res2.Deleted) != 0 {
		t.Errorf("second run not idempotent: %+v", res2)
	}
	if len(res2.Unchanged) != 3 {
		t.Errorf("second run unchanged = %d, want 3", len(res2.Unchanged))
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
