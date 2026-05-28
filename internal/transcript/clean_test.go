package transcript

import "testing"

func TestCleanUserText(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantText  string
		wantIsCmd bool
	}{
		{"plain", "Just a normal prompt", "Just a normal prompt", false},
		{
			"strips system reminder",
			"Do the thing\n<system-reminder>\nignore me\n</system-reminder>",
			"Do the thing",
			false,
		},
		{
			"strips local command stdout",
			"<local-command-stdout>noise output</local-command-stdout>real ask",
			"real ask",
			false,
		},
		{
			"command with args",
			"<command-name>/review</command-name><command-args>main</command-args><local-command-stdout>...</local-command-stdout>",
			"/review main",
			true,
		},
		{
			"command without args",
			"<command-name>/clear</command-name>",
			"/clear",
			true,
		},
		{"dangling open tag", "keep this <system-reminder>and drop the rest", "keep this", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, isCmd := cleanUserText(tt.in)
			if text != tt.wantText || isCmd != tt.wantIsCmd {
				t.Errorf("cleanUserText(%q) = (%q, %v), want (%q, %v)", tt.in, text, isCmd, tt.wantText, tt.wantIsCmd)
			}
		})
	}
}
