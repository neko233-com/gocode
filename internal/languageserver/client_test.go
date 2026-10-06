package languageserver

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/copilotservice"
	"github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/lsp"
)

func TestRealFramingCapabilitiesVersionsAndLifecycle(t *testing.T) {
	app, server := net.Pipe()
	rpc := lsp.Connect(app, app, nil)
	peer := lsp.Connect(server, server, nil)
	defer rpc.Close()
	defer peer.Close()
	peer.Register("initialize", func(_ context.Context, raw json.RawMessage) (any, error) {
		if !strings.Contains(string(raw), `"utf-16"`) || !strings.Contains(string(raw), `"snippetSupport":false`) {
			t.Error("capability negotiation missing")
		}
		return map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-16", "textDocumentSync": map[string]any{"openClose": true, "change": 2, "save": map[string]bool{"includeText": true}}, "hoverProvider": true, "completionProvider": map[string]any{}}}, nil
	})
	peer.Register("textDocument/hover", func(_ context.Context, raw json.RawMessage) (any, error) {
		return map[string]any{"contents": map[string]string{"kind": "plaintext", "value": "actual hover"}}, nil
	})
	peer.Register("shutdown", func(context.Context, json.RawMessage) (any, error) { return nil, nil })
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	workspace := t.TempDir()
	config := Config{Name: "fixture", Command: "unused", Languages: []string{"go"}, Settings: map[string]any{"fixture": map[string]any{"mode": "fast"}}}
	c, err := Initialize(ctx, rpc, workspace, config)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workspace, "你 SHORT~1.go")
	b, _ := editor.New("你😀\r\n")
	first := b.Snapshot()
	if err := c.Sync(ctx, path, first, nil); err != nil {
		t.Fatal(err)
	}
	_ = b.SetSelection(editor.Selection{Anchor: editor.Position{Character: 3}, Active: editor.Position{Character: 3}})
	change, _ := b.ReplaceSelection("!")
	if err := c.Sync(ctx, path, b.Snapshot(), &change); err != nil {
		t.Fatal(err)
	}
	if err := c.Sync(ctx, path, first, nil); err == nil {
		t.Fatal("stale source was resynchronized")
	}
	if err := c.Save(ctx, path, b.Snapshot()); err != nil {
		t.Fatal(err)
	}
	var hover json.RawMessage
	if err := c.Request(ctx, path, b.Snapshot(), editor.Position{Character: 4}, "textDocument/hover", &hover); err != nil || HoverText(hover) != "actual hover" {
		t.Fatalf("request %s %v", hover, err)
	}
	if err := c.Request(ctx, path, b.Snapshot(), editor.Position{}, "textDocument/formatting", nil); err == nil {
		t.Fatal("unadvertised method accepted")
	}
	if err := c.CloseDocument(ctx, path); err != nil {
		t.Fatal(err)
	}
	want := []string{"initialized", "workspace/didChangeConfiguration", "textDocument/didOpen", "textDocument/didChange", "textDocument/didSave", "textDocument/didClose"}
	for _, method := range want {
		select {
		case p := <-peer.Notifications():
			if p.Method != method {
				t.Fatalf("expected %s, got %s", method, p.Method)
			}
			if method == "textDocument/didChange" && (!strings.Contains(string(p.Params), `"character":3`) || !strings.Contains(string(p.Params), `"text":"!"`) || strings.Contains(string(p.Params), `你😀`)) {
				t.Fatalf("incremental UTF-16 sync %s", p.Params)
			}
			if method == "textDocument/didSave" && !strings.Contains(string(p.Params), `你😀!\r\n`) {
				t.Fatalf("requested save text omitted %s", p.Params)
			}
		case <-ctx.Done():
			t.Fatal("missing", method)
		}
	}
	var settings []any
	if err := peer.Call(ctx, "workspace/configuration", map[string]any{"items": []any{map[string]string{"section": "fixture.mode"}, map[string]string{"section": "missing"}}}, &settings); err != nil || len(settings) != 2 || settings[0] != "fast" || settings[1] != nil {
		t.Fatalf("server settings %v %v", settings, err)
	}
	actual, err := PathFromURI(copilotservice.FileURI(path))
	if err != nil || actual != path {
		t.Fatalf("file URI %q %v", actual, err)
	}
	c.Close()
}

func TestResponseVariantsAndPolicy(t *testing.T) {
	items, err := Completions(json.RawMessage(`{"isIncomplete":false,"items":[{"label":"sum","insertText":"sum"}]}`))
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	for _, raw := range []string{`{"uri":"file:///a","range":{"start":{"line":1,"character":2}}}`, `[{"targetUri":"file:///a","targetSelectionRange":{"start":{"line":1,"character":2}}}]`} {
		loc, err := Locations(json.RawMessage(raw))
		if err != nil || len(loc) != 1 || loc[0].Range.Start.Line != 1 {
			t.Fatal(loc, err)
		}
	}
	if HoverText(json.RawMessage(`{"contents":["hello",{"language":"go","value":"world"}]}`)) != "hello\nworld" {
		t.Fatal("hover variants")
	}
	if _, err := PathFromURI("https://example.com/file.go"); err == nil {
		t.Fatal("external URI accepted")
	}
	if _, err := PathFromURI("file://server/share/a.go"); err == nil {
		t.Fatal("remote path accepted")
	}
	b, _ := editor.New(strings.Repeat("x", MaxDocumentBytes+1))
	c := &Client{Config: Config{Languages: []string{"go"}}}
	if err := c.Sync(context.Background(), "a.go", b.Snapshot(), nil); err == nil {
		t.Fatal("oversized snapshot accepted")
	}
}
