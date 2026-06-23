// Package docroot reconciles a desired set of generated markdown files against
// a target directory. The sync is idempotent: a re-run with identical inputs
// writes nothing.
//
// By default the target is treated as an ARCHIVE — owned files this tool
// previously wrote that the current run did NOT produce are listed as orphans
// in the Result but are NOT deleted. That criterion is purely about the current
// render plan: an upstream session being pruned is the common case, but a
// transcript that failed to parse, was filtered, or was simply not visible this
// run would surface the same way — none of those are good reasons to drop
// archived history. Set Plan.Prune to opt into a true mirror that deletes
// orphans and prunes the empty directories left behind. Deletion is always
// bounded to files carrying this tool's generated_by marker, so hand-authored
// files, .git, and signing material are never touched.
package docroot

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// File is one document to materialize, keyed by a forward-slash relative path.
type File struct {
	RelPath string
	Content string
}

// Plan describes a full reconcile.
type Plan struct {
	TargetDir   string
	Files       []File
	OwnedMarker string // value of generated_by stamped on tool-owned files
	Prune       bool   // when true, delete owned orphans and prune empty dirs
}

// Result reports what the reconcile did (or would do, when dry-run).
//
// Orphans are owned files that the current render plan did not produce
// (whatever the reason — upstream pruning, a parse failure, a filter, anything).
// In the default archive mode they are listed but left in place; in prune mode
// the same set is removed and appears in Deleted instead.
type Result struct {
	Created    []string
	Updated    []string
	Unchanged  []string
	Orphans    []string
	Deleted    []string
	PrunedDirs []string
}

// Sync materializes plan.Files under plan.TargetDir and removes orphaned
// tool-owned files. When dryRun is true no filesystem changes are made but the
// Result still reflects the actions that would be taken.
func Sync(plan Plan, dryRun bool, logger *slog.Logger) (*Result, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if strings.TrimSpace(plan.TargetDir) == "" {
		return nil, fmt.Errorf("target directory is required")
	}
	res := &Result{}

	desired := make(map[string]string, len(plan.Files))
	for _, f := range plan.Files {
		rel := filepath.ToSlash(f.RelPath)
		if _, dup := desired[rel]; dup {
			return nil, fmt.Errorf("duplicate document path %q in render plan", rel)
		}
		desired[rel] = f.Content
	}

	if !dryRun {
		if err := os.MkdirAll(plan.TargetDir, 0o755); err != nil {
			return nil, fmt.Errorf("create target %s: %w", plan.TargetDir, err)
		}
	}

	if err := writeDesired(plan, desired, dryRun, res, logger); err != nil {
		return nil, err
	}
	if err := handleOrphans(plan, desired, dryRun, res, logger); err != nil {
		return nil, err
	}
	if plan.Prune {
		if err := pruneEmptyDirs(plan.TargetDir, dryRun, res); err != nil {
			return nil, err
		}
	}

	sort.Strings(res.Created)
	sort.Strings(res.Updated)
	sort.Strings(res.Unchanged)
	sort.Strings(res.Orphans)
	sort.Strings(res.Deleted)
	sort.Strings(res.PrunedDirs)
	return res, nil
}

func writeDesired(plan Plan, desired map[string]string, dryRun bool, res *Result, logger *slog.Logger) error {
	for rel, content := range desired {
		abs := filepath.Join(plan.TargetDir, filepath.FromSlash(rel))
		existing, err := os.ReadFile(abs)
		switch {
		case err == nil && string(existing) == content:
			res.Unchanged = append(res.Unchanged, rel)
			continue
		case err == nil:
			res.Updated = append(res.Updated, rel)
		case os.IsNotExist(err):
			res.Created = append(res.Created, rel)
		default:
			return fmt.Errorf("read existing %s: %w", abs, err)
		}
		if dryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", rel, err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", rel, err)
		}
		logger.Debug("wrote document", "path", rel)
	}
	return nil
}

// handleOrphans walks the target for owned files that aren't in the desired
// set this run — whatever the reason they're missing from it. In archive mode
// (Plan.Prune == false) they're recorded in Result.Orphans and left in place,
// so a transient blip (upstream prune, parse failure, filtered transcript) does
// not silently destroy archived history. In prune mode they're deleted and
// recorded in Result.Deleted. Either way only files carrying the OwnedMarker
// are considered.
func handleOrphans(plan Plan, desired map[string]string, dryRun bool, res *Result, logger *slog.Logger) error {
	walkErr := filepath.WalkDir(plan.TargetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, relErr := filepath.Rel(plan.TargetDir, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if _, want := desired[rel]; want {
			return nil
		}
		owned, ownErr := isOwned(path, plan.OwnedMarker)
		if ownErr != nil {
			return ownErr
		}
		if !owned {
			return nil
		}
		if !plan.Prune {
			res.Orphans = append(res.Orphans, rel)
			logger.Debug("orphan kept (archive mode; pass --prune to remove)", "path", rel)
			return nil
		}
		res.Deleted = append(res.Deleted, rel)
		if dryRun {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("delete orphan %s: %w", rel, err)
		}
		logger.Debug("deleted orphaned document", "path", rel)
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("scan target for orphans: %w", walkErr)
	}
	return nil
}

// isOwned reports whether a markdown file carries our generated_by marker. Only
// the head of the file (where frontmatter lives) is inspected.
func isOwned(path, marker string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	head := make([]byte, 2048)
	n, err := f.Read(head)
	if err != nil && n == 0 {
		return false, nil
	}
	needle := fmt.Sprintf("generated_by: %q", marker)
	return strings.Contains(string(head[:n]), needle), nil
}

// pruneEmptyDirs removes directories that became empty, bottom-up, without ever
// removing the target root or descending into .git.
func pruneEmptyDirs(target string, dryRun bool, res *Result) error {
	var dirs []string
	err := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			if path != target {
				dirs = append(dirs, path)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scan target for empty dirs: %w", err)
	}
	// Deepest first so a parent can become empty once its children are removed.
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("read dir %s: %w", dir, err)
		}
		if len(entries) > 0 {
			continue
		}
		rel, _ := filepath.Rel(target, dir)
		res.PrunedDirs = append(res.PrunedDirs, filepath.ToSlash(rel))
		if dryRun {
			continue
		}
		if err := os.Remove(dir); err != nil {
			return fmt.Errorf("prune empty dir %s: %w", dir, err)
		}
	}
	return nil
}
