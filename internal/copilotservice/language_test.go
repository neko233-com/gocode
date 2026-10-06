package copilotservice

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/lsp"
)

func TestOfficialProtocolDocumentSyncAndAcceptanceOrdering(t *testing.T) {
	app, server := net.Pipe()
	defer server.Close()
	client := lsp.Connect(app, app, nil)
	defer client.Close()
	requests := make(chan map[string]json.RawMessage, 32)
	go func() {
		reader := bufio.NewReader(server)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			length, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:")))
			if _, err = reader.ReadString('\n'); err != nil {
				return
			}
			body := make([]byte, length)
			if _, err = io.ReadFull(reader, body); err != nil {
				return
			}
			var packet map[string]json.RawMessage
			if json.Unmarshal(body, &packet) != nil {
				return
			}
			requests <- packet
			if packet["id"] != nil {
				result := json.RawMessage(`{}`)
				if string(packet["method"]) == `"textDocument/inlineCompletion"` {
					result = json.RawMessage(`{"items":[{"insertText":"sum","range":{"start":{"line":0,"character":3},"end":{"line":0,"character":3}},"command":{"command":"github.copilot.didAcceptCompletionItem","arguments":["fixture-1"]}}]}`)
				}
				reply, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": packet["id"], "result": result})
				_, _ = fmt.Fprintf(server, "Content-Length: %d\r\n\r\n", len(reply))
				_, _ = server.Write(reply)
			}
		}
	}()
	language := &Language{RPC: client, versions: map[string]int{}}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	workspace := t.TempDir()
	file := filepath.Join(workspace, "你 main.go")
	if err := language.Initialize(ctx, workspace); err != nil {
		t.Fatal(err)
	}
	buffer, _ := editor.New("你😀\r\n")
	if err := language.Sync(ctx, file, buffer.Snapshot(), nil); err != nil {
		t.Fatal(err)
	}
	_ = buffer.SetSelection(editor.Selection{Anchor: editor.Position{Character: 3}, Active: editor.Position{Character: 3}})
	change, _ := buffer.ReplaceSelection("!")
	if err := language.Sync(ctx, file, buffer.Snapshot(), &change); err != nil {
		t.Fatal(err)
	}
	items, err := language.Inline(ctx, file, buffer.Snapshot(), editor.Position{Character: 4})
	if err != nil || len(items) != 1 {
		t.Fatalf("inline %v %v", items, err)
	}
	if err := language.Shown(ctx, items[0]); err != nil {
		t.Fatal(err)
	}
	if err := language.Accepted(ctx, items[0]); err != nil {
		t.Fatal(err)
	}
	if err := language.CloseDocument(ctx, file); err != nil {
		t.Fatal(err)
	}
	want := []string{"initialize", "initialized", "workspace/didChangeConfiguration", "textDocument/didOpen", "textDocument/didChange", "textDocument/inlineCompletion", "textDocument/didShowCompletion", "workspace/executeCommand", "textDocument/didClose"}
	for _, method := range want {
		var packet map[string]json.RawMessage
		select {
		case packet = <-requests:
		case <-ctx.Done():
			t.Fatal("missing", method)
		}
		if string(packet["method"]) != strconv.Quote(method) {
			t.Fatalf("protocol order expected %s, got %s", method, packet["method"])
		}
		if method == "textDocument/didChange" && (!strings.Contains(string(packet["params"]), `"character":3`) || !strings.Contains(string(packet["params"]), `"version":2`)) {
			t.Fatalf("UTF-16 incremental changes %s", packet["params"])
		}
		if method == "workspace/executeCommand" && !strings.Contains(string(packet["params"]), "fixture-1") {
			t.Fatal("acceptance lost command arguments")
		}
	}
}

func TestRuntimePathsAreExplicitAndURIsEscaped(t *testing.T) {
	if uri := FileURI(filepath.Join(t.TempDir(), "你 a#b.go")); !strings.Contains(uri, "%20") || !strings.Contains(uri, "%23") {
		t.Fatalf("invalid URI %s", uri)
	}
	if LanguageID("a.go") != "go" || LanguageID("a.json") != "json" || LanguageID("a.xyz") != "plaintext" {
		t.Fatal("language mapping")
	}
	t.Setenv("GOCODE_COPILOT_CLI", "")
	if _, err := CLIPath(t.TempDir()); err == nil {
		t.Fatal("missing runtime falsely accepted")
	}
}
