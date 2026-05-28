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
	n := longestBacktickRun(content) + 1
	if n < 3 {
		n = 3
	}
	return strings.Repeat("`", n)
}

// codeSpan renders s as an inline Markdown code span that survives backticks in
// the content: the delimiter is one longer than the longest internal backtick
// run, with surrounding spaces when the content begins or ends with a backtick.
// Used for untrusted dynamic values (git branch names, paths) that may, however
// rarely, contain backticks.
func codeSpan(s string) string {
	delim := strings.Repeat("`", longestBacktickRun(s)+1)
	pad := ""
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		pad = " "
	}
	return delim + pad + s + pad + delim
}

func longestBacktickRun(s string) int {
	longest, cur := 0, 0
	for _, r := range s {
		if r == '`' {
			cur++
			if cur > longest {
				longest = cur
			}
		} else {
			cur = 0
		}
	}
	return longest
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
