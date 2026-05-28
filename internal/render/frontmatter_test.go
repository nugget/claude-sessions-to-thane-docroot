package render

import (
	"strings"
	"testing"
)

func TestFrontmatterRender(t *testing.T) {
	fm := Frontmatter{
		Title:           `Session with "quotes" and   spaces`,
		Summary:         "A summary.",
		Tags:            []string{"session", "transcript", "session"}, // dup dropped
		GeneratedAt:     "2026-05-01T10:00:00Z",
		DocumentKind:    KindSession,
		RefreshStrategy: "replace",
		SourceRefs:      []string{"conversation:abc", "branch:main"},
		ManagedRoot:     "transcripts",
		Created:         "2026-05-01T09:00:00Z",
		Updated:         "2026-05-01T10:00:00Z",
	}
	out := fm.Render()

	mustContain := []string{
		"---\n",
		`title: "Session with 'quotes' and spaces"`, // quotes downgraded, whitespace collapsed
		`description: "A summary."`,
		"tags:\n  - \"session\"\n  - \"transcript\"\n", // deduped, block list
		`generated_by: "session-transcript-exporter"`,  // always stamped
		`document_kind: "session_transcript"`,
		`refresh_strategy: "replace"`,
		"source_refs:\n  - \"conversation:abc\"\n  - \"branch:main\"\n",
		`managed_root: "transcripts"`,
	}
	for _, want := range mustContain {
		if !strings.Contains(out, want) {
			t.Errorf("frontmatter missing %q in:\n%s", want, out)
		}
	}
	if strings.Count(out, "---\n") != 2 {
		t.Errorf("expected exactly two fence lines, got:\n%s", out)
	}
}

func TestFrontmatterSkipsEmptyFields(t *testing.T) {
	fm := Frontmatter{Title: "Only title"}
	out := fm.Render()
	if strings.Contains(out, "description:") || strings.Contains(out, "tags:") || strings.Contains(out, "source_refs:") {
		t.Errorf("empty fields should be omitted:\n%s", out)
	}
	if !strings.Contains(out, `generated_by: "session-transcript-exporter"`) {
		t.Errorf("generated_by must always be present:\n%s", out)
	}
}

func TestSanitizeValue(t *testing.T) {
	tests := map[string]string{
		"plain":              "plain",
		"has \"quote\" mark": "has 'quote' mark",
		"multi\nline\ttext":  "multi line text",
		"  trim me  ":        "trim me",
	}
	for in, want := range tests {
		if got := sanitizeValue(in); got != want {
			t.Errorf("sanitizeValue(%q) = %q, want %q", in, got, want)
		}
	}
}
