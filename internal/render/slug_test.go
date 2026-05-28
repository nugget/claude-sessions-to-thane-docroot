package render

import "testing"

func TestSlug(t *testing.T) {
	tests := map[string]string{
		"Adopt Trailhead Terminology": "adopt-trailhead-terminology",
		"Fix bug #788 (verifier)":     "fix-bug-788-verifier",
		"  spaced  out  ":             "spaced-out",
		"":                            "untitled",
		"!!!":                         "untitled",
		"café déjà":                   "café-déjà",
	}
	for in, want := range tests {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
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

func TestHrefEncode(t *testing.T) {
	if got := hrefEncode("2026-05-01 my-title.md"); got != "2026-05-01%20my-title.md" {
		t.Errorf("hrefEncode = %q", got)
	}
}
