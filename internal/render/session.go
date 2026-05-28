package render

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/nugget/session-transcript-exporter/internal/transcript"
)

// RenderSession produces the full markdown document for one session. prev and
// next are the chronologically adjacent sessions in the same branch (either may
// be nil) and are linked for narrative navigation.
func RenderSession(p *Placement, prev, next *Placement, opts Options) string {
	s := p.Session

	var b strings.Builder
	b.WriteString(sessionFrontmatter(p, opts).Render())
	b.WriteByte('\n')

	fmt.Fprintf(&b, "# %s\n\n", oneLine(s.Title()))
	// Lead paragraph — this is what thane indexes as the document summary.
	b.WriteString(s.Synopsis())
	b.WriteString("\n\n")

	writeOverview(&b, s, opts)

	b.WriteString("## Conversation\n\n")
	writeConversation(&b, s, opts)

	writeNavigation(&b, p, prev, next)
	return b.String()
}

func sessionFrontmatter(p *Placement, opts Options) Frontmatter {
	s := p.Session
	return Frontmatter{
		Title:           s.Title(),
		Summary:         s.Synopsis(),
		Tags:            sessionTags(s),
		GeneratedAt:     rfc3339(s.EndedAt),
		DocumentKind:    KindSession,
		RefreshStrategy: "replace",
		SourceRefs:      sessionSourceRefs(s, opts.RootName),
		ManagedRoot:     opts.RootName,
		Created:         rfc3339(s.StartedAt),
		Updated:         rfc3339(s.EndedAt),
	}
}

func sessionTags(s *transcript.Session) []string {
	tags := []string{"session", "transcript"}
	if !s.StartedAt.IsZero() {
		tags = append(tags, strconv.Itoa(s.StartedAt.UTC().Year()))
	}
	if len(s.PRs) > 0 {
		tags = append(tags, "has-pr")
	}
	return tags
}

func sessionSourceRefs(s *transcript.Session, root string) []string {
	refs := []string{"conversation:" + s.ID}
	for _, br := range s.TouchedBranches() {
		refs = append(refs, "branch:"+br)
	}
	for _, pr := range s.PRs {
		if pr.Number > 0 {
			refs = append(refs, "pr:"+strconv.Itoa(pr.Number))
		}
	}
	for _, cwd := range s.CWDs {
		refs = append(refs, "cwd:"+cwd)
	}
	return refs
}

func writeOverview(b *strings.Builder, s *transcript.Session, opts Options) {
	loc := opts.loc()
	b.WriteString("## Overview\n\n")

	if when := timeRange(s, loc); when != "" {
		fmt.Fprintf(b, "- **When:** %s\n", when)
	}
	if len(s.Branches) > 0 {
		names := make([]string, 0, len(s.Branches))
		for _, br := range s.Branches {
			names = append(names, codeSpan(br.Branch))
		}
		label := "Branch"
		if len(names) > 1 {
			label = "Branches"
		}
		fmt.Fprintf(b, "- **%s:** %s\n", label, strings.Join(names, ", "))
	}
	if len(s.CWDs) > 0 {
		dirs := make([]string, len(s.CWDs))
		for i, d := range s.CWDs {
			dirs[i] = codeSpan(d)
		}
		fmt.Fprintf(b, "- **Working directory:** %s\n", strings.Join(dirs, ", "))
	}
	fmt.Fprintf(b, "- **Activity:** %s · %s · %s\n",
		countNoun(s.UserTurns, "prompt"), countNoun(s.AssistantTurns, "assistant turn"), countNoun(s.ToolCalls, "tool call"))
	if tools := topTools(s.ToolUse, 8); tools != "" {
		fmt.Fprintf(b, "- **Tools:** %s\n", tools)
	}
	if len(s.PRs) > 0 {
		fmt.Fprintf(b, "- **Pull requests:** %s\n", prLinks(s.PRs))
	}
	if s.Version != "" {
		entry := s.Version
		if s.Entrypoint != "" {
			entry += " · " + s.Entrypoint
		}
		fmt.Fprintf(b, "- **Claude Code:** %s\n", entry)
	}
	fmt.Fprintf(b, "- **Session:** `%s`\n", s.ID)
	b.WriteString("\n")
}

func writeConversation(b *strings.Builder, s *transcript.Session, opts Options) {
	promptNo := 0
	currentBranch := ""
	for i := range s.Events {
		ev := &s.Events[i]
		// A session interleaves work across branches; surface each switch so
		// the movement is visible even though the document isn't filed by branch.
		if ev.Branch != "" && ev.Branch != currentBranch {
			writeBranchMarker(b, ev.Branch, currentBranch == "")
			currentBranch = ev.Branch
		}
		switch ev.Kind {
		case transcript.KindUserPrompt:
			promptNo++
			fmt.Fprintf(b, "### %d. %s\n\n", promptNo, oneLine(firstSentenceLabel(ev.Text)))
			b.WriteString(blockquote(ev.Text))
			b.WriteString("\n\n")
		case transcript.KindAssistantText:
			b.WriteString(ev.Text)
			b.WriteString("\n\n")
		case transcript.KindThinking:
			if opts.IncludeThinking {
				writeThinking(b, ev.Text, opts.maxThinking())
			}
		case transcript.KindToolCall:
			writeToolCall(b, ev.Tool)
		case transcript.KindPRLink:
			writePRLink(b, ev.PR)
		}
	}
}

func writeBranchMarker(b *strings.Builder, branch string, first bool) {
	verb := "Switched to"
	if first {
		verb = "On"
	}
	fmt.Fprintf(b, "> ⎇ %s branch %s\n\n", verb, codeSpan(branch))
}

