package docroot

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

const marker = "session-transcript-exporter"

func owned(body string) string {
	return "---\ngenerated_by: \"" + marker + "\"\n---\n\n" + body
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func mkPlan(dir string, files ...File) Plan {
	return Plan{TargetDir: dir, OwnedMarker: marker, Files: files}
}

func mkPrunePlan(dir string, files ...File) Plan {
	p := mkPlan(dir, files...)
	p.Prune = true
	return p
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestSyncCreateUpdateUnchanged(t *testing.T) {
	dir := t.TempDir()
	log := quietLogger()

	plan := mkPlan(dir,
		File{RelPath: "README.md", Content: owned("root")},
		File{RelPath: "main/a.md", Content: owned("alpha")},
	)

	res, err := Sync(plan, false, log)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 2 || len(res.Updated) != 0 {
		t.Fatalf("first sync: %+v", res)
	}
	if got := read(t, filepath.Join(dir, "main", "a.md")); got != owned("alpha") {
		t.Errorf("content = %q", got)
	}

	// Re-sync: nothing changes.
	res, err = Sync(plan, false, log)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Unchanged) != 2 || len(res.Created) != 0 || len(res.Updated) != 0 {
		t.Fatalf("idempotent re-sync should be all-unchanged: %+v", res)
	}

	// Change content: one update.
	plan.Files[1].Content = owned("alpha v2")
	res, err = Sync(plan, false, log)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Updated) != 1 || len(res.Unchanged) != 1 {
		t.Fatalf("expected one update: %+v", res)
	}
}

func TestSyncDeletesOwnedOrphansAndPrunes(t *testing.T) {
	dir := t.TempDir()
	log := quietLogger()

	full := mkPrunePlan(dir,
		File{RelPath: "README.md", Content: owned("root")},
		File{RelPath: "feat/x/old.md", Content: owned("old")},
	)
	if _, err := Sync(full, false, log); err != nil {
		t.Fatal(err)
	}

	// Drop the branch file from the plan; with --prune it should be deleted and
	// its now-empty directories pruned.
	reduced := mkPrunePlan(dir, File{RelPath: "README.md", Content: owned("root")})
	res, err := Sync(reduced, false, log)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deleted) != 1 || res.Deleted[0] != "feat/x/old.md" {
		t.Fatalf("expected orphan deletion: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "feat")); !os.IsNotExist(err) {
		t.Errorf("expected pruned empty dir feat/, stat err = %v", err)
	}
}

// TestSyncArchiveModeKeepsOrphans guards the post-incident default: when the
// source shrinks (e.g. Claude Code pruned an upstream session), the previously
// archived doc is reported as an orphan but left in place. This is the contract
// that lets the target stand as a permanent archive of historical sessions.
func TestSyncArchiveModeKeepsOrphans(t *testing.T) {
	dir := t.TempDir()
	log := quietLogger()

	full := mkPlan(dir,
		File{RelPath: "README.md", Content: owned("root")},
		File{RelPath: "feat/x/old.md", Content: owned("old")},
	)
	if _, err := Sync(full, false, log); err != nil {
		t.Fatal(err)
	}

	reduced := mkPlan(dir, File{RelPath: "README.md", Content: owned("root")})
	res, err := Sync(reduced, false, log)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deleted) != 0 || len(res.PrunedDirs) != 0 {
		t.Fatalf("archive mode must not delete or prune: %+v", res)
	}
	if len(res.Orphans) != 1 || res.Orphans[0] != "feat/x/old.md" {
		t.Fatalf("expected orphan reported: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "feat", "x", "old.md")); err != nil {
		t.Errorf("orphan file must remain on disk in archive mode: %v", err)
	}
}

func TestSyncLeavesUnownedFiles(t *testing.T) {
	dir := t.TempDir()
	log := quietLogger()

	// A hand-authored file with no marker.
	handPath := filepath.Join(dir, "NOTES.md")
	if err := os.WriteFile(handPath, []byte("# Hand-written\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := mkPlan(dir, File{RelPath: "README.md", Content: owned("root")})
	res, err := Sync(plan, false, log)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Deleted) != 0 {
		t.Errorf("must not delete unowned files: %+v", res.Deleted)
	}
	if _, err := os.Stat(handPath); err != nil {
		t.Errorf("hand-written file was removed: %v", err)
	}
}

func TestSyncDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	log := quietLogger()
	plan := mkPlan(dir, File{RelPath: "README.md", Content: owned("root")})

	res, err := Sync(plan, true, log)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Created) != 1 {
		t.Fatalf("dry-run should report intended creation: %+v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "README.md")); !os.IsNotExist(err) {
		t.Errorf("dry-run must not write files")
	}
}

func TestSyncRejectsDuplicatePaths(t *testing.T) {
	dir := t.TempDir()
	plan := mkPlan(dir,
		File{RelPath: "a.md", Content: owned("one")},
		File{RelPath: "a.md", Content: owned("two")},
	)
	if _, err := Sync(plan, false, quietLogger()); err == nil {
		t.Error("expected error on duplicate document path")
	}
}
