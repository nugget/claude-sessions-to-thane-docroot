package render

import (
	"strings"
	"testing"
	"time"

	"github.com/nugget/session-transcript-exporter/internal/transcript"
)

func TestRenderSessionBranchMarkers(t *testing.T) {
	start := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	s := &transcript.Session{
		ID:          "abcd1234-xxxx",
		CustomTitle: "Interleaved work",
		FirstPrompt: "Do the first thing.",
		StartedAt:   start,
		EndedAt:     start.Add(time.Hour),
		Branches: []transcript.BranchStat{
			{Branch: "main", Count: 2},
			{Branch: "feature/y", Count: 1},
		},
		UserTurns: 1,
		Events: []transcript.Event{
			{Kind: transcript.KindUserPrompt, Branch: "main", Text: "Do the first thing."},
			{Kind: transcript.KindAssistantText, Branch: "main", Text: "Working on main."},
			{Kind: transcript.KindAssistantText, Branch: "feature/y", Text: "Now on the feature branch."},
			{Kind: transcript.KindAssistantText, Branch: "main", Text: "Back to main."},
		},
	}
	p := &Placement{Session: s, FileName: "f.md", RelPath: "2026-05/f.md"}

	out := RenderSession(p, nil, nil, nil, Options{RootName: "agentic-coding", Location: time.UTC})

	for _, want := range []string{
		"> ⎇ On branch `main`",
		"> ⎇ Switched to branch `feature/y`",
		"- **Branches:** `main`, `feature/y`", // overview lists all touched
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered session missing %q\n---\n%s", want, out)
		}
	}

	// The switch back to main must produce a second "On"/"Switched" marker, so
	// there are at least three markers total for main→feature→main.
	if n := strings.Count(out, "> ⎇ "); n != 3 {
		t.Errorf("expected 3 branch markers (main, feature/y, main), got %d", n)
	}
}

func TestRenderSessionForkNote(t *testing.T) {
	start := time.Date(2026, 5, 2, 14, 30, 0, 0, time.UTC)
	fork := &Placement{
		Session: &transcript.Session{
			ID:                "fork99",
			CustomTitle:       "Diverged work",
			FirstPrompt:       "Now do the new thing.",
			StartedAt:         start,
			EndedAt:           start.Add(20 * time.Minute),
			ForkedFrom:        "parent11",
			InheritedMessages: 142,
			Events: []transcript.Event{
				{Kind: transcript.KindUserPrompt, Text: "Now do the new thing."},
			},
		},
		FileName: "2026-05-02 1430 diverged-work.md",
		RelPath:  "2026-05/2026-05-02 1430 diverged-work.md",
	}
	parent := &Placement{
		Session:  &transcript.Session{ID: "parent11", CustomTitle: "Original work"},
		FileName: "2026-04-30 0900 original-work.md",
		RelPath:  "2026-04/2026-04-30 0900 original-work.md",
	}

	out := RenderSession(fork, nil, nil, parent, Options{RootName: "agentic-coding", Location: time.UTC})

	for _, want := range []string{
		"🍴 Forked from [Original work](<../2026-04/2026-04-30%200900%20original-work.md>)",
		"142 earlier messages",
		"forked-from:parent11", // source-ref in frontmatter
	} {
		if !strings.Contains(out, want) {
			t.Errorf("fork render missing %q\n---\n%s", want, out)
		}
	}

	// A non-forked session emits no fork note.
	plain := &Placement{Session: &transcript.Session{ID: "p", CustomTitle: "Plain", FirstPrompt: "hi"}, RelPath: "2026-05/p.md"}
	if strings.Contains(RenderSession(plain, nil, nil, nil, Options{}), "🍴") {
		t.Error("non-fork session should not render a fork note")
	}
}

func TestRelLink(t *testing.T) {
	tests := []struct {
		from, to, want string
	}{
		{"2026-05/a.md", "2026-05/b.md", "b.md"},            // same month
		{"2026-05/a.md", "2026-04/b.md", "../2026-04/b.md"}, // across months
		{"README.md", "2026-05/b.md", "2026-05/b.md"},       // from root
		{"2026-05/a.md", "README.md", "../README.md"},       // up to root
	}
	for _, tt := range tests {
		if got := relLink(tt.from, tt.to); got != tt.want {
			t.Errorf("relLink(%q, %q) = %q, want %q", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCodeSpanEscapesBackticks(t *testing.T) {
	tests := map[string]string{
		"main":         "`main`",      // common case unchanged
		"feature/y":    "`feature/y`", // slash is fine
		"weird`branch": "``weird`branch``",
		"`leading":     "`` `leading ``",
	}
	for in, want := range tests {
		if got := codeSpan(in); got != want {
			t.Errorf("codeSpan(%q) = %q, want %q", in, got, want)
		}
	}
}
