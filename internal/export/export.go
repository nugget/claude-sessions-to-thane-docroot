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

	groups := placeSessions(sessions, loc)
	plan := docroot.Plan{
		TargetDir:   c.TargetDir,
		OwnedMarker: render.GeneratedBy,
		Files:       renderAll(proj.DisplayName, groups, opts),
	}

	logger.Info("rendered corpus",
		"sessions", len(sessions),
		"branches", len(groups),
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

// placeSessions groups sessions by their home branch directory, orders them
// chronologically, and assigns collision-free filenames.
func placeSessions(sessions []*transcript.Session, loc *time.Location) []*render.BranchGroup {
	byDir := make(map[string]*render.BranchGroup)
	for _, s := range sessions {
		home := s.HomeBranch()
		dir := render.BranchDir(home)
		g, ok := byDir[dir]
		if !ok {
			g = &render.BranchGroup{Branch: home, Dir: dir}
			byDir[dir] = g
		}
		g.Sessions = append(g.Sessions, &render.Placement{Session: s, Branch: home, BranchDir: dir})
	}

	groups := make([]*render.BranchGroup, 0, len(byDir))
	for _, g := range byDir {
		orderSessions(g.Sessions)
		assignFilenames(g, loc)
		groups = append(groups, g)
	}
	// Overview order: most-recently-active branch first, then by directory.
	sort.Slice(groups, func(i, j int) bool {
		_, li := branchSpan(groups[i])
		_, lj := branchSpan(groups[j])
		if !li.Equal(lj) {
			return li.After(lj)
		}
		return groups[i].Dir < groups[j].Dir
	})
	return groups
}

func orderSessions(ps []*render.Placement) {
	sort.Slice(ps, func(i, j int) bool {
		a, b := ps[i].Session, ps[j].Session
		if !a.StartedAt.Equal(b.StartedAt) {
			return a.StartedAt.Before(b.StartedAt)
		}
		return a.ID < b.ID
	})
}

func assignFilenames(g *render.BranchGroup, loc *time.Location) {
	used := make(map[string]bool)
	for _, p := range g.Sessions {
		date := "undated"
		if !p.Session.StartedAt.IsZero() {
			date = p.Session.StartedAt.In(loc).Format("2006-01-02")
		}
		base := date + " " + render.Slug(p.Session.Title())
		name := base + ".md"
		if used[name] {
			// Deterministic disambiguation: first by short session id,
			// then by an incrementing suffix as a last resort.
			name = base + " (" + p.Session.ShortID() + ").md"
			for i := 2; used[name]; i++ {
				name = fmt.Sprintf("%s (%s-%d).md", base, p.Session.ShortID(), i)
			}
		}
		used[name] = true
		p.FileName = name
		p.RelPath = path.Join(g.Dir, name)
	}
}

func renderAll(project string, groups []*render.BranchGroup, opts render.Options) []docroot.File {
	var files []docroot.File
	files = append(files, docroot.File{
		RelPath: "README.md",
		Content: render.RenderRootOverview(project, groups, opts),
	})
	for _, g := range groups {
		files = append(files, docroot.File{
			RelPath: path.Join(g.Dir, "README.md"),
			Content: render.RenderBranchIndex(g, opts),
		})
		for i, p := range g.Sessions {
			var prev, next *render.Placement
			if i > 0 {
				prev = g.Sessions[i-1]
			}
			if i < len(g.Sessions)-1 {
				next = g.Sessions[i+1]
			}
			files = append(files, docroot.File{
				RelPath: p.RelPath,
				Content: render.RenderSession(p, prev, next, opts),
			})
		}
	}
	return files
}

func branchSpan(g *render.BranchGroup) (first, last time.Time) {
	for _, p := range g.Sessions {
		s := p.Session
		if !s.StartedAt.IsZero() && (first.IsZero() || s.StartedAt.Before(first)) {
			first = s.StartedAt
		}
		if s.EndedAt.After(last) {
			last = s.EndedAt
		}
	}
	return first, last
}