func writeThinking(b *strings.Builder, text string, max int) {
	text, trimmed := truncateRunes(text, max)
	b.WriteString("<details><summary>💭 Thinking</summary>\n\n")
	b.WriteString(text)
	if trimmed {
		b.WriteString("\n\n_(thinking truncated)_")
	}
	b.WriteString("\n\n</details>\n\n")
}

func writeToolCall(b *strings.Builder, tc *transcript.ToolCall) {
	if tc == nil {
		return
	}
	head := fmt.Sprintf("🔧 **%s**", tc.Name)
	if tc.Summary != "" {
		head += " — " + oneLine(tc.Summary)
	}
	if tc.IsError {
		head += " ⚠️"
	}
	b.WriteString(head)
	b.WriteString("\n\n")

	if tc.Detail != "" {
		detail, _ := truncateRunes(tc.Detail, 800)
		writeCodeBlock(b, "bash", detail)
		b.WriteString("\n")
	}
	if tc.HasResult && tc.Result != "" {
		writeToolResult(b, tc)
	}
}

func writeToolResult(b *strings.Builder, tc *transcript.ToolCall) {
	result := tc.Result
	suffix := ""
	if tc.ResultTrimmed {
		suffix = "\n\n_(output truncated)_"
	}
	lines := strings.Count(result, "\n") + 1
	switch {
	case tc.IsError:
		b.WriteString("Error:\n\n")
		writeCodeBlock(b, "", result)
		b.WriteString(suffix + "\n")
	case lines <= 3 && len(result) <= 200:
		b.WriteString(blockquote(result))
		b.WriteString(suffix + "\n\n")
	default:
		fmt.Fprintf(b, "<details><summary>output (%d lines)</summary>\n\n", lines)
		writeCodeBlock(b, "", result)
		b.WriteString(suffix)
		b.WriteString("\n</details>\n\n")
	}
}

func writePRLink(b *strings.Builder, pr *transcript.PullRequest) {
	if pr == nil {
		return
	}
	fmt.Fprintf(b, "> 📦 Opened pull request %s\n\n", prLink(*pr))
}

func writeNavigation(b *strings.Builder, p, prev, next *Placement) {
	if prev == nil && next == nil {
		return
	}
	b.WriteString("---\n\n")
	var parts []string
	if prev != nil {
		parts = append(parts, fmt.Sprintf("← Previous: [%s](<%s>)", oneLine(prev.Session.Title()), hrefEncode(relLink(p.RelPath, prev.RelPath))))
	}
	if next != nil {
		parts = append(parts, fmt.Sprintf("Next: [%s](<%s>) →", oneLine(next.Session.Title()), hrefEncode(relLink(p.RelPath, next.RelPath))))
	}
	b.WriteString(strings.Join(parts, " · "))
	b.WriteString("\n")
}

// relLink computes a relative markdown link target from the directory of
// fromRel to the file toRel (both root-relative, forward-slash). Chronological
// neighbors can live in different month directories, so a bare filename won't do.
func relLink(fromRel, toRel string) string {
	var from []string
	if dir := path.Dir(fromRel); dir != "." {
		from = strings.Split(dir, "/")
	}
	to := strings.Split(toRel, "/")
	i := 0
	for i < len(from) && i < len(to)-1 && from[i] == to[i] {
		i++
	}
	var parts []string
	for j := i; j < len(from); j++ {
		parts = append(parts, "..")
	}
	parts = append(parts, to[i:]...)
	return path.Join(parts...)
}

// --- small helpers ---

func timeRange(s *transcript.Session, loc *time.Location) string {
	if s.StartedAt.IsZero() {
		return ""
	}
	start := fmtDateTime(s.StartedAt, loc)
	if s.EndedAt.IsZero() || s.EndedAt.Equal(s.StartedAt) {
		return start
	}
	dur := humanDuration(s.EndedAt.Sub(s.StartedAt))
	end := s.EndedAt.In(loc).Format("15:04 MST")
	if fmtDate(s.StartedAt, loc) != fmtDate(s.EndedAt, loc) {
		end = fmtDateTime(s.EndedAt, loc)
	}
	if dur != "" {
		return fmt.Sprintf("%s → %s (%s)", start, end, dur)
	}
	return fmt.Sprintf("%s → %s", start, end)
}

func topTools(stats []transcript.ToolStat, n int) string {
	if len(stats) == 0 {
		return ""
	}
	if len(stats) > n {
		stats = stats[:n]
	}
	parts := make([]string, len(stats))
	for i, st := range stats {
		parts[i] = fmt.Sprintf("%s ×%d", st.Name, st.Count)
	}
	return strings.Join(parts, ", ")
}

func prLinks(prs []transcript.PullRequest) string {
	parts := make([]string, 0, len(prs))
	for _, pr := range prs {
		parts = append(parts, prLink(pr))
	}
	return strings.Join(parts, ", ")
}

func prLink(pr transcript.PullRequest) string {
	label := pr.Repository
	if pr.Number > 0 {
		if label != "" {
			label += "#" + strconv.Itoa(pr.Number)
		} else {
			label = "#" + strconv.Itoa(pr.Number)
		}
	}
	if label == "" {
		label = pr.URL
	}
	if pr.URL != "" {
		return fmt.Sprintf("[%s](%s)", label, pr.URL)
	}
	return label
}

func rfc3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// oneLine collapses whitespace and strips a leading markdown heading marker so
// the text is safe to drop into a heading or inline context.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimLeft(s, "#")
}

func firstSentenceLabel(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return "User"
	}
	for i := 0; i < len(text) && i <= 80; i++ {
		if (text[i] == '.' || text[i] == '!' || text[i] == '?') &&
			(i+1 >= len(text) || text[i+1] == ' ') {
			return text[:i]
		}
	}
	if cut, trimmed := truncateRunes(text, 80); trimmed {
		return cut + "…"
	}
	return text
}
