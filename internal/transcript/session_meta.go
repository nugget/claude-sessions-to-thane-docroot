package transcript

import "strings"

// Title resolves the best human-facing title for the session.
func (s *Session) Title() string {
	if t := strings.TrimSpace(s.CustomTitle); t != "" {
		return t
	}
	if t := strings.TrimSpace(s.AITitle); t != "" {
		return t
	}
	if syn := firstSentence(s.FirstPrompt, 80); syn != "" {
		return syn
	}
	if s.ID != "" {
		return "Session " + s.ShortID()
	}
	return "Untitled session"
}

// HomeBranch is the branch the session spent the most turns on. Empty when no
// git branch was ever recorded.
func (s *Session) HomeBranch() string {
	if len(s.Branches) == 0 {
		return ""
	}
	return s.Branches[0].Branch
}

// TouchedBranches lists every branch observed, most-used first.
func (s *Session) TouchedBranches() []string {
	out := make([]string, 0, len(s.Branches))
	for _, b := range s.Branches {
		out = append(out, b.Branch)
	}
	return out
}

// ShortID returns the leading segment of the session UUID for compact display.
func (s *Session) ShortID() string {
	if i := strings.IndexByte(s.ID, '-'); i > 0 {
		return s.ID[:i]
	}
	if len(s.ID) > 8 {
		return s.ID[:8]
	}
	return s.ID
}

// Synopsis is a one-paragraph précis used as the document's indexed summary:
// the opening human request, condensed to a single line.
func (s *Session) Synopsis() string {
	if syn := firstSentence(s.FirstPrompt, 280); syn != "" {
		return syn
	}
	return s.Title()
}

// firstSentence collapses text to a single line and trims to roughly max runes,
// preferring to cut at a sentence boundary.
func firstSentence(text string, max int) string {
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if text == "" {
		return ""
	}
	if end := sentenceBoundary(text, max); end > 0 {
		return strings.TrimSpace(text[:end])
	}
	if len(text) <= max {
		return text
	}
	cut := text[:max]
	if sp := strings.LastIndexByte(cut, ' '); sp > max/2 {
		cut = cut[:sp]
	}
	return strings.TrimSpace(cut) + "…"
}

// sentenceBoundary returns the index just past the first sentence-ending
// punctuation within max bytes, requiring the punctuation be followed by a
// space or end-of-text so that "cytoscape.js" or "app.go" are not split.
func sentenceBoundary(text string, max int) int {
	for i := 0; i < len(text) && i <= max; i++ {
		switch text[i] {
		case '.', '!', '?':
			if i+1 >= len(text) || text[i+1] == ' ' {
				return i + 1
			}
		}
	}
	return 0
}
