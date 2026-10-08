// Package languageserver manages standard LSP servers without coupling their
// processes or protocol state to the UI. All document coordinates are UTF-16.
package languageserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gocode/internal/copilotservice"
	"github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/lsp"
)

const MaxDocumentBytes = 2 << 20 // JSON escaping can expand each source byte sixfold.

type Config struct {
	ClientVersion         string         `json:"-"`
	AutomaticGoFallback   bool           `json:"-"` // Set only by the automatic startup loader, never by user JSON.
	Name                  string         `json:"name"`
	Languages             []string       `json:"languages"`
	Command               string         `json:"command"`
	Arguments             []string       `json:"arguments,omitempty"`
	Settings              map[string]any `json:"settings,omitempty"`
	InitializationOptions any            `json:"initializationOptions,omitempty"`
}
type Capabilities struct {
	TextDocumentSync           json.RawMessage `json:"textDocumentSync"`
	PositionEncoding           string          `json:"positionEncoding"`
	CompletionProvider         json.RawMessage `json:"completionProvider"`
	HoverProvider              json.RawMessage `json:"hoverProvider"`
	DefinitionProvider         json.RawMessage `json:"definitionProvider"`
	DocumentFormattingProvider json.RawMessage `json:"documentFormattingProvider"`
}
type Client struct {
	RPC             *lsp.Client
	process         *serverTransport
	Config          Config
	Capabilities    Capabilities
	mu              sync.Mutex
	versions        map[string]int
	syncKind        int
	openClose, save bool
	saveText        bool
}
type TextEdit struct {
	Range   editor.Range `json:"range"`
	NewText string       `json:"newText"`
}
type Completion struct {
	Label               json.RawMessage `json:"label"`
	InsertText          string          `json:"insertText"`
	InsertTextFormat    int             `json:"insertTextFormat"`
	TextEdit            json.RawMessage `json:"textEdit"`
	AdditionalTextEdits []TextEdit      `json:"additionalTextEdits"`
}
type Location struct {
	URI   string       `json:"uri"`
	Range editor.Range `json:"range"`
}
type PublishDiagnostics struct {
	URI         string       `json:"uri"`
	Version     *int         `json:"version"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
type Diagnostic struct {
	Range    editor.Range `json:"range"`
	Message  string       `json:"message"`
	Severity int          `json:"severity"`
}

func (c Config) Supports(path string) bool {
	language := copilotservice.LanguageID(path)
	for _, l := range c.Languages {
		if l == language {
			return true
		}
	}
	return false
}
func Start(ctx context.Context, workspace string, config Config) (*Client, error) {
	if config.Command == "" || config.Name == "" || len(config.Languages) == 0 {
		return nil, errors.New("LSP config requires name, command and languages")
	}
	rpc, owner, err := startOwned(ctx, lsp.Command{Executable: config.Command, Arguments: config.Arguments, Directory: workspace})
	if err != nil {
		return nil, err
	}
	c, err := Initialize(ctx, rpc, workspace, config)
	if err != nil {
		_ = rpc.Close()
		return nil, errors.Join(err, owner.Close())
	}
	c.process = owner
	return c, nil
}

func (c *Client) ProcessID() int {
	if c.process == nil {
		return 0
	}
	return c.process.child.PID()
}

// ProcessIDs observes this owned process tree on platforms with a native job
// query. A platform without that query returns an error rather than a root-only
// list that could be mistaken for complete descendant evidence. Call off UI.
func (c *Client) ProcessIDs() ([]int, error) {
	if c.process == nil {
		return nil, errors.New("language client has no owned process transport")
	}
	if observer, ok := c.process.child.(interface{ ProcessIDs() ([]int, error) }); ok {
		return observer.ProcessIDs()
	}
	return nil, errors.New("owned language process-tree observation is unavailable on this platform")
}
func (c *Client) ProcessClosed() bool {
	if c.process == nil {
		return false
	}
	select {
	case <-c.process.closed:
		return c.process.closeErr == nil
	default:
		return false
	}
}

// Close reports both protocol shutdown and the owned process/worker join.
// The transport's shared acknowledgement prevents repeated calls from starting
// another three-second wait after a timed-out close.
func (c *Client) Close() error {
	err := c.RPC.Close()
	if c.process != nil {
		err = errors.Join(err, c.process.Close())
	}
	return err
}
func Initialize(ctx context.Context, rpc *lsp.Client, workspace string, config Config) (*Client, error) {
	c := &Client{RPC: rpc, Config: config, versions: map[string]int{}}
	rpc.Register("workspace/configuration", func(_ context.Context, raw json.RawMessage) (any, error) {
		var p struct {
			Items []struct {
				Section string `json:"section"`
			} `json:"items"`
		}
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, err
		}
		values := make([]any, len(p.Items))
		for i, item := range p.Items {
			values[i] = setting(config.Settings, item.Section)
		}
		return values, nil
	})
	for _, method := range []string{"client/registerCapability", "client/unregisterCapability", "window/workDoneProgress/create"} {
		rpc.Register(method, func(context.Context, json.RawMessage) (any, error) { return nil, nil })
	}
	rpc.Register("workspace/applyEdit", func(context.Context, json.RawMessage) (any, error) {
		return map[string]any{"applied": false, "failureReason": "this operation is not supported by the current LSP client"}, nil
	})
	rpc.Register("window/showMessageRequest", func(context.Context, json.RawMessage) (any, error) { return nil, nil })
	var result struct {
		Capabilities Capabilities `json:"capabilities"`
	}
	initCtx, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	err := rpc.Call(initCtx, "initialize", map[string]any{
		"processId": os.Getpid(), "clientInfo": map[string]string{"name": "gocode", "version": config.ClientVersion}, "rootUri": copilotservice.FileURI(workspace),
		"workspaceFolders": []any{map[string]string{"uri": copilotservice.FileURI(workspace), "name": filepath.Base(workspace)}},
		"capabilities": map[string]any{
			"general":   map[string]any{"positionEncodings": []string{"utf-16"}},
			"workspace": map[string]any{"configuration": true, "workspaceFolders": true, "applyEdit": false},
			"textDocument": map[string]any{
				"synchronization":    map[string]bool{"didSave": true},
				"completion":         map[string]any{"completionItem": map[string]any{"snippetSupport": false, "insertReplaceSupport": true}},
				"hover":              map[string]any{"contentFormat": []string{"plaintext"}},
				"publishDiagnostics": map[string]bool{"versionSupport": true},
				"definition":         map[string]bool{"linkSupport": true},
			},
		}, "initializationOptions": config.InitializationOptions,
	}, &result)
	if err != nil {
		return nil, err
	}
	c.Capabilities = result.Capabilities
	if encoding := result.Capabilities.PositionEncoding; encoding != "" && encoding != "utf-16" {
		return nil, fmt.Errorf("unsupported server position encoding %q", encoding)
	}
	var kind int
	if json.Unmarshal(result.Capabilities.TextDocumentSync, &kind) == nil {
		c.syncKind = kind
		c.openClose = kind != 0
	} else {
		var options struct {
			OpenClose bool            `json:"openClose"`
			Change    int             `json:"change"`
			Save      json.RawMessage `json:"save"`
		}
		if len(result.Capabilities.TextDocumentSync) > 0 && json.Unmarshal(result.Capabilities.TextDocumentSync, &options) != nil {
			return nil, errors.New("invalid LSP textDocumentSync capability")
		}
		c.syncKind = options.Change
		c.openClose = options.OpenClose
		c.save = supported(options.Save)
		var saveOptions struct {
			IncludeText bool `json:"includeText"`
		}
		if json.Unmarshal(options.Save, &saveOptions) == nil {
			c.saveText = saveOptions.IncludeText
		}
	}
	if c.syncKind < 0 || c.syncKind > 2 {
		return nil, errors.New("unsupported LSP synchronization kind")
	}
	if err := rpc.Notify(ctx, "initialized", map[string]any{}); err != nil {
		return nil, err
	}
	if err := rpc.Notify(ctx, "workspace/didChangeConfiguration", map[string]any{"settings": config.Settings}); err != nil {
		return nil, err
	}
	return c, nil
}
func setting(settings map[string]any, section string) any {
	if section == "" {
		return settings
	}
	var value any = settings
	for _, part := range strings.Split(section, ".") {
		m, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		value = m[part]
	}
	return value
}
func supported(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return s != "" && s != "null" && s != "false"
}
func (c *Client) Supports(method string) bool {
	switch method {
	case "textDocument/completion":
		return supported(c.Capabilities.CompletionProvider)
	case "textDocument/hover":
		return supported(c.Capabilities.HoverProvider)
	case "textDocument/definition":
		return supported(c.Capabilities.DefinitionProvider)
	case "textDocument/formatting":
		return supported(c.Capabilities.DocumentFormattingProvider)
	}
	return false
}

func (c *Client) Sync(ctx context.Context, path string, s editor.Snapshot, change *editor.ChangeEvent) error {
	if !c.Config.Supports(path) {
		return nil
	}
	if len(s.Text()) > MaxDocumentBytes {
		return errors.New("source exceeds 2 MiB LSP snapshot policy; native editing remains available")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	uri := copilotservice.FileURI(path)
	version, exists := c.versions[uri]
	if exists && version > s.Version {
		return errors.New("request snapshot is older than the server document")
	}
	if !exists {
		if c.openClose {
			if err := c.RPC.Notify(ctx, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "languageId": copilotservice.LanguageID(path), "version": s.Version, "text": s.Text()}}); err != nil {
				return err
			}
		}
	} else if s.Version > version && c.syncKind != 0 {
		changes := []any{map[string]string{"text": s.Text()}}
		if c.syncKind == 2 && change != nil && change.Version == s.Version && version == s.Version-1 && len(change.Changes) > 0 {
			changes = nil
			for _, ch := range change.Changes {
				changes = append(changes, map[string]any{"range": ch.Range, "text": ch.Text})
			}
		}
		if err := c.RPC.Notify(ctx, "textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": s.Version}, "contentChanges": changes}); err != nil {
			return err
		}
	}
	c.versions[uri] = s.Version
	return nil
}
func (c *Client) Save(ctx context.Context, path string, s editor.Snapshot) error {
	if !c.save {
		return nil
	}
	params := map[string]any{"textDocument": map[string]string{"uri": copilotservice.FileURI(path)}}
	if c.saveText {
		params["text"] = s.Text()
	}
	return c.RPC.Notify(ctx, "textDocument/didSave", params)
}
func (c *Client) CloseDocument(ctx context.Context, path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	uri := copilotservice.FileURI(path)
	if _, ok := c.versions[uri]; !ok {
		return nil
	}
	delete(c.versions, uri)
	if !c.openClose {
		return nil
	}
	return c.RPC.Notify(ctx, "textDocument/didClose", map[string]any{"textDocument": map[string]string{"uri": uri}})
}
func (c *Client) Request(ctx context.Context, path string, s editor.Snapshot, position editor.Position, method string, out any) error {
	if !c.Supports(method) {
		return fmt.Errorf("%s does not advertise %s", c.Config.Name, method)
	}
	if err := c.Sync(ctx, path, s, nil); err != nil {
		return err
	}
	params := map[string]any{"textDocument": map[string]string{"uri": copilotservice.FileURI(path)}, "position": position}
	if method == "textDocument/formatting" {
		delete(params, "position")
		params["options"] = map[string]any{"tabSize": 4, "insertSpaces": true}
	}
	if method == "textDocument/completion" {
		params["context"] = map[string]int{"triggerKind": 1}
	}
	return c.RPC.Call(ctx, method, params, out)
}
func PathFromURI(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "file" {
		return "", errors.New("language server returned a non-file location")
	}
	if u.Host != "" && u.Host != "localhost" {
		return "", errors.New("remote file URI is unsupported")
	}
	path := u.Path
	if runtime.GOOS == "windows" && len(path) > 2 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.Abs(filepath.FromSlash(path))
}
func Completions(raw json.RawMessage) ([]Completion, error) {
	var items []Completion
	if json.Unmarshal(raw, &items) == nil {
		return items, nil
	}
	var list struct {
		Items []Completion `json:"items"`
	}
	err := json.Unmarshal(raw, &list)
	return list.Items, err
}
func Locations(raw json.RawMessage) ([]Location, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		items = []json.RawMessage{raw}
	}
	var result []Location
	for _, item := range items {
		var loc Location
		if err := json.Unmarshal(item, &loc); err != nil {
			return nil, err
		}
		if loc.URI == "" {
			var link struct {
				URI   string       `json:"targetUri"`
				Range editor.Range `json:"targetSelectionRange"`
			}
			if err := json.Unmarshal(item, &link); err != nil {
				return nil, err
			}
			loc = Location{link.URI, link.Range}
		}
		if loc.URI != "" {
			result = append(result, loc)
		}
	}
	return result, nil
}
func HoverText(raw json.RawMessage) string {
	var hover struct {
		Contents json.RawMessage `json:"contents"`
	}
	if json.Unmarshal(raw, &hover) != nil {
		return ""
	}
	var flatten func(json.RawMessage) string
	flatten = func(raw json.RawMessage) string {
		var value string
		if json.Unmarshal(raw, &value) == nil {
			return value
		}
		var content struct {
			Value string `json:"value"`
		}
		if json.Unmarshal(raw, &content) == nil && content.Value != "" {
			return content.Value
		}
		var parts []json.RawMessage
		if json.Unmarshal(raw, &parts) == nil {
			var texts []string
			for _, part := range parts {
				texts = append(texts, flatten(part))
			}
			return strings.Join(texts, "\n")
		}
		return ""
	}
	text := flatten(hover.Contents)
	if len(text) > 16<<10 {
		text = text[:16<<10]
	}
	return strings.ToValidUTF8(text, "")
}
