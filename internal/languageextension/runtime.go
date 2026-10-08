package languageextension

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/gocode/internal/nativeguard"
)

//go:embed runtime/package.json
var packageJSON []byte

//go:embed runtime/package-lock.json
var lockJSON []byte

var runtimeMu sync.Mutex

const MaxInstallTime = 120 * time.Second

type Runtime struct {
	Kind, Version, Command string
	Arguments              []string
	Files                  []File
}

func runtimePath(tools, kind string) string {
	version := TSVersion
	if kind == "go" {
		version = languageserver.GoplsVersion
	}
	return filepath.Join(tools, kind, version)
}

func toolCommand(ctx context.Context, args []string, directory string, environment []string) (nativeguard.Result, error) {
	result, err := nativeguard.Run(ctx, nativeguard.Options{Command: args, Directory: directory, Environment: environment, Timeout: 120 * time.Second})
	if err != nil {
		return result, fmt.Errorf("language dependency command failed: %w: %s", err, result.Stderr[:min(len(result.Stderr), 8192)])
	}
	return result, nil
}

func nodePath(ctx context.Context) (string, error) {
	node, err := exec.LookPath("node")
	if err != nil {
		return "", errors.New("TypeScript support requires Node.js >=22.22.2 on PATH")
	}
	result, err := toolCommand(ctx, []string{node, "--version"}, "", os.Environ())
	if err != nil {
		return "", err
	}
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(string(result.Stdout)), "v"), ".")
	if len(parts) != 3 {
		return "", errors.New("unrecognized Node.js version")
	}
	var v [3]int
	for i := range parts {
		v[i], err = strconv.Atoi(parts[i])
		if err != nil || v[i] < 0 {
			return "", errors.New("unrecognized Node.js version")
		}
	}
	if v[0] < 22 || v[0] == 22 && (v[1] < 22 || v[1] == 22 && v[2] < 2) {
		return "", errors.New("TypeScript LSP 6.0.1 requires Node.js >=22.22.2")
	}
	return filepath.Abs(node)
}

func npmPath() (string, error) {
	npm, err := exec.LookPath("npm")
	if err != nil {
		return "", errors.New("TypeScript support requires npm with a JavaScript CLI entry point")
	}
	resolved, err := filepath.EvalSymlinks(npm)
	if err == nil && strings.HasSuffix(resolved, ".js") {
		return resolved, nil
	}
	for _, path := range []string{filepath.Join(filepath.Dir(npm), "node_modules", "npm", "bin", "npm-cli.js"), filepath.Join(filepath.Dir(npm), "..", "lib", "node_modules", "npm", "bin", "npm-cli.js")} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return filepath.Abs(path)
		}
	}
	return "", errors.New("npm JavaScript CLI entry point missing")
}

func verifyRuntime(tools, kind string) (Runtime, error) {
	var r Runtime
	if _, err := Package(kind); err != nil {
		return r, err
	}
	root := runtimePath(tools, kind)
	if err := readJSON(root, "runtime.json", 1<<20, &r); err != nil {
		return r, fmt.Errorf("native %s dependency is not installed: %w", kind, err)
	}
	version := TSVersion
	name := "node_modules/typescript-language-server/lib/cli.mjs"
	if kind == "go" {
		version = languageserver.GoplsVersion
		name = "gopls"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
	}
	if r.Kind != kind || r.Version != version || len(r.Files) == 0 || len(r.Files) > 4096 {
		return Runtime{}, errors.New("invalid native language dependency receipt")
	}
	seen := map[string]bool{}
	var total int64
	for _, file := range r.Files {
		if file.Bytes < 0 || file.Bytes > 64<<20 || seen[file.Path] {
			return Runtime{}, errors.New("invalid native language dependency file receipt")
		}
		seen[file.Path] = true
		total += file.Bytes
		if total > 64<<20 {
			return Runtime{}, errors.New("native language dependency exceeds limit")
		}
		actual, err := hashFile(root, file.Path, 64<<20)
		if err != nil || actual != file {
			return Runtime{}, errors.Join(err, errors.New("native language dependency changed"))
		}
	}
	if !seen[filepath.FromSlash(name)] {
		return Runtime{}, errors.New("native language dependency has no executable entry point")
	}
	if kind == "go" {
		r.Command, r.Arguments = filepath.Join(root, name), nil
	} else {
		// The lockfile is the distribution contract; no global npm packages or
		// workspace TypeScript fallback can silently replace this entry point.
		b, err := os.ReadFile(filepath.Join(root, "package-lock.json"))
		if err != nil || !bytes.Equal(b, lockJSON) {
			return Runtime{}, errors.Join(err, errors.New("TypeScript dependency lockfile changed"))
		}
		node, err := exec.LookPath("node")
		if err != nil {
			return Runtime{}, err
		}
		r.Command, r.Arguments = node, []string{filepath.Join(root, filepath.FromSlash(name)), "--stdio"}
	}
	return r, nil
}

