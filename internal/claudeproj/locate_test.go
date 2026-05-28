package claudeproj

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestEncodePath(t *testing.T) {
	in := "/Users/nugget/Sync/Projects/AI/Claude/thane-ai-agent"
	want := "-Users-nugget-Sync-Projects-AI-Claude-thane-ai-agent"
	if got := encodePath(in); got != want {
		t.Errorf("encodePath = %q, want %q", got, want)
	}
	// A worktree path's dot-segment encodes to the observed "--" form.
	wt := "/Users/nugget/Sync/Projects/AI/Claude/thane-ai-agent/.claude/worktrees/zealous"
	if got := encodePath(wt); got != "-Users-nugget-Sync-Projects-AI-Claude-thane-ai-agent--claude-worktrees-zealous" {
		t.Errorf("worktree encode = %q", got)
	}
}

func TestLocateAndSessionFiles(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", root)
	projects := filepath.Join(root, "projects")

	main := "-Users-me-proj-thane-ai-agent"
	wt := main + "--claude-worktrees-abc"
	other := "-Users-me-proj-other-thing"
	for _, name := range []string{main, wt, other} {
		if err := os.MkdirAll(filepath.Join(projects, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Sessions: two in main, one in worktree, plus a subagent sidecar dir.
	writeFile(t, filepath.Join(projects, main, "b.jsonl"), "{}")
	writeFile(t, filepath.Join(projects, main, "a.jsonl"), "{}")
	writeFile(t, filepath.Join(projects, wt, "c.jsonl"), "{}")
	if err := os.MkdirAll(filepath.Join(projects, main, "a", "subagents"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(projects, main, "a", "subagents", "agent.jsonl"), "{}")

	t.Run("merge worktrees", func(t *testing.T) {
		proj, err := Locate(main, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(proj.SourceDirs) != 2 {
			t.Fatalf("SourceDirs = %v, want main + worktree", proj.SourceDirs)
		}
		files, err := proj.SessionFiles()
		if err != nil {
			t.Fatal(err)
		}
		// a.jsonl, b.jsonl (main), c.jsonl (worktree); sidecar excluded.
		if len(files) != 3 {
			t.Fatalf("SessionFiles = %v, want 3 (sidecar excluded)", files)
		}
		if !sort.StringsAreSorted(files) {
			t.Errorf("files not in deterministic sorted order: %v", files)
		}
		for _, f := range files {
			if strings.Contains(f, "subagents") {
				t.Errorf("sidecar leaked into session files: %s", f)
			}
		}
	})

	t.Run("exclude worktrees", func(t *testing.T) {
		proj, err := Locate(main, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(proj.SourceDirs) != 1 {
			t.Errorf("SourceDirs = %v, want main only", proj.SourceDirs)
		}
	})

	t.Run("match by name", func(t *testing.T) {
		proj, err := Locate("thane-ai-agent", true)
		if err != nil {
			t.Fatal(err)
		}
		if proj.MainEncoded != main {
			t.Errorf("MainEncoded = %q, want %q", proj.MainEncoded, main)
		}
	})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
