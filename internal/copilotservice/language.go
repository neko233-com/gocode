// Package copilotservice connects the native editor to official GitHub runtimes.
package copilotservice

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/lsp"
)

type Command struct {
	Command   string `json:"command"`
	Arguments []any  `json:"arguments,omitempty"`
	Title     string `json:"title,omitempty"`
}
type InlineItem struct {
	InsertText string        `json:"insertText"`
	Range      *editor.Range `json:"range,omitempty"`
	Command    *Command      `json:"command,omitempty"`
}
type Status struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Busy    bool   `json:"busy"`
}
type SignIn struct {
	UserCode string  `json:"userCode"`
	Command  Command `json:"command"`
}
type Language struct {
	RPC      *lsp.Client
	mu       sync.Mutex
	versions map[string]int
}

func FileURI(path string) string {
	path, _ = filepath.Abs(path)
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return (&url.URL{Scheme: "file", Path: path}).String()
}
func LanguageID(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".js", ".cjs", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp":
		return "cpp"
	default:
		return "plaintext"
	}
}

// RuntimeRoot finds a deliberately installed sidecar, without global npm mutation.
func RuntimeRoot(explicit string) (string, error) {
	if explicit != "" {
		root, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		if info, err := os.Stat(filepath.Join(root, "node_modules", "@github", "copilot-language-server", "dist", "language-server.js")); err != nil || !info.Mode().IsRegular() {
			return "", errors.New("specified Copilot runtime does not contain the installed official language server")
		}
		return root, nil
	}
	exe, _ := os.Executable()
	for _, root := range []string{filepath.Join(filepath.Dir(exe), "copilot-runtime"), filepath.Join("tools", "copilot-runtime")} {
		if _, err := os.Stat(filepath.Join(root, "node_modules", "@github", "copilot-language-server", "dist", "language-server.js")); err == nil {
			return filepath.Abs(root)
		}
	}
	return "", errors.New("Copilot runtime missing: run npm ci in tools/copilot-runtime or set -copilot-runtime")
}

