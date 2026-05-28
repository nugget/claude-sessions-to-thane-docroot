package transcript

import (
	"strings"
	"testing"
)

const sampleJSONL = `{"type":"queue-operation","operation":"enqueue","sessionId":"11111111-aaaa-bbbb","timestamp":"2026-05-01T10:00:00Z"}
{"type":"custom-title","customTitle":"My custom title","sessionId":"11111111-aaaa-bbbb"}
{"type":"ai-title","aiTitle":"AI generated title","sessionId":"11111111-aaaa-bbbb"}
{"type":"user","sessionId":"11111111-aaaa-bbbb","timestamp":"2026-05-01T10:00:01Z","gitBranch":"main","cwd":"/repo","version":"2.1.0","entrypoint":"cli","message":{"role":"user","content":"Please do the thing. Second sentence."}}
{"type":"assistant","sessionId":"11111111-aaaa-bbbb","timestamp":"2026-05-01T10:00:02Z","gitBranch":"main","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Let me think"},{"type":"text","text":"On it."},{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"ls","description":"List files"}}]}}
{"type":"user","sessionId":"11111111-aaaa-bbbb","timestamp":"2026-05-01T10:00:03Z","gitBranch":"feature/x","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"file1\nfile2"}]}}
{"type":"user","isMeta":true,"sessionId":"11111111-aaaa-bbbb","timestamp":"2026-05-01T10:00:04Z","message":{"role":"user","content":"META should be skipped"}}
{"type":"user","sessionId":"11111111-aaaa-bbbb","timestamp":"2026-05-01T10:00:05Z","gitBranch":"feature/x","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"early result","is_error":true}]}}
{"type":"assistant","sessionId":"11111111-aaaa-bbbb","timestamp":"2026-05-01T10:00:06Z","gitBranch":"feature/x","message":{"role":"assistant","content":[{"type":"tool_use","id":"t2","name":"Read","input":{"file_path":"/a/b.go"}}]}}
{"type":"pr-link","sessionId":"11111111-aaaa-bbbb","prNumber":42,"prUrl":"https://gh/pr/42","prRepository":"nugget/x","timestamp":"2026-05-01T10:00:07Z"}
{"type":"pr-link","sessionId":"11111111-aaaa-bbbb","prNumber":42,"prUrl":"https://gh/pr/42","prRepository":"nugget/x","timestamp":"2026-05-01T10:00:08Z"}
`

func parseSample(t *testing.T) *Session {
	t.Helper()
	s, err := parse(strings.NewReader(sampleJSONL), "sample.jsonl", Options{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return s
}

func TestParseSessionMetadata(t *testing.T) {
	s := parseSample(t)
	if s.ID != "11111111-aaaa-bbbb" {
		t.Errorf("ID = %q", s.ID)
	}
	if s.CustomTitle != "My custom title" || s.AITitle != "AI generated title" {
		t.Errorf("titles = %q / %q", s.CustomTitle, s.AITitle)
	}
	if s.Title() != "My custom title" {
		t.Errorf("Title() = %q, want custom title to win", s.Title())
	}
	if !strings.HasPrefix(s.FirstPrompt, "Please do the thing") {
		t.Errorf("FirstPrompt = %q", s.FirstPrompt)
	}
	if s.Version != "2.1.0" || s.Entrypoint != "cli" {
		t.Errorf("version/entrypoint = %q / %q", s.Version, s.Entrypoint)
	}
	if got := s.UserTurns; got != 1 {
		t.Errorf("UserTurns = %d, want 1 (meta + tool_result entries excluded)", got)
	}
	if s.AssistantTurns != 2 {
		t.Errorf("AssistantTurns = %d, want 2", s.AssistantTurns)
	}
	if s.ToolCalls != 2 {
		t.Errorf("ToolCalls = %d, want 2", s.ToolCalls)
	}
}

func TestParseEventOrderAndResults(t *testing.T) {
	s := parseSample(t)
	wantKinds := []EventKind{
		KindUserPrompt, KindThinking, KindAssistantText, KindToolCall, KindToolCall, KindPRLink,
	}
	if len(s.Events) != len(wantKinds) {
		t.Fatalf("got %d events, want %d: %+v", len(s.Events), len(wantKinds), s.Events)
	}
	for i, want := range wantKinds {
		if s.Events[i].Kind != want {
			t.Errorf("event %d kind = %q, want %q", i, s.Events[i].Kind, want)
		}
	}

	bash := s.Events[3].Tool
	if bash == nil || bash.Name != "Bash" || bash.Summary != "List files" {
		t.Fatalf("bash tool = %+v", bash)
	}
	if !bash.HasResult || bash.Result != "file1\nfile2" {
		t.Errorf("bash result = %q hasResult=%v", bash.Result, bash.HasResult)
	}

	// The Read result arrived in the log before the Read tool_use; it must still
	// be attached via the pending-result buffer.
	read := s.Events[4].Tool
	if read == nil || read.Name != "Read" {
		t.Fatalf("read tool = %+v", read)
	}
	if !read.HasResult || read.Result != "early result" || !read.IsError {
		t.Errorf("out-of-order read result not attached: %+v", read)
	}
}

func TestParsePRDedup(t *testing.T) {
	s := parseSample(t)
	if len(s.PRs) != 1 {
		t.Fatalf("PRs = %d, want 1 (deduped)", len(s.PRs))
	}
	if s.PRs[0].Number != 42 || s.PRs[0].Repository != "nugget/x" {
		t.Errorf("PR = %+v", s.PRs[0])
	}
}

func TestParseHomeBranchByFrequency(t *testing.T) {
	s := parseSample(t)
	// feature/x is referenced by more entries than main.
	if hb := s.HomeBranch(); hb != "feature/x" {
		t.Errorf("HomeBranch = %q, want feature/x", hb)
	}
	touched := s.TouchedBranches()
	if len(touched) != 2 {
		t.Errorf("TouchedBranches = %v, want 2 distinct", touched)
	}
}

func TestParseEmptyInputIsSafe(t *testing.T) {
	s, err := parse(strings.NewReader(""), "empty.jsonl", Options{})
	if err != nil {
		t.Fatalf("parse empty: %v", err)
	}
	if len(s.Events) != 0 || s.UserTurns != 0 {
		t.Errorf("expected empty session, got %+v", s)
	}
}

func TestParseSkipsMalformedLines(t *testing.T) {
	in := "{not json}\n" + sampleJSONL
	s, err := parse(strings.NewReader(in), "x.jsonl", Options{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.ID == "" {
		t.Error("expected valid lines to parse despite a leading malformed line")
	}
}

func TestTrimResultRespectsLimits(t *testing.T) {
	p := &parser{opts: Options{MaxResultLines: 2, MaxResultBytes: 1000}}
	got, trimmed := p.trimResult("a\nb\nc\nd")
	if !trimmed || got != "a\nb" {
		t.Errorf("trimResult lines = %q trimmed=%v", got, trimmed)
	}
	p = &parser{opts: Options{MaxResultLines: 100, MaxResultBytes: 3}}
	got, trimmed = p.trimResult("abcdef")
	if !trimmed || got != "abc" {
		t.Errorf("trimResult bytes = %q trimmed=%v", got, trimmed)
	}
}
