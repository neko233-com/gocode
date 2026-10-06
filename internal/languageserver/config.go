package languageserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const GoplsVersion = "v0.23.0"

func ManagedGopls() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	name := "gopls"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(root, "gocode", "tools", "gopls", GoplsVersion, name), nil
}
func LoadConfig(path string) ([]Config, error) {
	if path == "" {
		root, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(root, "gocode", "lsp.json")
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			command, _ := ManagedGopls()
			if _, err := os.Stat(command); err != nil {
				command, _ = exec.LookPath("gopls")
			}
			if command == "" {
				return nil, nil
			}
			return []Config{{Name: "gopls", Languages: []string{"go"}, Command: command}}, nil
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 1<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, errors.New("LSP config exceeds 1 MiB")
	}
	var configs []Config
	if err := json.Unmarshal(data, &configs); err != nil {
		return nil, err
	}
	if len(configs) > 16 {
		return nil, errors.New("at most 16 language servers can be configured")
	}
	seen := map[string]bool{}
	for _, c := range configs {
		if c.Name == "" || seen[c.Name] || c.Command == "" || len(c.Languages) == 0 {
			return nil, errors.New("invalid or duplicate LSP config")
		}
		seen[c.Name] = true
	}
	return configs, nil
}
func InstallGopls(ctx context.Context) (string, error) {
	path, err := ManagedGopls()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	command := exec.CommandContext(ctx, "go", "install", "golang.org/x/tools/gopls@"+GoplsVersion)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "GOBIN") && !strings.EqualFold(key, "GOWORK") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "GOBIN="+filepath.Dir(path), "GOWORK=off")
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return "", err
	}
	return path, nil
}
