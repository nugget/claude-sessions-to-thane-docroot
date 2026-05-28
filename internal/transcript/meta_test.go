package transcript

import "testing"

func TestFirstSentenceDoesNotSplitOnDottedTokens(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"Evaluate cytoscape.js as a render engine for the graph", 280, "Evaluate cytoscape.js as a render engine for the graph"},
		{"Fix the bug. Then ship it.", 280, "Fix the bug."},
		{"Look at app.go and main.go please.", 280, "Look at app.go and main.go please."},
		{"", 280, ""},
	}
	for _, tt := range tests {
		if got := firstSentence(tt.in, tt.max); got != tt.want {
			t.Errorf("firstSentence(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestFirstSentenceTruncates(t *testing.T) {
	long := "word " // 5 chars
	in := ""
	for i := 0; i < 40; i++ {
		in += long
	}
	got := firstSentence(in, 50)
	if len(got) > 53 { // 50 + ellipsis rune
		t.Errorf("expected truncation near 50, got len %d: %q", len(got), got)
	}
	if got[len(got)-3:] != "…" {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
}

func TestTitlePrecedence(t *testing.T) {
	tests := []struct {
		name string
		s    Session
		want string
	}{
		{"custom wins", Session{CustomTitle: "Custom", AITitle: "AI", FirstPrompt: "prompt"}, "Custom"},
		{"ai when no custom", Session{AITitle: "AI title", FirstPrompt: "prompt here"}, "AI title"},
		{"prompt when no titles", Session{FirstPrompt: "Do a thing.", ID: "abcd1234-xx"}, "Do a thing."},
		{"id fallback", Session{ID: "abcd1234-xxxx-yyyy"}, "Session abcd1234"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.Title(); got != tt.want {
				t.Errorf("Title() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShortID(t *testing.T) {
	s := Session{ID: "6daadaf0-bdbe-47fb"}
	if got := s.ShortID(); got != "6daadaf0" {
		t.Errorf("ShortID() = %q", got)
	}
}
