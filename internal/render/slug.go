package render

import (
	"strings"
	"unicode"
)

// Slug converts arbitrary text into a filesystem- and URL-friendly token:
// lowercase, alphanumerics kept, everything else collapsed to single hyphens.
func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "untitled"
	}
	return clampSlug(out, 70)
}

// clampSlug trims to at most max bytes without splitting a word mid-hyphen.
func clampSlug(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if i := strings.LastIndexByte(cut, '-'); i > max/2 {
		cut = cut[:i]
	}
	return strings.Trim(cut, "-")
}

// hrefEncode makes a relative file path safe to use as a markdown link target,
// percent-encoding spaces so links resolve in standard renderers.
func hrefEncode(path string) string {
	return strings.ReplaceAll(path, " ", "%20")
}
