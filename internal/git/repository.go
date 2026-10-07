package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

var ErrNotRepository = errors.New("workspace is not a Git repository")
var ErrStale = errors.New("repository or index changed; refresh Source Control before retrying")

type Repository struct{ Root, indexPath string }

func Init(ctx context.Context, directory string) (*Repository, error) {
	if _, err := runGit(ctx, directory, nil, 65536, "init"); err != nil {
		return nil, err
	}
	return Open(ctx, directory)
}

// SetIdentity updates only this repository, after an explicit user choice.
func (r *Repository) SetIdentity(ctx context.Context, name, email string) error {
	if !utf8.ValidString(name) || !utf8.ValidString(email) || strings.TrimSpace(name) == "" || strings.TrimSpace(email) == "" || len(name) > 256 || len(email) > 256 || strings.ContainsAny(name+email, "\x00\r\n") {
		return errors.New("invalid repository identity")
	}
	for _, pair := range []struct{ key, value string }{{"user.name", name}, {"user.email", email}} {
		if _, err := runGit(ctx, r.Root, nil, 65536, "config", "--local", pair.key, pair.value); err != nil {
			return err
		}
	}
	return nil
}

func Open(ctx context.Context, directory string) (*Repository, error) {
	root, err := runGit(ctx, directory, nil, 8192, "rev-parse", "--show-toplevel")
	if err != nil {
		var command *commandError
		if errors.As(err, &command) && command.Code == 128 {
			return nil, ErrNotRepository
		}
		return nil, err
	}
	path := strings.TrimSuffix(strings.TrimSuffix(string(root), "\n"), "\r")
	if !utf8.ValidString(path) || !filepath.IsAbs(path) {
		return nil, errors.New("invalid repository root")
	}
	index, err := runGit(ctx, path, nil, 8192, "rev-parse", "--path-format=absolute", "--git-path", "index")
	if err != nil {
		return nil, err
	}
	indexPath := strings.TrimSuffix(strings.TrimSuffix(string(index), "\n"), "\r")
	if !filepath.IsAbs(indexPath) {
		return nil, errors.New("invalid repository index")
	}
	return &Repository{Root: filepath.Clean(path), indexPath: indexPath}, nil
}
func (r *Repository) indexToken(ctx context.Context) ([32]byte, error) {
	file, err := os.Open(r.indexPath)
	if os.IsNotExist(err) {
		return sha256.Sum256([]byte("missing Git index")), nil
	}
	if err != nil {
		return [32]byte{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return [32]byte{}, err
	}
	if info.Size() > 64<<20 {
		return [32]byte{}, errors.New("Git index exceeds 64 MiB")
	}
	hash := sha256.New()
	buffer := make([]byte, 128<<10)
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return [32]byte{}, err
		}
		n, err := file.Read(buffer)
		total += n
		if total > 64<<20 {
			return [32]byte{}, ErrOutputLimit
		}
		_, _ = hash.Write(buffer[:n])
		if err == io.EOF {
			break
		}
		if err != nil {
			return [32]byte{}, err
		}
	}
	var token [32]byte
	copy(token[:], hash.Sum(nil))
	return token, nil
}
func (r *Repository) Status(ctx context.Context) (State, error) {
	before, err := r.indexToken(ctx)
	if err != nil {
		return State{}, err
	}
	raw, err := runGit(ctx, r.Root, nil, 4<<20, "status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return State{}, err
	}
	state, err := parseStatus(raw)
	if err != nil {
		return State{}, err
	}
	after, err := r.indexToken(ctx)
	if err != nil {
		return State{}, err
	}
	if before != after {
		return State{}, ErrStale
	}
	state.Root, state.IndexToken = r.Root, after
	return state, nil
}
func (r *Repository) check(ctx context.Context, expected State) (State, error) {
	current, err := r.Status(ctx)
	if err != nil {
		return State{}, err
	}
	if expected.Root != r.Root || expected.Head != current.Head || expected.IndexToken != current.IndexToken {
		return current, ErrStale
	}
	return current, nil
}
func (r *Repository) ChangeIndex(ctx context.Context, expected State, paths []string, stage bool) (State, error) {
	current, err := r.check(ctx, expected)
	if err != nil {
		return State{}, err
	}
	if len(paths) == 0 || len(paths) > MaxEntries {
		return State{}, errors.New("select changed files")
	}
	known := map[string]Entry{}
	for _, entry := range current.Entries {
		known[entry.Path] = entry
	}
	selected := map[string]bool{}
	var input bytes.Buffer
	for _, path := range paths {
		entry, ok := known[path]
		if !ok || !validPath(path) || (stage && !entry.Changed()) || (!stage && !entry.Staged()) {
			return State{}, errors.New("selected Git resource changed")
		}
		for _, value := range []string{entry.Path, entry.OriginalPath} {
			if value != "" && !selected[value] {
				if input.Len()+len(value)+1 > 1<<20 {
					return State{}, errors.New("Git path selection exceeds 1 MiB")
				}
				selected[value] = true
				input.WriteString(value)
				input.WriteByte(0)
			}
		}
	}
	if input.Len() > 1<<20 {
		return State{}, errors.New("Git path selection exceeds 1 MiB")
	}
	args := []string{"add", "-A", "--pathspec-from-file=-", "--pathspec-file-nul"}
	if !stage {
		args = []string{"restore", "--staged", "--source=HEAD", "--pathspec-from-file=-", "--pathspec-file-nul"}
		if current.Head == "" {
			args = []string{"rm", "--cached", "--force", "--ignore-unmatch", "-r", "--pathspec-from-file=-", "--pathspec-file-nul"}
		}
	}
	if _, err := runGit(ctx, r.Root, input.Bytes(), 64<<10, args...); err != nil {
		return State{}, err
	}
	return r.Status(ctx)
}
func (r *Repository) Commit(ctx context.Context, expected State, message string) (State, error) {
	if !utf8.ValidString(message) || strings.ContainsRune(message, 0) || len(message) > 64<<10 || strings.TrimSpace(message) == "" {
		return State{}, errors.New("enter a commit message (maximum 64 KiB)")
	}
	current, err := r.check(ctx, expected)
	if err != nil {
		return State{}, err
	}
	staged := false
	for _, entry := range current.Entries {
		if entry.Conflict {
			return State{}, errors.New("resolve merge conflicts before committing")
		}
		staged = staged || entry.Staged()
	}
	if !staged {
		return State{}, errors.New("stage changes before committing")
	}
	if _, err := runGit(ctx, r.Root, []byte(message), 64<<10, "commit", "--file=-"); err != nil {
		return State{}, err
	}
	return r.Status(ctx)
}

