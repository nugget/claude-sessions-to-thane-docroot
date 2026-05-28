package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Options tunes how aggressively bulky content is trimmed while parsing.
type Options struct {
	MaxResultLines int // cap tool-result text at this many lines (0 → default)
	MaxResultBytes int // cap tool-result text at this many bytes (0 → default)
}

const (
	defaultMaxResultLines = 30
	defaultMaxResultBytes = 4000
)

func (o Options) withDefaults() Options {
	if o.MaxResultLines <= 0 {
		o.MaxResultLines = defaultMaxResultLines
	}
	if o.MaxResultBytes <= 0 {
		o.MaxResultBytes = defaultMaxResultBytes
	}
	return o
}

type rawEntry struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	IsMeta    bool   `json:"isMeta"`

	CWD        string `json:"cwd"`
	GitBranch  string `json:"gitBranch"`
	Version    string `json:"version"`
	Entrypoint string `json:"entrypoint"`

	Message *rawMessage `json:"message"`

	AITitle     string `json:"aiTitle"`
	CustomTitle string `json:"customTitle"`

	PRNumber     int    `json:"prNumber"`
	PRURL        string `json:"prUrl"`
	PRRepository string `json:"prRepository"`
}

type rawMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type rawBlock struct {
	Type     string         `json:"type"`
	Text     string         `json:"text"`
	Thinking string         `json:"thinking"`
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Input    map[string]any `json:"input"`

	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

// ParseFile reads and distills one session transcript .jsonl file.
func ParseFile(path string, opts Options) (*Session, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parse(f, path, opts)
}

func parse(r io.Reader, path string, opts Options) (*Session, error) {
	opts = opts.withDefaults()
	canonicalID := sessionIDFromPath(path)
	p := &parser{
		opts:        opts,
		sess:        &Session{SourceFile: path, ID: canonicalID},
		canonicalID: canonicalID,
		toolByID:    make(map[string]*ToolCall),
		pending:     make(map[string]pendingResult),
		branchHits:  make(map[string]int),
		branchLast:  make(map[string]int),
		toolHits:    make(map[string]int),
		seenCWD:     make(map[string]bool),
		seenPR:      make(map[string]bool),
	}

	br := bufio.NewReader(r)
	for {
		line, readErr := br.ReadBytes('\n')
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			var e rawEntry
			if err := json.Unmarshal(trimmed, &e); err == nil {
				p.handle(&e)
			}
			// Malformed lines are skipped: a corrupt entry should not
			// abort an otherwise readable transcript.
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return nil, fmt.Errorf("read %s: %w", path, readErr)
		}
	}

	p.finalize()
	return p.sess, nil
}

type parser struct {
	opts Options
	sess *Session

	canonicalID   string // the session's own id (filename stem)
	ownStarted    bool   // seen the first own message (the fork divergence point)
	parentID      string // immediate parent id while in the inherited prefix
	inheritedMsgs int    // copied prefix messages skipped

	toolByID   map[string]*ToolCall
	pending    map[string]pendingResult // results seen before their tool_use
	branchHits map[string]int
	branchLast map[string]int // branch → sequence of its last occurrence
	toolHits   map[string]int
	seenCWD    map[string]bool
	seenPR     map[string]bool
	seq        int
}

// pendingResult holds a tool_result whose tool_use has not been seen yet.
// Claude Code can log a result before its call when tools run in parallel.
type pendingResult struct {
	text    string
	isError bool
	trimmed bool
}

func (p *parser) handle(e *rawEntry) {
	// Without a reliable filename id, fall back to the first record's id; the
	// fork-prefix detection then no-ops (every record looks "own").
	if p.canonicalID == "" && e.SessionID != "" {
		p.canonicalID = e.SessionID
		p.sess.ID = e.SessionID
	}

	isMessage := e.Type == "user" || e.Type == "assistant"
	if !p.isOwn(e) {
		// Copied conversation prefix from a parent session: a fork keeps the
		// parent's sessionId on the inherited records. Count it, remember the
		// immediate parent, and skip — the fork document doesn't repeat it.
		if isMessage {
			p.inheritedMsgs++
			p.parentID = e.SessionID
		}
		return
	}
	ts := parseTime(e.Timestamp)
	if isMessage && !p.ownStarted {
		p.ownStarted = true
		if p.inheritedMsgs > 0 {
			p.sess.ForkedFrom = p.parentID
			p.sess.InheritedMessages = p.inheritedMsgs
			// Anchor the fork to its divergence point. Own metadata
			// (queue-op/title) before the first own message can carry an
			// earlier timestamp; the fork's work begins here, and from now on
			// observeMeta leaves StartedAt alone (see the ForkedFrom guard).
			if !ts.IsZero() {
				p.sess.StartedAt = ts
			}
		}
	}
	p.observeMeta(e, ts)

	switch e.Type {
	case "user":
		p.handleUser(e, ts)
	case "assistant":
		p.handleAssistant(e, ts)
	case "ai-title":
		if e.AITitle != "" {
			p.sess.AITitle = e.AITitle
		}
	case "custom-title":
		if e.CustomTitle != "" {
			p.sess.CustomTitle = e.CustomTitle
		}
	case "pr-link":
		p.handlePRLink(e, ts)
	}
	// last-prompt, attachment, system, queue-operation: intentionally ignored.
}

