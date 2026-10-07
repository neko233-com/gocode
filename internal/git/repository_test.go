package git

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T) (*Repository, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	directory := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.name", "Gocode fixture"}, {"config", "user.email", "fixture@example.invalid"}, {"config", "commit.gpgsign", "false"}, {"config", "core.autocrlf", "false"}, {"config", "core.hooksPath", filepath.ToSlash(filepath.Join(directory, "no-hooks"))}} {
		if output, err := runGit(ctx, directory, nil, 65536, args...); err != nil {
			t.Fatalf("%v %v %s", args, err, output)
		}
	}
	r, err := Open(ctx, directory)
	if err != nil {
		t.Fatal(err)
	}
	return r, ctx
}
func write(t *testing.T, r *Repository, path, text string) {
	t.Helper()
	full := filepath.Join(r.Root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func status(t *testing.T, r *Repository, ctx context.Context) State {
	t.Helper()
	state, err := r.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestRealGitStageUnstageCommitDiffAndStaleIndex(t *testing.T) {
	r, ctx := fixture(t)
	path := "目录/main 世界😀.go"
	original := "package main\r\n// before 世界 😀\r\nfunc main() {}\r\n"
	write(t, r, path, original)
	first := status(t, r, ctx)
	if first.Head != "" || first.Branch != "main" || len(first.Entries) != 1 || first.Entries[0].Path != path {
		t.Fatal(first)
	}
	diff, err := r.Diff(ctx, first, path, false)
	if err != nil || diff.Before != "" || diff.After != original || !strings.Contains(diff.Patch, "+// before 世界 😀") {
		t.Fatal(diff, err)
	}
	staged, err := r.ChangeIndex(ctx, first, []string{path}, true)
	if err != nil || !staged.Entries[0].Staged() {
		t.Fatal(staged, err)
	}
	write(t, r, path, original+"// unstaged\r\n")
	unstaged, err := r.ChangeIndex(ctx, staged, []string{path}, false)
	if err != nil || unstaged.Entries[0].Index != '?' {
		t.Fatal(unstaged, err)
	}
	data, err := os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(path)))
	if err != nil || string(data) != original+"// unstaged\r\n" {
		t.Fatal("unstage changed disk", err)
	}
	staged, err = r.ChangeIndex(ctx, unstaged, []string{path}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = r.Commit(ctx, first, "stale"); !errors.Is(err, ErrStale) {
		t.Fatal("stale index committed", err)
	}
	committed, err := r.Commit(ctx, staged, "First 世界 😀\n\nActual index only")
	if err != nil || committed.Head == "" || len(committed.Entries) != 0 {
		t.Fatal(committed, err)
	}
	write(t, r, path, "package main\r\n// after 世界 😀\r\nfunc main() {}\r\n")
	changed := status(t, r, ctx)
	diff, err = r.Diff(ctx, changed, path, false)
	if err != nil || diff.Before != original+"// unstaged\r\n" || !strings.Contains(diff.After, "after 世界") || !strings.Contains(diff.Patch, "-// before") {
		t.Fatal(diff, err)
	}
	staged, err = r.ChangeIndex(ctx, changed, []string{path}, true)
	if err != nil {
		t.Fatal(err)
	}
	write(t, r, path, "newer uncommitted disk\n")
	diff, err = r.Diff(ctx, staged, path, true)
	if err != nil || !strings.Contains(diff.After, "after 世界") || strings.Contains(diff.After, "newer") {
		t.Fatal("staged diff used working tree", diff, err)
	}
	committed, err = r.Commit(ctx, staged, "Second")
	if err != nil || len(committed.Entries) != 1 || committed.Entries[0].Index != '.' || committed.Entries[0].Worktree != 'M' {
		t.Fatal(committed, err)
	}
	data, _ = os.ReadFile(filepath.Join(r.Root, filepath.FromSlash(path)))
	if string(data) != "newer uncommitted disk\n" {
		t.Fatal("commit modified unstaged disk")
	}
}

func TestRealGitLiteralPathsRenameDeletedAndBinary(t *testing.T) {
	r, ctx := fixture(t)
	old := "old [abc] 世界.go"
	write(t, r, old, "first\nsecond\nthird\n")
	state := status(t, r, ctx)
	state, err := r.ChangeIndex(ctx, state, []string{old}, true)
	if err != nil {
		t.Fatal(err)
	}
	state, err = r.Commit(ctx, state, "Base")
	if err != nil {
		t.Fatal(err)
	}
	new := "new [abc] 😀.go"
	if _, err := runGit(ctx, r.Root, nil, 65536, "mv", "--", old, new); err != nil {
		t.Fatal(err)
	}
	state = status(t, r, ctx)
	if len(state.Entries) != 1 || state.Entries[0].OriginalPath != old || state.Entries[0].Path != new {
		t.Fatal(state)
	}
	diff, err := r.Diff(ctx, state, new, true)
	if err != nil || diff.Before != diff.After {
		t.Fatal(diff, err)
	}
	state, err = r.ChangeIndex(ctx, state, []string{new}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(r.Root, new)); err != nil {
		t.Fatal("unstage rename changed disk", err)
	}
	paths := make([]string, 0, len(state.Entries))
	for _, entry := range state.Entries {
		paths = append(paths, entry.Path)
	}
	state, err = r.ChangeIndex(ctx, state, paths, true)
	if err != nil {
		t.Fatal(err)
	}
	state, err = r.Commit(ctx, state, "Rename")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(r.Root, new)); err != nil {
		t.Fatal(err)
	}
	state = status(t, r, ctx)
	diff, err = r.Diff(ctx, state, new, false)
	if err != nil || diff.After != "" || !strings.Contains(diff.Patch, "-first") {
		t.Fatal(diff, err)
	}
	write(t, r, "binary.dat", "\x00\x01data")
	state = status(t, r, ctx)
	diff, err = r.Diff(ctx, state, "binary.dat", false)
	if err != nil || !diff.Binary || diff.NewBytes != 6 || diff.After != "" {
		t.Fatal(diff, err)
	}
}

func TestGitCancellationOutputLimitsAndNoRepository(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Open(ctx, t.TempDir()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), t.TempDir()); !errors.Is(err, ErrNotRepository) {
		t.Fatal(err)
	}
	r, ctx := fixture(t)
	write(t, r, "large.go", strings.Repeat("x", 4096))
	if _, err := runGit(ctx, r.Root, nil, 10, "status", "--porcelain=v2", "-z"); !errors.Is(err, ErrOutputLimit) {
		t.Fatal(err)
	}
	if _, err := r.ChangeIndex(ctx, status(t, r, ctx), []string{"../escape"}, true); err == nil {
		t.Fatal("invalid resource accepted")
	}
}

