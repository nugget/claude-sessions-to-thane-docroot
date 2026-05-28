package render

import (
	"strings"
)

// Frontmatter is the document header. It renders to the strict subset thane's
// parser understands: flat keys ([A-Za-z0-9_-]+), scalar values, and block
// lists. No nested maps, no inline arrays, no multi-line scalars.
type Frontmatter struct {
	Title           string
	Summary         string // rendered as "description"
	Tags            []string
	GeneratedAt     string // RFC3339
	DocumentKind    string
	RefreshStrategy string
	SourceRefs      []string
	ManagedRoot     string
	Created         string // RFC3339
	Updated         string // RFC3339
}

// Render emits the YAML-fenced frontmatter block, fields in thane's documented
// house order. generated_by is always stamped so the sync layer can recognize
// the file as ours.
func (f Frontmatter) Render() string {
	var b strings.Builder
	b.WriteString("---\n")
	writeScalar(&b, "title", f.Title)
	writeScalar(&b, "description", f.Summary)
	writeList(&b, "tags", f.Tags)
	writeScalar(&b, "generated_by", GeneratedBy)
	writeScalar(&b, "generated_at", f.GeneratedAt)
	writeScalar(&b, "document_kind", f.DocumentKind)
	writeScalar(&b, "refresh_strategy", f.RefreshStrategy)
	writeList(&b, "source_refs", f.SourceRefs)
	writeScalar(&b, "managed_root", f.ManagedRoot)
	writeScalar(&b, "created", f.Created)
	writeScalar(&b, "updated", f.Updated)
	b.WriteString("---\n")
	return b.String()
}

func writeScalar(b *strings.Builder, key, value string) {
	value = sanitizeValue(value)
	if value == "" {
		return
	}
	b.WriteString(key)
	b.WriteString(": \"")
	b.WriteString(value)
	b.WriteString("\"\n")
}

func writeList(b *strings.Builder, key string, values []string) {
	cleaned := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, v := range values {
		v = sanitizeValue(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		cleaned = append(cleaned, v)
	}
	if len(cleaned) == 0 {
		return
	}
	b.WriteString(key)
	b.WriteString(":\n")
	for _, v := range cleaned {
		b.WriteString("  - \"")
		b.WriteString(v)
		b.WriteString("\"\n")
	}
}

// sanitizeValue makes a string safe to emit as a single double-quoted scalar
// that thane's parser will read back intact. The parser strips only outer
// quotes (no unescaping), so interior double-quotes are downgraded to single
// quotes, and all whitespace is collapsed to single spaces.
func sanitizeValue(v string) string {
	v = strings.ReplaceAll(v, "\"", "'")
	v = strings.Join(strings.Fields(v), " ")
	return strings.TrimSpace(v)
}