// isOwn reports whether a record belongs to this session rather than to a parent
// it was forked from. A fork's copied prefix carries the parent's sessionId;
// everything the fork itself produced carries the canonical (filename) id.
// Records without a sessionId (rare metadata) are treated as own.
func (p *parser) isOwn(e *rawEntry) bool {
	return e.SessionID == "" || e.SessionID == p.canonicalID
}

func sessionIDFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".jsonl")
}

func (p *parser) observeMeta(e *rawEntry, ts time.Time) {
	if !ts.IsZero() {
		// For a fork, StartedAt is pinned to the first own message (the
		// divergence point) and must not drift to an earlier own-metadata
		// timestamp; for a non-fork it tracks the earliest record as usual.
		if p.sess.ForkedFrom == "" && (p.sess.StartedAt.IsZero() || ts.Before(p.sess.StartedAt)) {
			p.sess.StartedAt = ts
		}
		if ts.After(p.sess.EndedAt) {
			p.sess.EndedAt = ts
		}
	}
	if e.Version != "" {
		p.sess.Version = e.Version
	}
	if e.Entrypoint != "" {
		p.sess.Entrypoint = e.Entrypoint
	}
	if e.CWD != "" && !p.seenCWD[e.CWD] {
		p.seenCWD[e.CWD] = true
		p.sess.CWDs = append(p.sess.CWDs, e.CWD)
	}
	if e.GitBranch != "" && (e.Type == "user" || e.Type == "assistant") {
		p.seq++
		p.branchHits[e.GitBranch]++
		p.branchLast[e.GitBranch] = p.seq
	}
}

func (p *parser) handleUser(e *rawEntry, ts time.Time) {
	if e.IsMeta || e.Message == nil {
		return
	}
	kind, blocks := decodeContent(e.Message.Content)
	switch kind {
	case contentString:
		text, isCmd := cleanUserText(blocks[0].Text)
		p.emitUserPrompt(text, isCmd, ts, e.GitBranch)
	case contentBlocks:
		var humanText []string
		for i := range blocks {
			b := &blocks[i]
			switch b.Type {
			case "tool_result":
				p.attachResult(b)
			case "text":
				if t, _ := cleanUserText(b.Text); t != "" {
					humanText = append(humanText, t)
				}
			}
		}
		if joined := strings.TrimSpace(strings.Join(humanText, "\n\n")); joined != "" {
			p.emitUserPrompt(joined, false, ts, e.GitBranch)
		}
	}
}

func (p *parser) emitUserPrompt(text string, isCmd bool, ts time.Time, branch string) {
	if text == "" {
		return
	}
	if isCmd {
		text = "Ran command `" + text + "`"
	}
	p.sess.UserTurns++
	if p.sess.FirstPrompt == "" && !isCmd {
		p.sess.FirstPrompt = text
	}
	p.sess.Events = append(p.sess.Events, Event{
		Kind:      KindUserPrompt,
		Timestamp: ts,
		Branch:    branch,
		Text:      text,
	})
}

func (p *parser) handleAssistant(e *rawEntry, ts time.Time) {
	if e.Message == nil {
		return
	}
	kind, blocks := decodeContent(e.Message.Content)
	if kind != contentBlocks {
		return
	}
	p.sess.AssistantTurns++
	for i := range blocks {
		b := &blocks[i]
		switch b.Type {
		case "text":
			if t := strings.TrimSpace(b.Text); t != "" {
				p.sess.Events = append(p.sess.Events, Event{
					Kind: KindAssistantText, Timestamp: ts, Branch: e.GitBranch, Text: t,
				})
			}
		case "thinking":
			if t := strings.TrimSpace(b.Thinking); t != "" {
				p.sess.Events = append(p.sess.Events, Event{
					Kind: KindThinking, Timestamp: ts, Branch: e.GitBranch, Text: t,
				})
			}
		case "tool_use":
			p.emitToolCall(b, ts, e.GitBranch)
		}
	}
}

func (p *parser) emitToolCall(b *rawBlock, ts time.Time, branch string) {
	summary, detail, sub := humanizeTool(b.Name, b.Input)
	tc := &ToolCall{
		Name:     b.Name,
		Summary:  summary,
		Detail:   detail,
		Subagent: sub,
	}
	if b.ID != "" {
		p.toolByID[b.ID] = tc
		if pr, ok := p.pending[b.ID]; ok {
			applyResult(tc, pr)
			delete(p.pending, b.ID)
		}
	}
	p.sess.ToolCalls++
	p.toolHits[b.Name]++
	p.sess.Events = append(p.sess.Events, Event{
		Kind: KindToolCall, Timestamp: ts, Branch: branch, Tool: tc,
	})
}

