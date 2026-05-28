package render

import (
	"fmt"
	"strings"
	"time"
)

// fence returns a backtick run long enough to safely wrap content that may
// itself contain backtick fences (at least 3, always longer than the longest
// internal run).
func fence(content string) string {
	longest, cur := 0, 0
	for _, r := range content {
		if r == '`' {
			cur++
			if cur > longest {
				longest = cur
			}
		} else {
			cur = 0
		}
	}
	n := longest + 1
	if n < 3 {
		n = 3
	}
	return strings.Repeat("`", n)
}

// writeCodeBlock writes a fenced code block whose fence cannot collide with
// backticks inside content.
func writeCodeBlock(b *strings.Builder, lang, content string) {
	f := fence(content)
	b.WriteString(f)
	b.WriteString(lang)
	b.WriteByte('\n')
	b.WriteString(strings.TrimRight(content, "\n"))
	b.WriteByte('\n')
	b.WriteString(f)
	b.WriteString("\n")
}

// blockquote prefixes every line with "> " so multi-line content renders as one
// quote. Blank lines keep the quote contiguous.
func blockquote(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, line := range lines {
		if line == "" {
			lines[i] = ">"
		} else {
			lines[i] = "> " + line
		}
	}
	return strings.Join(lines, "\n")
}

func fmtDate(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	return t.In(loc).Format("2006-01-02")
}

func fmtDateTime(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return ""
	}
	return t.In(loc).Format("2006-01-02 15:04 MST")
}

// humanDuration renders a span as a compact "2h 13m" / "47m" / "38s".
func humanDuration(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// truncateRunes trims s to at most max runes, appending an ellipsis marker when
// it cut anything.
func truncateRunes(s string, max int) (string, bool) {
	r := []rune(s)
	if len(r) <= max {
		return s, false
	}
	return strings.TrimSpace(string(r[:max])), true
}
