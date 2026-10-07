package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	textbuffer "github.com/neko233-com/godesktop/editor"
)

func TestEditorExtensionViewIdentitySelectionRevealAndRenumber(t *testing.T) {
	m := testModel(t)
	d := m.current()
	d.buffer, _ = textbuffer.New(strings.Repeat("😀界\r\n", 140))
	d.serviceVersion = 0
	m.moveCursor(d, 100, 1, false)
	m.splitEditor(false)
	for _, g := range m.allGroups() {
		g.visibleRows = 20
	}
	state := m.editorLayout(nil, 0)
	if len(state.Editors) != 2 || state.Editors[0].ID == state.Editors[1].ID || state.Editors[0].Path != state.Editors[1].Path {
		t.Fatal("views are collapsed into a document", state)
	}
	id := state.Editors[0].ID
	activeSelection := d.buffer.Selection()
	active := m.groups.active
	p := textbuffer.Position{Line: 3, Character: 2}
	selection := textbuffer.Selection{Anchor: p, Active: p}
	if err := m.setEditorSelection(id, d.path, d.buffer.Version(), selection); err != nil {
		t.Fatal(err)
	}
	if m.groups.active != active || d.buffer.Selection() != activeSelection || m.allGroups()[0].views[d].selection != selection {
		t.Fatal("inactive selection stole focus or canonical caret")
	}
	r := textbuffer.Range{Start: textbuffer.Position{Line: 90}, End: textbuffer.Position{Line: 90}}
	if err := m.revealEditorRange(id, d.path, d.buffer.Version(), r, 3); err != nil {
		t.Fatal(err)
	}
	state = m.editorLayout(nil, 0)
	if state.Editors[0].VisibleRanges[0].Start.Line != 90 || m.groups.active != active {
		t.Fatal("reveal did not retain independent viewport", state)
	}
	m.focusGroup(m.allGroups()[0].id)
	m.codeView(20)
	if d.scroll != 90 || d.buffer.Selection() != selection {
		t.Fatal("renderer/focus undid explicit reveal", d.scroll, d.buffer.Selection())
	}
	m.moveCursor(d, 120, 1, false)
	m.codeView(20)
	if d.scroll != 101 {
		t.Fatal("caret following did not resume after native movement", d.scroll)
	}
	if err := m.setEditorSelection(id, d.path, d.buffer.Version()-1, selection); err == nil {
		t.Fatal("stale document selection accepted")
	}
	right := m.allGroups()[1]
	rightID := m.editorLayout(nil, 0).Editors[1].ID
	m.dropEditorGroup(m.allGroups()[0].id)
	state = m.editorLayout(nil, 0)
	if state.Editors[0].ID != rightID || state.Editors[0].ViewColumn != 1 || m.allGroups()[0] != right {
		t.Fatal("renumber changed surviving identity", state)
	}
	if err := m.setEditorSelection(id, d.path, d.buffer.Version(), selection); err == nil {
		t.Fatal("disposed view changed surviving source")
	}
}

func TestEditorExtensionHiddenTabsDisposeIDsAndNineColumns(t *testing.T) {
	m := tabModel(t)
	d := m.current()
	initial := m.editorLayout(nil, 0)
	if len(initial.Editors) != 1 {
		t.Fatal("hidden tabs advertised as visible", initial)
	}
	id := initial.Editors[0].ID
	m.focusTab(m.docs[1])
	if next := m.editorLayout(nil, 0); next.Editors[0].ID == id {
		t.Fatal("different document reused editor identity")
	}
	m.focusTab(d)
	if next := m.editorLayout(nil, 0); next.Editors[0].ID == id {
		t.Fatal("disposed editor revived")
	}
	current := m.groups.active
	last, err := m.editorColumn(9)
	if err != nil || len(m.allGroups()) != 9 || last != m.allGroups()[8] || m.groups.active != current {
		t.Fatal("Nine column creation stole focus", err)
	}
	if len(m.editorLayout(nil, 0).Editors) != 1 {
		t.Fatal("intermediate empty columns exposed fake editors")
	}
	if _, err = m.editorColumn(10); err == nil {
		t.Fatal("invalid column accepted")
	}
	if same, err := m.editorColumn(-1); err != nil || same.id != current {
		t.Fatal("Active resolved incorrectly")
	}
	if beside, err := m.editorColumn(-2); err != nil || beside != m.allGroups()[1] {
		t.Fatal("Beside resolved incorrectly")
	}
}

