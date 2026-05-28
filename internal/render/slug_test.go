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

func TestBranchDir(t *testing.T) {
	tests := map[string]string{
		"main":                  "main",
		"claude/archive-layout": "claude/archive-layout",
		"fix/issue-788":         "fix/issue-788",
		"":                      "_unbranched",
		"weird:branch*name":     "weird-branch-name",
		"feature/sub/deep":      "feature/sub/deep",
	}
	for in, want := range tests {
		if got := BranchDir(in); got != want {
			t.Errorf("BranchDir(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHrefEncode(t *testing.T) {
	if got := hrefEncode("2026-05-01 my-title.md"); got != "2026-05-01%20my-title.md" {
		t.Errorf("hrefEncode = %q", got)
	}
}
