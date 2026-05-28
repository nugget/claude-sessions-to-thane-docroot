package render

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RenderMonthIndex builds the README.md for one month directory: a chronological
// table of contents for the sessions that started that month.
func RenderMonthIndex(g *MonthGroup, opts Options) string {
	loc := opts.loc()
	label := monthLabel(g.Month)

	first, last := groupSpan(g)
	synopsis := fmt.Sprintf("%s in %s.", countNoun(len(g.Sessions), "development session"), label)
	if span := spanText(first, last, loc); span != "" {
		synopsis += " " + span
	}

	fm := Frontmatter{
		Title:           label + " — sessions",
		Summary:         synopsis,
		Tags:            []string{"transcript", "index", g.Month},
		GeneratedAt:     rfc3339(last),
		DocumentKind:    KindMonth,
		RefreshStrategy: "replace",
		ManagedRoot:     opts.RootName,
		Created:         rfc3339(first),
		Updated:         rfc3339(last),
	}

	var b strings.Builder
	b.WriteString(fm.Render())
	b.WriteByte('\n')
	fmt.Fprintf(&b, "# %s\n\n", label)
	b.WriteString(synopsis)
	b.WriteString("\n\n## Sessions\n\n")
	for _, p := range g.Sessions {
		writeSessionRow(&b, p, loc)
	}
	return b.String()
}

func writeSessionRow(b *strings.Builder, p *Placement, loc *time.Location) {
	s := p.Session
	fmt.Fprintf(b, "- **%s** — [%s](<%s>)", fmtRowStamp(s.StartedAt, loc), oneLine(s.Title()), hrefEncode(p.FileName))
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
	if brs := s.TouchedBranches(); len(brs) > 0 {
		fmt.Fprintf(b, " · %s", branchHint(brs))
	}
	b.WriteString("\n")
	if syn := s.Synopsis(); syn != "" && !strings.EqualFold(syn, s.Title()) {
		fmt.Fprintf(b, "  - %s\n", oneLine(syn))
	}
}

// branchHint renders a compact branch summary for an index row.
func branchHint(branches []string) string {
	if len(branches) == 1 {
		return "`" + branches[0] + "`"
	}
	return fmt.Sprintf("`%s` +%d", branches[0], len(branches)-1)
}

// RenderRootOverview builds the document-root README for this project: the entry
// point that frames the corpus and links each month index.
func RenderRootOverview(project string, groups []*MonthGroup, opts Options) string {
	loc := opts.loc()
	total, first, last, prCount := corpusStats(groups)

	synopsis := fmt.Sprintf("Development session transcripts for %s, exported as primary source material for tracing the project's development narrative. %s across %s.",
		project, countNoun(total, "session"), countNoun(len(groups), "month"))
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

	b.WriteString("## Months\n\n")
	for _, g := range groups {
		writeMonthRow(&b, g, loc)
	}
	return b.String()
}

func writeMonthRow(b *strings.Builder, g *MonthGroup, _ *time.Location) {
	indexPath := g.Dir + "/README.md"
	fmt.Fprintf(b, "- [%s](<%s>) — %s\n", monthLabel(g.Month), hrefEncode(indexPath), countNoun(len(g.Sessions), "session"))
}

// --- helpers ---

// monthLabel turns a "YYYY-MM" directory name into a human label; anything that
// doesn't parse (i.e. the undated bucket) renders as "Undated".
func monthLabel(month string) string {
	if t, err := time.Parse("2006-01", month); err == nil {
		return t.Format("January 2006")
	}
	return "Undated"
}

func fmtRowStamp(t time.Time, loc *time.Location) string {
	if t.IsZero() {
		return "undated"
	}
	return t.In(loc).Format("2006-01-02 15:04")
}

func groupSpan(g *MonthGroup) (first, last time.Time) {
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

func corpusStats(groups []*MonthGroup) (sessions int, first, last time.Time, prs int) {
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