type Diff struct {
	Path               string
	Staged, Binary     bool
	Before, After      string
	Patch              string
	OldBytes, NewBytes int
}

func (r *Repository) blob(ctx context.Context, oid string) ([]byte, error) {
	if emptyOID(oid) {
		return nil, nil
	}
	if !validOID(oid) {
		return nil, errors.New("invalid blob")
	}
	return runGit(ctx, r.Root, nil, 8<<20, "cat-file", "blob", oid)
}
func (r *Repository) Diff(ctx context.Context, expected State, path string, staged bool) (Diff, error) {
	state, err := r.check(ctx, expected)
	if err != nil {
		return Diff{}, err
	}
	var entry *Entry
	for _, item := range state.Entries {
		if item.Path == path {
			copy := item
			entry = &copy
			break
		}
	}
	if entry == nil || !validPath(path) {
		return Diff{}, errors.New("Git resource no longer exists")
	}
	if entry.Conflict {
		return Diff{}, errors.New("unmerged file requires a merge view")
	}
	if strings.HasPrefix(entry.Submodule, "S") {
		return Diff{}, errors.New("submodule changes require opening the submodule repository")
	}
	if staged && !entry.Staged() || !staged && !entry.Changed() {
		return Diff{}, errors.New("Git resource has no selected changes")
	}
	left := entry.IndexBlob
	if staged {
		left = entry.HeadBlob
	}
	before, err := r.blob(ctx, left)
	if err != nil {
		return Diff{}, err
	}
	var after []byte
	if staged {
		after, err = r.blob(ctx, entry.IndexBlob)
	} else if entry.Worktree != 'D' {
		full := filepath.Join(r.Root, filepath.FromSlash(path))
		info, statErr := os.Lstat(full)
		if statErr != nil {
			return Diff{}, statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, e := os.Readlink(full)
			after, err = []byte(target), e
		} else if !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return Diff{}, errors.New("diff supports regular files up to 8 MiB; open the file for bounded large-file browsing")
		} else {
			file, e := os.Open(full)
			if e != nil {
				return Diff{}, e
			}
			after, err = io.ReadAll(io.LimitReader(file, (8<<20)+1))
			file.Close()
			if len(after) > 8<<20 {
				return Diff{}, ErrOutputLimit
			}
		}
	}
	if err != nil {
		return Diff{}, err
	}
	// Diff these exact bounded snapshots, not a second read of a changing file.
	temporary, err := os.MkdirTemp("", "gocode-git-diff-")
	if err != nil {
		return Diff{}, err
	}
	defer os.RemoveAll(temporary)
	oldFile, newFile := filepath.Join(temporary, "before"), filepath.Join(temporary, "after")
	if err := os.WriteFile(oldFile, before, 0600); err != nil {
		return Diff{}, err
	}
	if err := os.WriteFile(newFile, after, 0600); err != nil {
		return Diff{}, err
	}
	args := []string{"diff", "--no-ext-diff", "--no-textconv", "--no-color", "--unified=3", "--no-renames", "--no-index", "--", oldFile, newFile}
	patch, err := runGit(ctx, r.Root, nil, 8<<20, args...)
	if err != nil {
		var exit *commandError
		if !(errors.As(err, &exit) && exit.Code == 1) {
			return Diff{}, err
		}
	}
	if _, err := r.check(ctx, expected); err != nil {
		return Diff{}, err
	}
	result := Diff{Path: path, Staged: staged, OldBytes: len(before), NewBytes: len(after), Binary: bytes.Contains(before, []byte{0}) || bytes.Contains(after, []byte{0}) || !utf8.Valid(before) || !utf8.Valid(after)}
	if !result.Binary {
		result.Before, result.After, result.Patch = string(before), string(after), string(patch)
	}
	return result, nil
}
func (r *Repository) String() string { return fmt.Sprintf("Git repository %s", r.Root) }