func NewLanguage(ctx context.Context, root, workspace string) (*Language, error) {
	client, err := lsp.Start(ctx, lsp.Command{Executable: "node", Arguments: []string{filepath.Join(root, "node_modules", "@github", "copilot-language-server", "dist", "language-server.js"), "--stdio"}, Directory: workspace})
	if err != nil {
		return nil, err
	}
	l := &Language{RPC: client, versions: map[string]int{}}
	initializeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err = l.Initialize(initializeCtx, workspace); err != nil {
		client.Close()
		return nil, err
	}
	return l, nil
}
func (l *Language) Initialize(ctx context.Context, workspace string) error {
	l.RPC.Register("workspace/configuration", func(_ context.Context, raw json.RawMessage) (any, error) {
		var params struct {
			Items []any `json:"items"`
		}
		if err := json.Unmarshal(raw, &params); err != nil {
			return nil, err
		}
		result := make([]any, len(params.Items))
		for i := range result {
			result[i] = map[string]any{}
		}
		return result, nil
	})
	for _, method := range []string{"client/registerCapability", "client/unregisterCapability", "window/workDoneProgress/create"} {
		l.RPC.Register(method, func(context.Context, json.RawMessage) (any, error) { return nil, nil })
	}
	// Device-flow URLs are displayed by the editor. The server can use its native
	// browser opener; we do not claim the client opened a URL when it has not.
	l.RPC.Register("window/showDocument", func(context.Context, json.RawMessage) (any, error) { return map[string]bool{"success": false}, nil })
	var result json.RawMessage
	if err := l.RPC.Call(ctx, "initialize", map[string]any{
		"processId": os.Getpid(), "rootUri": FileURI(workspace),
		"workspaceFolders":      []any{map[string]any{"uri": FileURI(workspace), "name": filepath.Base(workspace)}},
		"capabilities":          map[string]any{"workspace": map[string]any{"workspaceFolders": true}, "general": map[string]any{"positionEncodings": []string{"utf-16"}}},
		"initializationOptions": map[string]any{"editorInfo": map[string]string{"name": "gocode", "version": "0.4.0"}, "editorPluginInfo": map[string]string{"name": "gocode Copilot", "version": "0.1.0"}},
	}, &result); err != nil {
		return err
	}
	if err := l.RPC.Notify(ctx, "initialized", map[string]any{}); err != nil {
		return err
	}
	return l.RPC.Notify(ctx, "workspace/didChangeConfiguration", map[string]any{"settings": map[string]any{"telemetry": map[string]string{"telemetryLevel": "off"}}})
}
func (l *Language) Sync(ctx context.Context, path string, snapshot editor.Snapshot, changes *editor.ChangeEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sync(ctx, path, snapshot, changes)
}
func (l *Language) sync(ctx context.Context, path string, s editor.Snapshot, changes *editor.ChangeEvent) error {
	uri := FileURI(path)
	version, open := l.versions[uri]
	if !open {
		if err := l.RPC.Notify(ctx, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": LanguageID(path), "version": s.Version, "text": s.Text()}}); err != nil {
			return err
		}
	} else if s.Version > version {
		var content any = []any{map[string]any{"text": s.Text()}}
		if changes != nil && changes.Version == s.Version && version+1 == s.Version {
			content = changes.Changes
		}
		if err := l.RPC.Notify(ctx, "textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": s.Version}, "contentChanges": content}); err != nil {
			return err
		}
	} else if s.Version < version {
		return errors.New("stale document snapshot")
	}
	l.versions[uri] = s.Version
	return nil
}
func (l *Language) Focus(ctx context.Context, path string) error {
	params := map[string]any{}
	if path != "" {
		params["textDocument"] = map[string]string{"uri": FileURI(path)}
	}
	return l.RPC.Notify(ctx, "textDocument/didFocus", params)
}
func (l *Language) CloseDocument(ctx context.Context, path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	uri := FileURI(path)
	if _, ok := l.versions[uri]; !ok {
		return nil
	}
	if err := l.RPC.Notify(ctx, "textDocument/didClose", map[string]any{"textDocument": map[string]string{"uri": uri}}); err != nil {
		return err
	}
	delete(l.versions, uri)
	return nil
}
func (l *Language) Inline(ctx context.Context, path string, s editor.Snapshot, position editor.Position) ([]InlineItem, error) {
	if err := l.Sync(ctx, path, s, nil); err != nil {
		return nil, err
	}
	var result struct {
		Items []InlineItem `json:"items"`
	}
	err := l.RPC.Call(ctx, "textDocument/inlineCompletion", map[string]any{"textDocument": map[string]any{"uri": FileURI(path), "version": s.Version}, "position": position, "context": map[string]int{"triggerKind": 1}, "formattingOptions": map[string]any{"tabSize": 4, "insertSpaces": true}}, &result)
	return result.Items, err
}
func (l *Language) Shown(ctx context.Context, item InlineItem) error {
	return l.RPC.Notify(ctx, "textDocument/didShowCompletion", map[string]any{"item": item})
}
func (l *Language) Accepted(ctx context.Context, item InlineItem) error {
	if item.Command == nil {
		return nil
	}
	return l.RPC.Call(ctx, "workspace/executeCommand", item.Command, nil)
}
func (l *Language) SignIn(ctx context.Context) (SignIn, error) {
	var result SignIn
	err := l.RPC.Call(ctx, "signIn", map[string]any{}, &result)
	return result, err
}
func (l *Language) FinishSignIn(ctx context.Context, command Command) error {
	return l.RPC.Call(ctx, "workspace/executeCommand", command, nil)
}

func CLIPath(root string) (string, error) {
	if override := os.Getenv("GOCODE_COPILOT_CLI"); override != "" {
		return override, nil
	}
	platform, arch := runtime.GOOS, runtime.GOARCH
	if platform == "windows" {
		platform = "win32"
	}
	if arch == "amd64" {
		arch = "x64"
	}
	name := "copilot"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	file := filepath.Join(root, "node_modules", "@github", "copilot-"+platform+"-"+arch, name)
	if _, err := os.Stat(file); err != nil {
		return "", errors.New("official Copilot CLI platform binary missing; reinstall tools/copilot-runtime with optional dependencies enabled")
	}
	return file, nil
}
