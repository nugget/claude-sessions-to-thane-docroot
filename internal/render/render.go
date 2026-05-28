// Package render turns parsed sessions into the markdown documents that make up
// a thane-ai-agent "document root". Output is tailored to that specific reader:
// frontmatter uses only the flat key/scalar/list subset thane's line-based
// parser understands, the indexed summary is the document's lead paragraph, and
// documents carry thane's generated-artifact provenance fields.
//
// All output is a deterministic function of the parsed transcripts so that a
// re-run produces byte-identical files and the sync becomes a clean no-op.
package render

import (
	"time"

	"github.com/nugget/session-transcript-exporter/internal/transcript"
)

// GeneratedBy is the frontmatter marker stamped on every document this tool
// writes. The sync layer uses it to recognize files it owns and may delete.
const GeneratedBy = "session-transcript-exporter"

// Document kinds surfaced in frontmatter for the reader.
const (
	KindSession  = "session_transcript"
	KindMonth    = "transcript_month_index"
	KindOverview = "transcript_root_overview"
)

// Options controls rendering decisions that aren't intrinsic to the data.
type Options struct {
	RootName         string         // thane root name (frontmatter managed_root + refs)
	IncludeThinking  bool           // render assistant thinking blocks
	MaxThinkingChars int            // cap per thinking block (0 → default)
	Location         *time.Location // timezone for human-facing dates (nil → UTC)
}

const defaultMaxThinkingChars = 1600

func (o Options) loc() *time.Location {
	if o.Location != nil {
		return o.Location
	}
	return time.UTC
}

func (o Options) maxThinking() int {
	if o.MaxThinkingChars <= 0 {
		return defaultMaxThinkingChars
	}
	return o.MaxThinkingChars
}

// Placement is a session assigned to its destination within the document root.
type Placement struct {
	Session  *transcript.Session
	FileName string // e.g. "2026-05-26 1431 adopt-trailhead.md"
	RelPath  string // MonthGroup.Dir + "/" + FileName
}

// MonthGroup is the set of sessions that started in one calendar month. The
// session — not the branch — is the unit of continuity, so sessions are
// organized chronologically; branches are recorded as metadata within each
// document instead of fragmenting it across folders.
type MonthGroup struct {
	Month    string // "YYYY-MM"
	Dir      string // relative directory (== Month, or "undated")
	Sessions []*Placement
}
