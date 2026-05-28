// Command transcript-exporter exports a Claude Code project's session
// transcripts into a hierarchy of markdown documents that form a document root
// consumed by the thane-ai-agent project.
//
// Usage:
//
//	transcript-exporter --project <name|path> --target <dir> [flags]
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
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
		target      = flag.String("target", "", "target document-root directory to sync (required)")
		rootName    = flag.String("root-name", "", "thane root name for frontmatter/refs (default: basename of --target)")
		worktrees   = flag.Bool("worktrees", true, "merge worktree-sibling project dirs into the same root")
		dryRun      = flag.Bool("dry-run", false, "report changes without writing to disk")
		thinking    = flag.Bool("thinking", true, "include assistant thinking blocks in the narrative")
		maxThinking = flag.Int("max-thinking-chars", 0, "cap per thinking block (0 = default)")
		maxResLines = flag.Int("max-result-lines", 0, "cap tool-result text at N lines (0 = default)")
		maxResBytes = flag.Int("max-result-bytes", 0, "cap tool-result text at N bytes (0 = default)")
		tz          = flag.String("timezone", "Local", `timezone for document dates: "Local", "UTC", or an IANA name`)
		verbose     = flag.Bool("verbose", false, "enable debug logging")
	)
	flag.Parse()

	if *project == "" || *target == "" {
		flag.Usage()
		return fmt.Errorf("both --project and --target are required")
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
		TargetDir:        *target,
		RootName:         *rootName,
		IncludeWorktrees: *worktrees,
		DryRun:           *dryRun,
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
}
