// Command transcript-exporter exports a Claude Code project's session
// transcripts into a hierarchy of markdown documents that form a document root
// consumed by the thane-ai-agent project.
//
// Usage:
//
//	# one shared root, project under a subpath (recommended for many projects):
//	transcript-exporter --project <name|path> --root <root-dir> --subpath <agent>/<project>
//
//	# or a standalone target that is itself the root:
//	transcript-exporter --project <name|path> --target <dir>
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nugget/session-transcript-exporter/internal/docroot"
	"github.com/nugget/session-transcript-exporter/internal/export"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		project     = flag.String("project", "", "Claude project to export: a name, the repo path, or the encoded ~/.claude/projects dir")
		root        = flag.String("root", "", "shared document-root directory; project docs land under --subpath. root-name derives from its basename")
		subpath     = flag.String("subpath", "", "relative path within --root for this project's docs, e.g. claude/thane-ai-agent")
		target      = flag.String("target", "", "standalone target directory that is itself the document root (alternative to --root)")
		rootName    = flag.String("root-name", "", "override the thane root name for frontmatter/refs (default: basename of --root or --target)")
		worktrees   = flag.Bool("worktrees", true, "merge worktree-sibling project dirs into the same root")
		dryRun      = flag.Bool("dry-run", false, "report changes without writing to disk")
		prune       = flag.Bool("prune", false, "delete owned files whose source transcript is gone (default: leave them as archive)")
		thinking    = flag.Bool("thinking", true, "include assistant thinking blocks in the narrative")
		maxThinking = flag.Int("max-thinking-chars", 0, "cap per thinking block (0 = default)")
		maxResLines = flag.Int("max-result-lines", 0, "cap tool-result text at N lines (0 = default)")
		maxResBytes = flag.Int("max-result-bytes", 0, "cap tool-result text at N bytes (0 = default)")
		tz          = flag.String("timezone", "Local", `timezone for document dates: "Local", "UTC", or an IANA name`)
		verbose     = flag.Bool("verbose", false, "enable debug logging")
	)
	flag.Parse()

	if *project == "" {
		flag.Usage()
		return fmt.Errorf("--project is required")
	}
	targetDir, resolvedRootName, err := resolveLocation(*root, *subpath, *target, *rootName)
	if err != nil {
		flag.Usage()
		return err
	}

	loc, err := loadLocation(*tz)
	if err != nil {
		return err
	}

	level := slog.LevelInfo
	if *verbose {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	cfg := export.Config{
		ProjectArg:       *project,
		TargetDir:        targetDir,
		RootName:         resolvedRootName,
		IncludeWorktrees: *worktrees,
		DryRun:           *dryRun,
		Prune:            *prune,
		IncludeThinking:  *thinking,
		MaxThinkingChars: *maxThinking,
		MaxResultLines:   *maxResLines,
		MaxResultBytes:   *maxResBytes,
		Location:         loc,
		Logger:           logger,
	}

	result, err := cfg.Run()
	if err != nil {
		return err
	}
	printSummary(result, *dryRun)
	return nil
}

// resolveLocation turns the location flags into the concrete write directory
// and the thane root name. Either --root (with optional --subpath) or --target
// must be given, but not both. With --root the root name derives from the
// root's basename so it stays stable across every project written under the
// same shared root; --root-name overrides it.
func resolveLocation(root, subpath, target, rootName string) (dir, name string, err error) {
	root = strings.TrimSpace(root)
	target = strings.TrimSpace(target)
	subpath = strings.TrimSpace(subpath)
	rootName = strings.TrimSpace(rootName)

	switch {
	case root != "" && target != "":
		return "", "", fmt.Errorf("use either --root or --target, not both")
	case root == "" && target == "":
		return "", "", fmt.Errorf("one of --root or --target is required")
	case target != "":
		if subpath != "" {
			return "", "", fmt.Errorf("--subpath requires --root")
		}
		return target, defaultName(rootName, target), nil
	default: // root set
		clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(subpath)))
		if filepath.IsAbs(subpath) || clean == ".." || strings.HasPrefix(clean, "../") {
			return "", "", fmt.Errorf("--subpath %q must stay within --root", subpath)
		}
		dir = root
		if clean != "" && clean != "." {
			dir = filepath.Join(root, filepath.FromSlash(clean))
		}
		return dir, defaultName(rootName, root), nil
	}
}

// defaultName returns the explicit root name, or the basename of path when none
// was given.
func defaultName(rootName, path string) string {
	if rootName != "" {
		return rootName
	}
	return filepath.Base(filepath.Clean(path))
}

func loadLocation(name string) (*time.Location, error) {
	switch name {
	case "", "Local":
		return time.Local, nil
	case "UTC":
		return time.UTC, nil
	default:
		loc, err := time.LoadLocation(name)
		if err != nil {
			return nil, fmt.Errorf("invalid --timezone %q: %w", name, err)
		}
		return loc, nil
	}
}

func printSummary(r *docroot.Result, dryRun bool) {
	verb := "synced"
	if dryRun {
		verb = "would sync (dry-run)"
	}
	fmt.Printf("%s: %d created, %d updated, %d unchanged, %d deleted, %d dirs pruned\n",
		verb, len(r.Created), len(r.Updated), len(r.Unchanged), len(r.Deleted), len(r.PrunedDirs))
	if n := len(r.Orphans); n > 0 {
		fmt.Printf("%d orphan(s) kept (no matching source; pass --prune to remove, --verbose to list)\n", n)
	}
}
