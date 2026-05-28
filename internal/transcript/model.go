// Package transcript parses Claude Code session transcript JSONL files into a
// narrative-ready model. The on-disk format is an append-only log of mixed
// entry types; this package distills one file into an ordered sequence of
// human/assistant/tool events plus session-level metadata.
package transcript

import "time"

// Session is the distilled view of one transcript file.
type Session struct {
	ID         string // sessionId, also the .jsonl filename stem
	SourceFile string // absolute path to the source .jsonl

	CustomTitle string // user-assigned title (custom-title entry)
	AITitle     string // model-generated title (ai-title entry)
	FirstPrompt string // text of the first genuine human prompt

	StartedAt time.Time
	EndedAt   time.Time

	CWDs       []string // distinct working directories observed, first-seen order
	Version    string   // last Claude Code version observed
	Entrypoint string   // e.g. "cli", "claude-desktop"

	Branches []BranchStat  // distinct git branches with hit counts, descending
	PRs      []PullRequest // pull requests linked during the session
	ToolUse  []ToolStat    // tool name with call count, descending then by name

	Events []Event // ordered narrative

	UserTurns      int
	AssistantTurns int
	ToolCalls      int
}

// BranchStat counts how many events referenced a given git branch.
type BranchStat struct {
	Branch string
	Count  int
}

// PullRequest is a PR linked from the session (pr-link entry).
type PullRequest struct {
	Number     int
	URL        string
	Repository string
	At         time.Time
}

// ToolStat counts invocations of a named tool.
type ToolStat struct {
	Name  string
	Count int
}

// EventKind discriminates the Event union.
type EventKind string

const (
	KindUserPrompt    EventKind = "user_prompt"
	KindAssistantText EventKind = "assistant_text"
	KindThinking      EventKind = "thinking"
	KindToolCall      EventKind = "tool_call"
	KindPRLink        EventKind = "pr_link"
)

// Event is one ordered item in the narrative. Only the fields relevant to Kind
// are populated.
type Event struct {
	Kind      EventKind
	Timestamp time.Time
	Branch    string

	// Text carries the body for user_prompt, assistant_text, and thinking.
	Text string

	// Tool is populated for tool_call events.
	Tool *ToolCall

	// PR is populated for pr_link events.
	PR *PullRequest
}

// ToolCall is one assistant tool invocation paired with its result.
type ToolCall struct {
	Name    string
	Summary string // one-line humanized description of the call
	Detail  string // optional multi-line detail (e.g. a bash command), no fence

	Result        string // textual tool result (possibly truncated)
	IsError       bool
	HasResult     bool
	ResultTrimmed bool // true when Result was truncated for length

	// Subagent is set for Task/Agent dispatches.
	Subagent *SubagentRef
}

// SubagentRef describes a dispatched subagent (from a Task/Agent tool call).
type SubagentRef struct {
	Type        string
	Description string
}
