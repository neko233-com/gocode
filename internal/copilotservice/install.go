package copilotservice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const runtimeVersion = "copilot-1.0.92-lsp-1.551.2"

var installMu sync.Mutex

func ManagedRuntimeRoot() (string, error) {
	config, err := os.UserConfigDir()
	return filepath.Join(config, "gocode", "tools", runtimeVersion), err
}

// npm's JavaScript entry point avoids a shell and its quoting/injection rules.
func npmCLI() (string, error) {
	npm, err := exec.LookPath("npm")
	if err != nil {
		return "", errors.New("Copilot installation requires Node.js 24 and npm on PATH")
	}
	resolved, err := filepath.EvalSymlinks(npm)
	if err == nil && strings.HasSuffix(resolved, ".js") {
		return resolved, nil
	}
	for _, path := range []string{
		filepath.Join(filepath.Dir(npm), "node_modules", "npm", "bin", "npm-cli.js"),
		filepath.Join(filepath.Dir(npm), "..", "lib", "node_modules", "npm", "bin", "npm-cli.js"),
	} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return filepath.Abs(path)
		}
	}
	return "", errors.New("npm's JavaScript entry point was not found; install a standard Node.js distribution")
}

type runtimeLog struct{ bytes.Buffer }

func (w *runtimeLog) Write(p []byte) (int, error) {
	n := len(p)
	if w.Len() < 32768 {
		w.Buffer.Write(p[:min(len(p), 32768-w.Len())])
	}
	return n, nil
}

// InstallRuntime installs the application-embedded npm lockfile into a fresh,
// per-user version directory. npm verifies package integrity; no global packages
// or lifecycle scripts are changed. Publication follows real runtime validation.
func InstallRuntime(ctx context.Context, packageJSON, lockJSON []byte) (string, error) {
	installMu.Lock()
	defer installMu.Unlock()
	if !json.Valid(packageJSON) || !json.Valid(lockJSON) {
		return "", errors.New("invalid embedded Copilot runtime package metadata")
	}
	root, err := ManagedRuntimeRoot()
	if err != nil {
		return "", err
	}
	if existing, err := RuntimeRoot(root); err == nil {
		lock, readErr := os.ReadFile(filepath.Join(root, "package-lock.json"))
		if readErr == nil && bytes.Equal(lock, lockJSON) {
			if _, err := CLIPath(existing); err == nil {
				return existing, nil
			}
		}
		return "", errors.New("managed Copilot runtime differs from the pinned lockfile; preserve it and choose an explicit runtime")
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("managed runtime directory already exists but is incomplete; no existing files were replaced")
	}
	cli, err := npmCLI()
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(root)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(parent, ".copilot-install-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage) // exclusively this call's freshly created directory
	for name, data := range map[string][]byte{"package.json": packageJSON, "package-lock.json": lockJSON} {
		if err := os.WriteFile(filepath.Join(stage, name), data, 0600); err != nil {
			return "", err
		}
	}
	command := exec.CommandContext(ctx, "node", cli, "ci", "--prefix", stage, "--ignore-scripts", "--include=optional", "--no-audit", "--no-fund")
	command.Dir = stage
	command.WaitDelay = 2 * time.Second
	hideProcess(command)
	var output runtimeLog
	command.Stdout, command.Stderr = &output, &output
	if err := command.Run(); err != nil {
		return "", fmt.Errorf("pinned Copilot runtime installation failed: %w: %s", err, output.String())
	}
	if _, err := RuntimeRoot(stage); err != nil {
		return "", err
	}
	if _, err := CLIPath(stage); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.Rename(stage, root); err != nil {
		return "", err
	}
	return root, nil
}