func (p *parser) attachResult(b *rawBlock) {
	text, trimmed := p.trimResult(extractResultText(b.Content))
	pr := pendingResult{text: text, isError: b.IsError, trimmed: trimmed}
	if tc, ok := p.toolByID[b.ToolUseID]; ok {
		applyResult(tc, pr)
		return
	}
	// The tool_use has not been parsed yet (parallel-tool ordering); stash the
	// result so emitToolCall can claim it.
	if b.ToolUseID != "" {
		p.pending[b.ToolUseID] = pr
	}
}

func applyResult(tc *ToolCall, pr pendingResult) {
	tc.Result = pr.text
	tc.IsError = pr.isError
	tc.HasResult = true
	tc.ResultTrimmed = pr.trimmed
}

func (p *parser) handlePRLink(e *rawEntry, ts time.Time) {
	if e.PRURL == "" && e.PRNumber == 0 {
		return
	}
	// CI polling re-links the same PR many times; keep only the first sighting.
	key := e.PRURL
	if key == "" {
		key = fmt.Sprintf("#%d", e.PRNumber)
	}
	if p.seenPR[key] {
		return
	}
	p.seenPR[key] = true
	pr := PullRequest{Number: e.PRNumber, URL: e.PRURL, Repository: e.PRRepository, At: ts}
	p.sess.PRs = append(p.sess.PRs, pr)
	prCopy := pr
	p.sess.Events = append(p.sess.Events, Event{
		Kind: KindPRLink, Timestamp: ts, Branch: e.GitBranch, PR: &prCopy,
	})
}

func (p *parser) finalize() {
	branches := make([]BranchStat, 0, len(p.branchHits))
	for name, count := range p.branchHits {
		branches = append(branches, BranchStat{Branch: name, Count: count})
	}
	// Most-used branch wins; ties break toward the branch touched later in the
	// session (its higher last-occurrence sequence), then by name for stability.
	sort.Slice(branches, func(i, j int) bool {
		a, b := branches[i], branches[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		if la, lb := p.branchLast[a.Branch], p.branchLast[b.Branch]; la != lb {
			return la > lb
		}
		return a.Branch < b.Branch
	})
	p.sess.Branches = branches
	p.sess.ToolUse = sortedStats(p.toolHits)
}

// trimResult caps result text by line count and byte length.
func (p *parser) trimResult(s string) (string, bool) {
	s = strings.TrimRight(s, "\n")
	trimmed := false
	if lines := strings.Split(s, "\n"); len(lines) > p.opts.MaxResultLines {
		s = strings.Join(lines[:p.opts.MaxResultLines], "\n")
		trimmed = true
	}
	if len(s) > p.opts.MaxResultBytes {
		s = s[:p.opts.MaxResultBytes]
		trimmed = true
	}
	return strings.TrimSpace(s), trimmed
}

type contentKind int

const (
	contentNone contentKind = iota
	contentString
	contentBlocks
)

// decodeContent normalizes the polymorphic message content (string or block
// array). For a bare string it returns a single synthetic text block.
func decodeContent(raw json.RawMessage) (contentKind, []rawBlock) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return contentNone, nil
	}
	switch trimmed[0] {
	case '"':
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return contentNone, nil
		}
		return contentString, []rawBlock{{Type: "text", Text: s}}
	case '[':
		var blocks []rawBlock
		if err := json.Unmarshal(trimmed, &blocks); err != nil {
			return contentNone, nil
		}
		return contentBlocks, blocks
	default:
		return contentNone, nil
	}
}

// extractResultText pulls human-readable text out of a tool_result content
// field, which may be a string or an array of typed blocks.
func extractResultText(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ""
	}
	switch trimmed[0] {
	case '"':
		var s string
		_ = json.Unmarshal(trimmed, &s)
		return s
	case '[':
		var blocks []rawBlock
		if err := json.Unmarshal(trimmed, &blocks); err != nil {
			return ""
		}
		var parts []string
		for _, b := range blocks {
			switch b.Type {
			case "text":
				parts = append(parts, b.Text)
			case "image":
				parts = append(parts, "[image]")
			}
		}
		return strings.Join(parts, "\n")
	default:
		return string(trimmed)
	}
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

func sortedStats(hits map[string]int) []ToolStat {
	stats := make([]ToolStat, 0, len(hits))
	for name, count := range hits {
		stats = append(stats, ToolStat{Name: name, Count: count})
	}
	sort.Slice(stats, func(i, j int) bool {
		if stats[i].Count != stats[j].Count {
			return stats[i].Count > stats[j].Count
		}
		return stats[i].Name < stats[j].Name
	})
	return stats
}
