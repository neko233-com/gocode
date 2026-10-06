// Package lspfixture is an owned synthetic stdio server used by process and
// workbench regression tests. It is not imported by the application.
package lspfixture

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"

	"github.com/neko233-com/godesktop/lsp"
)

const Flag = "gocode-owned-lsp-fixture"

func RunIfRequested() bool {
	args := os.Args
	if len(args) < 2 || args[len(args)-2] != Flag {
		return false
	}
	if args[len(args)-1] == "fail" {
		os.Exit(23)
	}
	if marker, ok := strings.CutPrefix(args[len(args)-1], "fail-until:"); ok {
		if _, err := os.Stat(marker); err != nil {
			os.Exit(23)
		}
	}
	peer := lsp.Connect(os.Stdin, os.Stdout, nil)
	defer peer.Close()
	type doc struct {
		URI     string `json:"uri"`
		Version int    `json:"version"`
		Text    string `json:"text"`
	}
	var mu sync.Mutex
	docs := map[string]doc{}
	peer.Register("initialize", func(ctx context.Context, raw json.RawMessage) (any, error) {
		if marker, ok := strings.CutPrefix(args[len(args)-1], "hang:"); ok {
			_ = os.WriteFile(marker, []byte("initialize received"), 0600)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return map[string]any{"capabilities": map[string]any{"positionEncoding": "utf-16", "textDocumentSync": 2, "hoverProvider": true, "completionProvider": map[string]any{}}}, nil
	})
	peer.Register("shutdown", func(context.Context, json.RawMessage) (any, error) { return nil, nil })
	peer.Register("fixture/crash", func(context.Context, json.RawMessage) (any, error) { os.Exit(24); return nil, nil })
	peer.Register("textDocument/hover", func(_ context.Context, raw json.RawMessage) (any, error) {
		var p struct {
			TextDocument doc `json:"textDocument"`
		}
		_ = json.Unmarshal(raw, &p)
		mu.Lock()
		d := docs[p.TextDocument.URI]
		mu.Unlock()
		return map[string]any{"contents": map[string]string{"kind": "plaintext", "value": d.Text}}, nil
	})
	for event := range peer.Notifications() {
		if event.Method == "exit" {
			return true
		}
		var p struct {
			TextDocument   doc `json:"textDocument"`
			ContentChanges []struct {
				Text string `json:"text"`
			} `json:"contentChanges"`
		}
		_ = json.Unmarshal(event.Params, &p)
		switch event.Method {
		case "textDocument/didOpen", "textDocument/didChange":
			d := p.TextDocument
			if len(p.ContentChanges) > 0 {
				d.Text = p.ContentChanges[len(p.ContentChanges)-1].Text
			}
			mu.Lock()
			docs[d.URI] = d
			mu.Unlock()
			items := []any{map[string]any{"range": map[string]any{"start": map[string]int{}, "end": map[string]int{}}, "message": d.Text, "severity": 1}}
			if args[len(args)-1] == "large-diagnostics" {
				for len(items) < 3000 {
					items = append(items, items[0])
				}
			}
			_ = peer.Notify(context.Background(), "textDocument/publishDiagnostics", map[string]any{"uri": d.URI, "version": d.Version, "diagnostics": items})
		case "textDocument/didClose":
			mu.Lock()
			delete(docs, p.TextDocument.URI)
			mu.Unlock()
		}
	}
	return true
}