func TestEditorExtensionRealHiddenOpenAndPreserveFocus(t *testing.T) {
	m := testModel(t)
	original := m.current()
	m.ensureGroups()
	m.editing = false
	m.terminalFocused = true
	file := filepath.Join(m.workspace, "hidden.go")
	if err := os.WriteFile(file, []byte("😀界\r\nsecond"), 0600); err != nil {
		t.Fatal(err)
	}
	mailbox := make(chan func(), 32)
	stop := m.startFileOpens(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, nil)
	defer stop()
	var opened *document
	var failure error
	m.requestDocumentOpen(context.Background(), file, func(d *document, err error) { opened, failure = d, err })
	saveAck(t, mailbox)()
	saveAck(t, mailbox)()
	if failure != nil || opened == nil || opened.buffer.Text() != "😀界\r\nsecond" || len(m.docs) != 2 || len(m.allGroups()[0].docs) != 1 || m.current() != original || !m.terminalFocused {
		t.Fatal("openTextDocument exposed/focused a hidden resource", failure)
	}
	r := textbuffer.Range{Start: textbuffer.Position{Character: 2}, End: textbuffer.Position{Character: 3}}
	var response any
	m.showExtensionDocument(context.Background(), editorOpenRequest{Path: file, ViewColumn: -2, PreserveFocus: true, Selection: &r}, func(value any, err error) { response, failure = value, err })
	if failure != nil || response == nil || len(m.allGroups()) != 2 || m.current() != original || m.editing || !m.terminalFocused || m.allGroups()[1].current != opened {
		t.Fatal("preserveFocus did not preserve native focus", failure)
	}
	if state := m.editorLayout(nil, 0); len(state.Editors) != 2 || state.Editors[1].Selection.Active != r.End || state.Active != state.Editors[0].ID {
		t.Fatal("selected native target differs from acknowledgement", state)
	}
}

func TestEditorExtensionDelayedShowRejectsNewFocusAndClosedTarget(t *testing.T) {
	for _, closeTarget := range []bool{false, true} {
		t.Run(map[bool]string{false: "new-focus", true: "closed-target"}[closeTarget], func(t *testing.T) {
			m := testModel(t)
			original := m.current()
			m.ensureGroups()
			if closeTarget {
				m.editorColumn(2)
			}
			file := filepath.Join(m.workspace, "delayed.go")
			os.WriteFile(file, []byte("real delayed bytes"), 0600)
			mailbox := make(chan func(), 32)
			entered, release := make(chan struct{}), make(chan struct{})
			stop := m.startFileOpens(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, func(ctx context.Context, path string) (*document, error) {
				close(entered)
				<-release
				return loadDocument(ctx, path)
			})
			defer stop()
			var failure error
			finished := false
			m.showExtensionDocument(context.Background(), editorOpenRequest{Path: file, ViewColumn: 2, PreserveFocus: closeTarget}, func(_ any, err error) { failure = err; finished = true })
			<-entered
			if closeTarget {
				m.dropEditorGroup(m.allGroups()[1].id)
			} else {
				m.focusTab(original)
				m.replaceSelection(original, "local ")
			}
			close(release)
			saveAck(t, mailbox)()
			saveAck(t, mailbox)()
			if !finished || !errors.Is(failure, errOpenSuperseded) || m.current() != original || len(m.allGroups()) != 1 || len(m.allGroups()[0].docs) != 1 {
				t.Fatal("old show acknowledged a different editor/recreated target", failure)
			}
			if d := m.findDocument(file); d == nil || d.buffer.Text() != "real delayed bytes" {
				t.Fatal("real hidden resource was lost")
			}
		})
	}
}

func TestEditorExtensionGiBOpenRejectedBeforeReaderAndAdoption(t *testing.T) {
	m := testModel(t)
	m.ensureGroups()
	file := filepath.Join(m.workspace, "huge.txt")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(1 << 30); err != nil {
		t.Fatal(err)
	}
	f.Close()
	mailbox := make(chan func(), 32)
	var reads atomic.Int32
	stop := m.startFileOpens(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, func(ctx context.Context, path string) (*document, error) {
		reads.Add(1)
		return loadDocument(ctx, path)
	})
	defer stop()
	var failure error
	m.requestDocumentOpen(context.Background(), file, func(_ *document, err error) { failure = err })
	saveAck(t, mailbox)()
	saveAck(t, mailbox)()
	if failure == nil || reads.Load() != 0 || m.findDocument(file) != nil || len(m.docs) != 1 {
		t.Fatal("extension opened/indexed a GiB file before text policy rejection", failure, reads.Load())
	}
}

func TestEditorExtensionHiddenReceiptDoesNotAdoptNewerFocus(t *testing.T) {
	m := tabModel(t)
	m.ensureGroups()
	old := m.current()
	m.editorLayout(nil, 0)
	receipt := m.editorFocusReceipt()
	m.focusTab(m.docs[1])
	var failure error
	m.showExtensionDocument(context.Background(), editorOpenRequest{Path: old.path, FocusReceipt: receipt}, func(_ any, err error) { failure = err })
	if !errors.Is(failure, errOpenSuperseded) || m.current() != m.docs[1] {
		t.Fatal("completed hidden read stole newer focus", failure)
	}
}
