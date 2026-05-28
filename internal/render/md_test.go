package render

import (
	"strings"
	"testing"
	"time"
)

func TestPluralizeAndCountNoun(t *testing.T) {
	tests := []struct {
		n    int
		noun string
		want string
	}{
		{1, "session", "1 session"},
		{3, "session", "3 sessions"},
		{20, "branch", "20 branches"},
		{2, "prompt", "2 prompts"},
		{1, "prompt", "1 prompt"},
		{5, "pull request", "5 pull requests"},
		{2, "category", "2 categories"},
		{2, "day", "2 days"},
	}
	for _, tt := range tests {
		if got := countNoun(tt.n, tt.noun); got != tt.want {
			t.Errorf("countNoun(%d, %q) = %q, want %q", tt.n, tt.noun, got, tt.want)
		}
	}
}

func TestFenceLongerThanInternalBackticks(t *testing.T) {
	content := "code with ```` four-tick fence inside"
	f := fence(content)
	if len(f) < 5 {
		t.Errorf("fence %q must be longer than the 4-backtick run inside", f)
	}
	if strings.Contains(content, f) {
		t.Errorf("fence %q collides with content", f)
	}
}

func TestBlockquote(t *testing.T) {
	got := blockquote("line one\n\nline two")
	want := "> line one\n>\n> line two"
	if got != want {
		t.Errorf("blockquote = %q, want %q", got, want)
	}
}

func TestHumanDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{2*time.Hour + 13*time.Minute, "2h 13m"},
		{47 * time.Minute, "47m"},
		{30 * time.Second, "30s"},
		{0, ""},
	}
	for _, tt := range tests {
		if got := humanDuration(tt.d); got != tt.want {
			t.Errorf("humanDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}