func commitFixture(t *testing.T, r *Repository, ctx context.Context, paths ...string) State {
	t.Helper()
	state, err := r.ChangeIndex(ctx, status(t, r, ctx), paths, true)
	if err != nil {
		t.Fatal(err)
	}
	state, err = r.Commit(ctx, state, "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	return state
}

func TestRealGitWorktreeAndIgnoredFiles(t *testing.T) {
	r, ctx := fixture(t)
	write(t, r, "main.go", "original\n")
	write(t, r, ".gitignore", "ignored/\n")
	base := commitFixture(t, r, ctx, "main.go", ".gitignore")
	worktree := filepath.Join(t.TempDir(), "linked 世界😀")
	if _, err := runGit(ctx, r.Root, nil, 65536, "worktree", "add", "-b", "linked", worktree); err != nil {
		t.Fatal(err)
	}
	linked, err := Open(ctx, worktree)
	if err != nil || linked.indexPath == r.indexPath {
		t.Fatal(linked, err)
	}
	write(t, linked, "ignored/private.go", "ignored\n")
	write(t, linked, "sub/main.go", "linked\n")
	inside, err := Open(ctx, filepath.Join(worktree, "sub"))
	if err != nil || filepath.Clean(inside.Root) != filepath.Clean(linked.Root) {
		t.Fatal(inside, err)
	}
	state := status(t, linked, ctx)
	if len(state.Entries) != 1 || state.Entries[0].Path != "sub/main.go" {
		t.Fatal(state)
	}
	changed := commitFixture(t, linked, ctx, "sub/main.go")
	if changed.Head == base.Head || status(t, r, ctx).Head != base.Head {
		t.Fatal("linked commit changed primary checkout")
	}
}

func TestRealGitSHA256Objects(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	directory := t.TempDir()
	for _, args := range [][]string{{"init", "--object-format=sha256", "-b", "main"}, {"config", "user.name", "Gocode SHA256"}, {"config", "user.email", "fixture@example.invalid"}, {"config", "commit.gpgsign", "false"}, {"config", "core.autocrlf", "false"}, {"config", "core.hooksPath", filepath.ToSlash(filepath.Join(directory, "no-hooks"))}} {
		if _, err := runGit(ctx, directory, nil, 65536, args...); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Open(ctx, directory)
	if err != nil {
		t.Fatal(err)
	}
	write(t, r, "main.go", "before\n")
	state := commitFixture(t, r, ctx, "main.go")
	if len(state.Head) != 64 {
		t.Fatal("SHA256 HEAD", state.Head)
	}
	write(t, r, "main.go", "after\n")
	state = status(t, r, ctx)
	diff, err := r.Diff(ctx, state, "main.go", false)
	if err != nil || diff.Before != "before\n" || diff.After != "after\n" {
		t.Fatal(diff, err)
	}
}

func TestRealGitConflictAndSubmoduleBoundaries(t *testing.T) {
	r, ctx := fixture(t)
	write(t, r, "main.go", "base\n")
	commitFixture(t, r, ctx, "main.go")
	if _, err := runGit(ctx, r.Root, nil, 65536, "checkout", "-b", "other"); err != nil {
		t.Fatal(err)
	}
	write(t, r, "main.go", "other\n")
	commitFixture(t, r, ctx, "main.go")
	if _, err := runGit(ctx, r.Root, nil, 65536, "checkout", "main"); err != nil {
		t.Fatal(err)
	}
	write(t, r, "main.go", "main\n")
	commitFixture(t, r, ctx, "main.go")
	if _, err := runGit(ctx, r.Root, nil, 65536, "merge", "--no-edit", "other"); err == nil {
		t.Fatal("fixture did not conflict")
	}
	state := status(t, r, ctx)
	if len(state.Entries) != 1 || !state.Entries[0].Conflict {
		t.Fatal(state)
	}
	if _, err := r.Commit(ctx, state, "unresolved"); err == nil {
		t.Fatal("committed conflict")
	}
	if _, err := r.Diff(ctx, state, "main.go", false); err == nil {
		t.Fatal("fabricated conflict diff")
	}
	if _, err := runGit(ctx, r.Root, nil, 65536, "merge", "--abort"); err != nil {
		t.Fatal(err)
	}
	child, childCtx := fixture(t)
	write(t, child, "child.go", "child\n")
	commitFixture(t, child, childCtx, "child.go")
	if _, err := runGit(ctx, r.Root, nil, 65536, "-c", "protocol.file.allow=always", "submodule", "add", "--", child.Root, "nested 世界"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Commit(ctx, status(t, r, ctx), "Add real submodule"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r.Root, "nested 世界", "child.go"), []byte("modified child\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state = status(t, r, ctx)
	if len(state.Entries) != 1 || !strings.HasPrefix(state.Entries[0].Submodule, "S") {
		t.Fatal(state)
	}
	if _, err := r.Diff(ctx, state, "nested 世界", false); err == nil || !strings.Contains(err.Error(), "submodule") {
		t.Fatal(err)
	}
}

func TestRealGitOversizeDiffLeavesFileUntouched(t *testing.T) {
	r, ctx := fixture(t)
	file := filepath.Join(r.Root, "large.go")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(1 << 30); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = r.Diff(ctx, status(t, r, ctx), "large.go", false); err == nil || !strings.Contains(err.Error(), "8 MiB") {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil || info.Size() != 1<<30 {
		t.Fatal("large diff modified source", err)
	}
}
