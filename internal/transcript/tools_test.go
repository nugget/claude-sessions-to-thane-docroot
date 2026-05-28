package transcript

import "testing"

func TestHumanizeTool(t *testing.T) {
	tests := []struct {
		name        string
		tool        string
		input       map[string]any
		wantSummary string
		wantDetail  string
		wantSubType string
	}{
		{
			"bash with description",
			"Bash", map[string]any{"command": "go test ./...", "description": "Run tests"},
			"Run tests", "go test ./...", "",
		},
		{
			"bash without description uses first line",
			"Bash", map[string]any{"command": "ls\ncd x"},
			"ls …", "ls\ncd x", "",
		},
		{"read", "Read", map[string]any{"file_path": "/a/b.go"}, "read /a/b.go", "", ""},
		{"edit", "Edit", map[string]any{"file_path": "/a/b.go"}, "edit /a/b.go", "", ""},
		{
			"task subagent",
			"Task", map[string]any{"subagent_type": "Explore", "description": "Find code"},
			"dispatch subagent Explore — Find code", "", "Explore",
		},
		{
			"mcp tool",
			"mcp__github__create_issue", map[string]any{"title": "Bug"},
			"create_issue (github): title=Bug", "", "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, detail, sub := humanizeTool(tt.tool, tt.input)
			if summary != tt.wantSummary {
				t.Errorf("summary = %q, want %q", summary, tt.wantSummary)
			}
			if detail != tt.wantDetail {
				t.Errorf("detail = %q, want %q", detail, tt.wantDetail)
			}
			if tt.wantSubType != "" {
				if sub == nil || sub.Type != tt.wantSubType {
					t.Errorf("subagent = %+v, want type %q", sub, tt.wantSubType)
				}
			}
		})
	}
}
