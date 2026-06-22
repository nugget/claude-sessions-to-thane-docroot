// Package export orchestrates the end-to-end run: locate a Claude project,
// parse its session transcripts, place each session in a branch-grouped
// hierarchy, render the documents, and reconcile them into the target document
// root.
package export

import (
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nugget/session-transcript-exporter/internal/claudeproj"
	"github.com/nugget/session-transcript-exporter/internal/docroot"
	"github.com/nugget/session-transcript-exporter/internal/render"
	"github.com/nugget/session-transcript-exporter/internal/transcript"
)

// Config holds everything one run needs.
type Config struct {
	ProjectArg       string
	TargetDir        string
	RootName         string
	IncludeWorktrees bool
	DryRun           bool
	Prune            bool
	IncludeThinking  bool
	MaxThinkingChars int
	MaxResultLines   int
	MaxResultBytes   int
	Location         *time.Location
	Logger           *slog.Logger
}

// Run performs the export and returns the sync result.
func (c Config) Run() (*docroot.Result, error) {
	logger := c.Logger
	if logger == nil {
		logger = slog.Default()
	}
	loc := c.Location
	if loc == nil {
		loc = time.Local
	}

	proj, err := claudeproj.Locate(c.ProjectArg, c.IncludeWorktrees)
	if err != nil {
		return nil, err
	}
	rootName := strings.TrimSpace(c.RootName)
	if rootName == "" {
		rootName = filepath.Base(c.TargetDir)
	}
	logger.Info("resolved project",
		"display_name", proj.DisplayName,
		"source_dirs", len(proj.SourceDirs),
		"root_name", rootName,
	)

	files, err := proj.SessionFiles()
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no session transcripts found for project %q", c.ProjectArg)
	}

	sessions, err := c.parseSessions(files, logger)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, fmt.Errorf("no sessions with renderable content found")
	}

	opts := render.Options{
		RootName:         rootName,
		IncludeThinking:  c.IncludeThinking,
		MaxThinkingChars: c.MaxThinkingChars,
		Location:         loc,
	}

	ordered, months := placeSessions(sessions, loc)
	plan := docroot.Plan{
		TargetDir:   c.TargetDir,
		OwnedMarker: render.GeneratedBy,
		Prune:       c.Prune,
		Files:       renderAll(proj.DisplayName, ordered, months, opts),
	}

	logger.Info("rendered corpus",
		"sessions", len(sessions),
		"months", len(months),
		"documents", len(plan.Files),
	)
	return docroot.Sync(plan, c.DryRun, logger)
}

func (c Config) parseSessions(files []string, logger *slog.Logger) ([]*transcript.Session, error) {
	popts := transcript.Options{MaxResultLines: c.MaxResultLines, MaxResultBytes: c.MaxResultBytes}
	out := make([]*transcript.Session, 0, len(files))
	for _, f := range files {
		s, err := transcript.ParseFile(f, popts)
		if err != nil {
			logger.Warn("skipping unreadable transcript", "file", f, "error", err)
			continue
		}
		if s.UserTurns == 0 && s.AssistantTurns == 0 {
			logger.Debug("skipping empty session", "file", f)
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

// placeSessions orders every session chronologically — the session, not the
// branch, is the unit of continuity — assigns each a date-time filename, and
// buckets it into its calendar-month directory. It returns the global
// chronological ordering (for prev/next navigation across the whole corpus) and
// the month groups (for the indexes).
func placeSessions(sessions []*transcript.Session, loc *time.Location) ([]*render.Placement, []*render.MonthGroup) {
	ordered := make([]*render.Placement, len(sessions))
	for i, s := range sessions {
		ordered[i] = &render.Placement{Session: s}
	}
	sort.Slice(ordered, func(i, j int) bool {
		a, b := ordered[i].Session, ordered[j].Session
		if !a.StartedAt.Equal(b.StartedAt) {
			return a.StartedAt.Before(b.StartedAt)
		}
		return a.ID < b.ID
	})

	byMonth := make(map[string]*render.MonthGroup)
	var months []*render.MonthGroup
	used := make(map[string]bool)
	for _, p := range ordered {
		month := monthDir(p.Session.StartedAt, loc)
		g, ok := byMonth[month]
		if !ok {
			g = &render.MonthGroup{Month: month, Dir: month}
			byMonth[month] = g
			months = append(months, g) // chronological: ordered is sorted
		}
		p.FileName = sessionFileName(p.Session, loc, used)
		p.RelPath = path.Join(month, p.FileName)
		g.Sessions = append(g.Sessions, p)
	}
	return ordered, months
}

const undatedDir = "undated"

func monthDir(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return undatedDir
	}
	return t.In(loc).Format("2006-01")
}

// sessionFileName builds "YYYY-MM-DD HHMM <slug>.md" in local time, made unique
// across the export via the shared used set. The date in the name means
// same-name collisions can only occur within a single minute, so the id suffix
// is rarely needed.
func sessionFileName(s *transcript.Session, loc *time.Location, used map[string]bool) string {
	var base string
	if s.StartedAt.IsZero() {
		base = "undated " + render.Slug(s.Title())
	} else {
		base = s.StartedAt.In(loc).Format("2006-01-02 1504") + " " + render.Slug(s.Title())
	}
	name := base + ".md"
	if used[name] {
		name = base + " (" + s.ShortID() + ").md"
		for i := 2; used[name]; i++ {
			name = fmt.Sprintf("%s (%s-%d).md", base, s.ShortID(), i)
		}
	}
	used[name] = true
	return name
}

func renderAll(project string, ordered []*render.Placement, months []*render.MonthGroup, opts render.Options) []docroot.File {
	files := make([]docroot.File, 0, len(ordered)+len(months)+1)
	files = append(files, docroot.File{
		RelPath: "README.md",
		Content: render.RenderRootOverview(project, months, opts),
	})
	for _, g := range months {
		files = append(files, docroot.File{
			RelPath: path.Join(g.Dir, "README.md"),
			Content: render.RenderMonthIndex(g, opts),
		})
	}
	byID := make(map[string]*render.Placement, len(ordered))
	for _, p := range ordered {
		byID[p.Session.ID] = p
	}
	for i, p := range ordered {
		var prev, next *render.Placement
		if i > 0 {
			prev = ordered[i-1]
		}
		if i < len(ordered)-1 {
			next = ordered[i+1]
		}
		// parent resolves to the forked-from session when it's also in the corpus.
		parent := byID[p.Session.ForkedFrom]
		files = append(files, docroot.File{
			RelPath: p.RelPath,
			Content: render.RenderSession(p, prev, next, parent, opts),
		})
	}
	return files
}
