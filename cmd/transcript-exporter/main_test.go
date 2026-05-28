package main

import (
	"path/filepath"
	"testing"
)

func TestResolveLocation(t *testing.T) {
	tests := []struct {
		name     string
		root     string
		subpath  string
		target   string
		rootName string
		wantDir  string
		wantName string
		wantErr  bool
	}{
		{
			name:     "root with subpath derives name from root",
			root:     "/Volumes/Thane/agentic-coding",
			subpath:  "claude/thane-ai-agent",
			wantDir:  filepath.Join("/Volumes/Thane/agentic-coding", "claude", "thane-ai-agent"),
			wantName: "agentic-coding",
		},
		{
			name:     "root without subpath writes at root",
			root:     "/Volumes/Thane/agentic-coding",
			wantDir:  "/Volumes/Thane/agentic-coding",
			wantName: "agentic-coding",
		},
		{
			name:     "explicit root-name overrides",
			root:     "/Volumes/Thane/agentic-coding",
			subpath:  "claude/x",
			rootName: "custom",
			wantDir:  filepath.Join("/Volumes/Thane/agentic-coding", "claude", "x"),
			wantName: "custom",
		},
		{
			name:     "target alone is its own root",
			target:   "/tmp/out",
			wantDir:  "/tmp/out",
			wantName: "out",
		},
		{
			name:     "target with explicit name",
			target:   "/tmp/out",
			rootName: "transcripts",
			wantDir:  "/tmp/out",
			wantName: "transcripts",
		},
		{name: "root and target are mutually exclusive", root: "/a", target: "/b", wantErr: true},
		{name: "neither root nor target", wantErr: true},
		{name: "subpath requires root", target: "/b", subpath: "x", wantErr: true},
		{name: "subpath cannot escape root", root: "/a", subpath: "../escape", wantErr: true},
		{name: "absolute subpath rejected", root: "/a", subpath: "/etc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, name, err := resolveLocation(tt.root, tt.subpath, tt.target, tt.rootName)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got dir=%q name=%q", dir, name)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if dir != tt.wantDir || name != tt.wantName {
				t.Errorf("got (dir=%q, name=%q), want (dir=%q, name=%q)", dir, name, tt.wantDir, tt.wantName)
			}
		})
	}
}
