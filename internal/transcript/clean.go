package transcript

import (
	"regexp"
	"strings"
)

// wrapperTags are harness-injected envelopes that wrap user-role content but
// are not things the human typed. They are stripped from rendered prompts.
var wrapperTags = []string{
	"system-reminder",
	"local-command-stdout",
	"user-prompt-submit-hook",
	"command-message",
}

var (
	commandNameRE = regexp.MustCompile(`(?s)<command-name>(.*?)</command-name>`)
	commandArgsRE = regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`)
)

// cleanUserText normalizes a user-role text payload into something worth
// rendering. It returns ("", false) when nothing human-authored remains.
//
// Slash-command invocations are surfaced as a compact "/cmd args" string with
// isCommand=true so the renderer can present them as an action rather than a
// quoted prompt; their stdout envelope is discarded as noise.
func cleanUserText(raw string) (text string, isCommand bool) {
	if m := commandNameRE.FindStringSubmatch(raw); m != nil {
		cmd := strings.TrimSpace(m[1])
		if cmd == "" {
			return "", false
		}
		if a := commandArgsRE.FindStringSubmatch(raw); a != nil {
			if args := strings.TrimSpace(a[1]); args != "" {
				cmd += " " + args
			}
		}
		return cmd, true
	}
	for _, tag := range wrapperTags {
		raw = stripTagBlocks(raw, tag)
	}
	return strings.TrimSpace(raw), false
}

// stripTagBlocks removes every <tag ...>...</tag> span (and any dangling
// <tag ...> with no close) for the given tag name.
func stripTagBlocks(s, tag string) string {
	open := "<" + tag
	close := "</" + tag + ">"
	for {
		start := strings.Index(s, open)
		if start < 0 {
			return s
		}
		end := strings.Index(s[start:], close)
		if end < 0 {
			return strings.TrimRight(s[:start], " \t\n")
		}
		s = s[:start] + s[start+end+len(close):]
	}
}
