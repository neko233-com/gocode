package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/gocode/internal/languageserver/lspfixture"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func TestWorkbenchLanguageServerProcess(t *testing.T) {
	if !lspfixture.RunIfRequested() {
		t.Skip("owned helper process only")
	}
}

func TestWorkbenchReplaysUnsavedSourceAndDoesNotStackHooks(t *testing.T) {
	m := testModel(t)
	d := m.current()
	d.buffer, _ = textbuffer.New("package main\n// initial 😀\n")
	mailbox := make(chan func(), 16)
	dispatch := func(fn func()) bool { mailbox <- fn; return true }
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	notifications := 0
	m.onDocument = func(string, *document, textbuffer.ChangeEvent) { notifications++ }
	config := languageserver.Config{Name: "fixture", Languages: []string{"go"}, Command: os.Args[0], Arguments: []string{"-test.run=^TestWorkbenchLanguageServerProcess$", "--", lspfixture.Flag, "serve"}}
	otherConfig := config
	otherConfig.Name, otherConfig.Languages = "other", []string{"python"}
	closeServices := m.bindLanguages(ctx, dispatch, []languageserver.Config{config, otherConfig})
	defer closeServices()
	b := m.languageBindings[0]
	pump := func(until func() bool) {
		t.Helper()
		for !until() {
			select {
			case fn := <-mailbox:
				fn()
			case <-ctx.Done():
				t.Fatal("UI lifecycle stalled", b.state, m.message)
			}
		}
	}
	key := "lsp:fixture\x00" + d.path
	pump(func() bool { return len(m.diagnostics[key]) > 0 && m.languageBindings[1].session != nil })
	otherSession := m.languageBindings[1].session
	first := b.session
	m.diagnostics["lsp:other\x00"+d.path] = []diagnostic{{Message: "other server retained"}}
	go first.Client.RPC.Close()
	pump(func() bool { return b.session == nil })
	if len(m.diagnostics[key]) != 0 || len(m.diagnostics["lsp:other\x00"+d.path]) != 1 {
		t.Fatal("disconnect retained old diagnostics or cleared another server")
	}
	if err := m.applyDocumentEdits(d.path, d.buffer.Version(), []textbuffer.Edit{{Range: textbuffer.Range{}, Text: "// latest unsaved 😀\n"}}); err != nil {
		t.Fatal(err)
	}
	pump(func() bool { return b.session != nil && b.session != first && len(m.diagnostics[key]) > 0 })
	if !strings.Contains(m.diagnostics[key][0].Message, "latest unsaved 😀") || !d.dirty() {
		t.Fatal("replacement didOpen lost unsaved source", m.diagnostics[key])
	}
	if !otherSession.Valid() || m.languageBindings[1].session != otherSession {
		t.Fatal("one server crash restarted another configured server")
	}
	if m.message != "" {
		t.Fatal("recovered UI retained stale disconnect notice", m.message)
	}
	for i := 0; i < 2; i++ {
		previous := b.session
		m.restartLanguages()
		pump(func() bool {
			return b.session != nil && b.session != previous && len(m.diagnostics[key]) > 0 && m.languageBindings[1].session != nil
		})
	}
	before := notifications
	m.onDocument("focus", d, textbuffer.ChangeEvent{})
	if notifications != before+1 {
		t.Fatal("restart stacked document hooks", notifications-before)
	}

	// Hold a genuine server result in the UI mailbox, then reopen the same path
	// at version 1. The old result must not mutate the replacement document/UI.
	pump(func() bool { return len(mailbox) == 0 })
	m.requestLSP(d, "textDocument/hover")
	var result func()
	select {
	case result = <-mailbox:
	case <-ctx.Done():
		t.Fatal("missing actual server response")
	}
	replacement := &document{path: d.path}
	replacement.buffer, _ = textbuffer.New("package main\n// replacement\n")
	m.docs = []*document{replacement}
	m.output = nil
	result()
	if len(m.output) != 0 {
		t.Fatal("queued response for closed document was accepted", m.output)
	}
}
