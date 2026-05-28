// Package claudeproj locates Claude Code session transcripts on disk.
//
// Claude Code stores one directory per project under ~/.claude/projects, named
// by encoding the project's working-directory path: every non-alphanumeric
// character becomes a dash. A git worktree opened inside a project gets its own
// sibling directory whose encoded name extends the main project's name (the
// extra path segments after the repo root encode to a "--…" suffix).
package claudeproj

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Project is a resolved Claude project: one or more source directories whose
// session transcripts should be exported together.
type Project struct {
	DisplayName string   // friendly name for titles/logging
	MainEncoded string   // encoded directory name of the canonical project
	SourceDirs  []string // absolute directories to scan (main + worktree siblings)
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

// encodePath applies Claude Code's path→directory-name encoding.
func encodePath(path string) string {
	return nonAlnum.ReplaceAllString(path, "-")
}

// ProjectsRoot returns the directory that holds per-project transcript dirs,
// honoring CLAUDE_CONFIG_DIR when set.
func ProjectsRoot() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); dir != "" {
		return filepath.Join(dir, "projects"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// Locate resolves a --project argument into a Project. The argument may be:
//   - a filesystem path to the project's working directory (e.g. the repo)
//   - the encoded directory name as it appears under ~/.claude/projects
//   - a bare project name matched against the trailing segment of encoded dirs
//
// When includeWorktrees is true, worktree-sibling directories are folded in.
func Locate(arg string, includeWorktrees bool) (*Project, error) {
	root, err := ProjectsRoot()
	if err != nil {
		return nil, err
	}
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return nil, fmt.Errorf("no project specified")
	}

	main, display, err := resolveMain(root, arg)
	if err != nil {
		return nil, err
	}

	proj := &Project{
		DisplayName: display,
		MainEncoded: main,
		SourceDirs:  []string{filepath.Join(root, main)},
	}
	if includeWorktrees {
		siblings, err := worktreeSiblings(root, main)
		if err != nil {
			return nil, err
		}
		proj.SourceDirs = append(proj.SourceDirs, siblings...)
	}
	return proj, nil
}

// resolveMain returns the encoded directory name of the canonical project and a
// display name for it.
func resolveMain(root, arg string) (encoded, display string, err error) {
	// 1. An existing path on disk (the repo working directory).
	if abs, err := filepath.Abs(arg); err == nil {
		if info, statErr := os.Stat(abs); statErr == nil && info.IsDir() {
			enc := encodePath(abs)
			if dirExists(filepath.Join(root, enc)) {
				return enc, filepath.Base(abs), nil
			}
		}
	}
	// 2. The encoded directory name itself.
	if dirExists(filepath.Join(root, arg)) {
		return arg, displayFromEncoded(arg), nil
	}
	// 3. A bare project name matched against the encoded dirs.
	return matchByName(root, arg)
}

// matchByName finds the single non-worktree project dir whose encoded name ends
// in "-<encoded name>".
func matchByName(root, name string) (encoded, display string, err error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", "", fmt.Errorf("read projects root %s: %w", root, err)
	}
	suffix := "-" + encodePath(name)
	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		if strings.HasSuffix(n, suffix) && !isWorktreeName(n) {
			matches = append(matches, n)
		}
	}
	switch len(matches) {
	case 0:
		return "", "", fmt.Errorf("no project under %s matches %q (pass the full project path instead)", root, name)
	case 1:
		return matches[0], name, nil
	default:
		sort.Strings(matches)
		return "", "", fmt.Errorf("%q is ambiguous; matched %d projects: %s (pass the full project path instead)",
			name, len(matches), strings.Join(matches, ", "))
	}
}

// worktreeSiblings returns absolute paths of dirs that extend main with a "--"
// suffix (a worktree opened inside the project).
func worktreeSiblings(root, main string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read projects root %s: %w", root, err)
	}
	prefix := main + "--"
	var out []string
	for _, e := range entries {
		if e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			out = append(out, filepath.Join(root, e.Name()))
		}
	}
	sort.Strings(out)
	return out, nil
}

// SessionFiles returns every top-level session transcript across the project's
// source directories, sorted for deterministic processing. Subdirectories
// (subagent sidecars) are not session files and are skipped.
func (p *Project) SessionFiles() ([]string, error) {
	var files []string
	for _, dir := range p.SourceDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read session dir %s: %w", dir, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func isWorktreeName(name string) bool {
	return strings.Contains(name, "--")
}

// displayFromEncoded makes a best-effort friendly name from an encoded dir
// name by taking its final path-like segment.
func displayFromEncoded(encoded string) string {
	encoded = strings.TrimSuffix(encoded, "/")
	if i := strings.LastIndex(encoded, "-"); i >= 0 && i < len(encoded)-1 {
		return encoded[i+1:]
	}
	return encoded
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
