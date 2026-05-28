package render

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RenderBranchIndex builds the README.md for one branch directory: a
// chronological table of contents for the sessions that landed there.
func RenderBranchIndex(g *BranchGroup, opts Options) string {
	loc := opts.loc()
	label := g.Branch
	if label == "" {
		label = "Unbranched sessions"
	}

	first, last := groupSpan(g)
	synopsis := fmt.Sprintf("%s on branch %s.", countNoun(len(g.Sessions), "development session"), backtick(g.Branch))
	if g.Branch == "" {
		synopsis = fmt.Sprintf("%s with no recorded git branch.", countNoun(len(g.Sessions), "development session"))
	}
	if span := spanText(first, last, loc); span != "" {
		synopsis += " " + span
	}

	fm := Frontmatter{
		Title:           "Branch: " + label,
		Summary:         synopsis,
		Tags:            indexTags(g),
		GeneratedAt:     rfc3339(last),
		DocumentKind:    KindBranch,
		RefreshStrategy: "replace",
		SourceRefs:      branchIndexRefs(g, opts.RootName),
		ManagedRoot:     opts.RootName,
		Created:         rfc3339(first),
		Updated:         rfc3339(last),
	}

	var b strings.Builder
	b.WriteString(fm.Render())
	b.WriteByte('\n')
	fmt.Fprintf(&b, "# Branch: %s\n\n", label)
	b.WriteString(synopsis)
	b.WriteString("\n\n## Sessions\n\n")
	for _, p := range g.Sessions {
		writeSessionRow(&b, p, loc)
	}
	return b.String()
}

func writeSessionRow(b *strings.Builder, p *Placement, loc *time.Location) {
	s := p.Session
	date := fmtDate(s.StartedAt, loc)
	fmt.Fprintf(b, "- **%s** — [%s](<%s>)", date, oneLine(s.Title()), hrefEncode(p.FileName))
	if len(s.PRs) > 0 {
		nums := make([]string, 0, len(s.PRs))
		for _, pr := range s.PRs {
			if pr.Number > 0 {
				nums = append(nums, "#"+strconv.Itoa(pr.Number))
			}
		}
		if len(nums) > 0 {
			fmt.Fprintf(b, " · PR %s", strings.Join(nums, ", "))
		}
	}
	b.WriteString("\n")
	if syn := s.Synopsis(); syn != "" && !strings.EqualFold(syn, s.Title()) {
		fmt.Fprintf(b, "  - %s\n", oneLine(syn))
	}
}

// RenderRootOverview builds the document-root README: the entry point that
// frames the whole corpus and links every branch index.
func RenderRootOverview(project string, groups []*BranchGroup, opts Options) string {
	loc := opts.loc()
	total, first, last, prCount := corpusStats(groups)

	synopsis := fmt.Sprintf("Development session transcripts for %s, exported as primary source material for tracing the project's development narrative. %s across %s.",
		project, countNoun(total, "session"), countNoun(len(groups), "branch"))
	if span := spanText(first, last, loc); span != "" {
		synopsis += " " + span
	}

	fm := Frontmatter{
		Title:           project + " development transcripts",
		Summary:         synopsis,
		Tags:            []string{"transcript", "index", "overview"},
		GeneratedAt:     rfc3339(last),
		DocumentKind:    KindOverview,
		RefreshStrategy: "replace",
		SourceRefs:      []string{"project:" + project},
		ManagedRoot:     opts.RootName,
		Created:         rfc3339(first),
		Updated:         rfc3339(last),
	}

	var b strings.Builder
	b.WriteString(fm.Render())
	b.WriteByte('\n')
	fmt.Fprintf(&b, "# %s development transcripts\n\n", project)
	b.WriteString(synopsis)
	b.WriteString("\n\n")

	if prCount > 0 {
		fmt.Fprintf(&b, "%s linked across the corpus.\n\n", countNoun(prCount, "pull request"))
	}

	b.WriteString("## Branches\n\n")
	for _, g := range groups {
		writeBranchRow(&b, g, loc)
	}
	return b.String()
}

func writeBranchRow(b *strings.Builder, g *BranchGroup, loc *time.Location) {
	label := g.Branch
	if label == "" {
		label = "Unbranched"
	}
	_, last := groupSpan(g)
	indexPath := g.Dir + "/README.md"
	fmt.Fprintf(b, "- [%s](<%s>) — %s", label, hrefEncode(indexPath), countNoun(len(g.Sessions), "session"))
	if !last.IsZero() {
		fmt.Fprintf(b, ", latest %s", fmtDate(last, loc))
	}
	b.WriteString("\n")
}

// --- helpers ---

func indexTags(g *BranchGroup) []string {
	tags := []string{"transcript", "index"}
	if g.Branch != "" {
		tags = append(tags, Slug(g.Branch))
	}
	return tags
}

func branchIndexRefs(g *BranchGroup, root string) []string {
	if g.Branch == "" {
		return nil
	}
	return []string{"branch:" + g.Branch}
}

func groupSpan(g *BranchGroup) (first, last time.Time) {
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

func corpusStats(groups []*BranchGroup) (sessions int, first, last time.Time, prs int) {
	for _, g := range groups {
		sessions += len(g.Sessions)
		gf, gl := groupSpan(g)
		if !gf.IsZero() && (first.IsZero() || gf.Before(first)) {
			first = gf
		}
		if gl.After(last) {
			last = gl
		}
		for _, p := range g.Sessions {
			prs += len(p.Session.PRs)
		}
	}
	return sessions, first, last, prs
}

func spanText(first, last time.Time, loc *time.Location) string {
	if first.IsZero() {
		return ""
	}
	fd := fmtDate(first, loc)
	if last.IsZero() || fmtDate(last, loc) == fd {
		return fmt.Sprintf("Recorded %s.", fd)
	}
	return fmt.Sprintf("Recorded from %s to %s.", fd, fmtDate(last, loc))
}

func countNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + pluralize(noun)
}

func pluralize(noun string) string {
	switch {
	case strings.HasSuffix(noun, "ch"), strings.HasSuffix(noun, "sh"),
		strings.HasSuffix(noun, "s"), strings.HasSuffix(noun, "x"):
		return noun + "es"
	case strings.HasSuffix(noun, "y") && !endsInVowelY(noun):
		return noun[:len(noun)-1] + "ies"
	default:
		return noun + "s"
	}
}

func endsInVowelY(noun string) bool {
	if len(noun) < 2 {
		return false
	}
	switch noun[len(noun)-2] {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

func backtick(s string) string {
	if s == "" {
		return ""
	}
	return "`" + s + "`"
}