// EnsureRuntime installs into a fresh private stage and publishes only after a
// real version command and complete-file hashes succeed. Reinstall does not
// download or modify a verified existing version.
func EnsureRuntime(ctx context.Context, tools, kind string) (Runtime, error) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()
	if err := ctx.Err(); err != nil {
		return Runtime{}, err
	}
	if _, err := Package(kind); err != nil {
		return Runtime{}, err
	}
	if r, err := verifyRuntime(tools, kind); err == nil {
		return r, nil
	}
	root := runtimePath(tools, kind)
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		return Runtime{}, errors.New("native language dependency already exists but differs; no existing files replaced")
	}
	if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return Runtime{}, err
	}
	stage, err := os.MkdirTemp(filepath.Dir(root), ".language-runtime-")
	if err != nil {
		return Runtime{}, err
	}
	defer os.RemoveAll(stage) // only this call's freshly created, owned stage
	r := Runtime{Kind: kind, Version: TSVersion}
	if kind == "go" {
		r.Version = languageserver.GoplsVersion
		goTool, err := exec.LookPath("go")
		if err != nil {
			return r, err
		}
		environment := make([]string, 0, len(os.Environ())+2)
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			if !strings.EqualFold(key, "GOBIN") && !strings.EqualFold(key, "GOWORK") {
				environment = append(environment, entry)
			}
		}
		environment = append(environment, "GOBIN="+stage, "GOWORK=off")
		if _, err = toolCommand(ctx, []string{goTool, "install", "golang.org/x/tools/gopls@" + languageserver.GoplsVersion}, stage, environment); err != nil {
			return r, err
		}
		name := "gopls"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		result, err := toolCommand(ctx, []string{filepath.Join(stage, name), "version"}, stage, os.Environ())
		if err != nil || !strings.Contains(string(result.Stdout), "golang.org/x/tools/gopls "+languageserver.GoplsVersion) {
			return r, errors.Join(err, errors.New("installed gopls version mismatch"))
		}
	} else {
		node, err := nodePath(ctx)
		if err != nil {
			return r, err
		}
		npm, err := npmPath()
		if err != nil {
			return r, err
		}
		if err = errors.Join(os.WriteFile(filepath.Join(stage, "package.json"), packageJSON, 0600), os.WriteFile(filepath.Join(stage, "package-lock.json"), lockJSON, 0600)); err != nil {
			return r, err
		}
		if _, err = toolCommand(ctx, []string{node, npm, "ci", "--prefix", stage, "--ignore-scripts", "--no-audit", "--no-fund"}, stage, os.Environ()); err != nil {
			return r, err
		}
		result, err := toolCommand(ctx, []string{node, filepath.Join(stage, "node_modules", "typescript-language-server", "lib", "cli.mjs"), "--version"}, stage, os.Environ())
		if err != nil || strings.TrimSpace(string(result.Stdout)) != TSVersion {
			return r, errors.Join(err, errors.New("installed TypeScript LSP version mismatch"))
		}
	}
	var total int64
	err = filepath.WalkDir(stage, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() && path == filepath.Join(stage, "node_modules", ".bin") {
			return filepath.SkipDir
		} // npm's optional shell links are never executed
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("language dependency contains a linked file")
		}
		if entry.IsDir() {
			return nil
		}
		name, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		file, err := hashFile(stage, name, 64<<20)
		if err != nil {
			return err
		}
		total += file.Bytes
		if total > 64<<20 || len(r.Files) >= 4096 {
			return errors.New("language dependency exceeds bounded inventory")
		}
		r.Files = append(r.Files, file)
		return nil
	})
	if err != nil {
		return r, err
	}
	if err = writeJSON(filepath.Join(stage, "runtime.json"), r); err != nil {
		return r, err
	}
	if err = ctx.Err(); err != nil {
		return r, err
	}
	if err = os.Rename(stage, root); err != nil {
		return r, err
	}
	return verifyRuntime(tools, kind)
}
