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

	out := RenderSession(p, nil, nil, Options{RootName: "agentic-coding", Location: time.UTC})

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
