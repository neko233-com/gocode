package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func historyActor(t *testing.T, m *model) chan func() {
	t.Helper()
	mailbox := make(chan func(), 64)
	stop := m.startHistory(context.Background(), func(fn func()) bool { mailbox <- fn; return true })
	t.Cleanup(stop)
	return mailbox
}
func historyReplacement(t *testing.T) (*model, chan func(), []string) {
	t.Helper()
	m, search, saves := replacementModel(t)
	m.replaceSelection(m.current(), "// unsaved main\n")
	before := []string{m.current().buffer.Text(), "main 😀\r\nmain\r\n"}
	replacementPreview(t, m, search)
	m.applyReplacement(nil)
	saveAck(t, search)()
	saveAck(t, saves)()
	if len(m.history.groups) != 1 {
		t.Fatal("replacement did not register one workspace history group")
	}
	return m, historyActor(t, m), before
}

func TestWorkspaceHistoryAtomicUndoRedoAndRealDiskSafety(t *testing.T) {
	m, mailbox, before := historyReplacement(t)
	d := m.current()
	m.search.focus = -2
	m.editing = true
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'Z', Modifiers: ui.ModifierControl})
	if m.history.prompt == nil || d.buffer.Text() == before[0] {
		t.Fatal("native shortcut skipped multi-file confirmation")
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 27})
	if m.history.prompt != nil || d.buffer.Dirty() {
		t.Fatal("cancel changed source")
	}
	m.requestHistory(d, false)
	m.confirmHistory(true)
	held := saveAck(t, mailbox)
	if d.buffer.Dirty() {
		t.Fatal("worker changed live history")
	}
	observed := 0
	m.onDocument = func(kind string, _ *document, _ textbuffer.ChangeEvent) {
		if kind == "change" {
			observed++
			for i, doc := range m.docs {
				if doc.buffer.Text() != before[i] {
					t.Fatal("notification saw partially committed batch")
				}
			}
		}
	}
	held()
	if observed != 2 || !m.history.groups[0].undone || m.history.busy {
		t.Fatal("group transition/notifications missing")
	}
	for i, doc := range m.docs {
		if doc.buffer.Text() != before[i] || !doc.buffer.Dirty() {
			t.Fatal("undo lost unsaved prefix/EOL/saved revision")
		}
		raw, _ := os.ReadFile(doc.path)
		if string(raw) == before[i] {
			t.Fatal("workspace undo implicitly saved over disk")
		}
	}
	// External source remains untouched by both undo and redo.
	os.WriteFile(m.docs[1].path, []byte("external replacement"), 0600)
	m.onDocument = nil
	m.requestHistory(d, true)
	saveAck(t, mailbox)()
	for _, doc := range m.docs {
		if doc.buffer.Dirty() {
			t.Fatal("redo failed to return to exact saved revision")
		}
	}
	raw, _ := os.ReadFile(m.docs[1].path)
	if string(raw) != "external replacement" {
		t.Fatal("redo overwrote external file")
	}
	m.requestHistory(d, false)
	m.confirmHistory(true)
	saveAck(t, mailbox)()
	m.requestHistory(d, false) // Earlier single-document unsaved edit remains.
	if strings.Contains(d.buffer.Text(), "unsaved") {
		t.Fatal("older local history lost")
	}
}

func TestWorkspaceHistorySplitProtectsNewerOrReopenedFiles(t *testing.T) {
	for _, scenario := range []string{"newer", "reopened", "one"} {
		t.Run(scenario, func(t *testing.T) {
			m, _, before := historyReplacement(t)
			d, other := m.current(), m.docs[1]
			if scenario == "newer" {
				m.replaceSelection(other, "newer ")
			}
			if scenario == "reopened" {
				b, _ := textbuffer.New(other.buffer.Text())
				next := &document{path: other.path, buffer: b}
				m.docs[1] = next
				m.rememberDocument(next.path, next)
				m.tabKey(next)
				other = next
			}
			protected := other.buffer.Text()
			m.requestHistory(d, false)
			if scenario == "one" {
				m.confirmHistory(false)
			}
			if d.buffer.Text() != before[0] || other.buffer.Text() != protected || len(m.history.groups) != 0 {
				t.Fatal("split modified protected file or retained group")
			}
		})
	}
}

func TestWorkspaceHistoryHeldReceiptsAreAllOrNothing(t *testing.T) {
	for _, scenario := range []string{"edit", "caret", "reopened", "shutdown", "dialog", "expired"} {
		t.Run(scenario, func(t *testing.T) {
			m, mailbox, _ := historyReplacement(t)
			d, other := m.current(), m.docs[1]
			m.requestHistory(d, false)
			if scenario == "dialog" {
				m.replaceSelection(other, "dialog edit ")
				m.confirmHistory(true)
				if m.history.busy {
					t.Fatal("stale dialog started whole-file undo")
				}
				return
			}
			m.confirmHistory(true)
			held := saveAck(t, mailbox)
			switch scenario {
			case "edit":
				m.replaceSelection(other, "newer ")
			case "caret":
				other.buffer.SetSelection(textbuffer.Selection{Anchor: textbuffer.Position{Line: 0, Character: 1}, Active: textbuffer.Position{Line: 0, Character: 1}})
			case "reopened":
				b, _ := textbuffer.New(other.buffer.Text())
				next := &document{path: other.path, buffer: b}
				m.docs[1] = next
				m.rememberDocument(next.path, next)
			case "shutdown":
				m.history.generation++
				m.history.busy = false
			case "expired":
				for range maxWorkspaceHistory {
					m.recordWorkspaceHistory("newer group", m.docs)
				}
			}
			beforeA, beforeB := d.buffer.Text(), other.buffer.Text()
			held()
			if d.buffer.Text() != beforeA || other.buffer.Text() != beforeB || m.history.groups[0].undone {
				t.Fatal("stale worker receipt partially committed")
			}
		})
	}
}

func TestWorkspaceHistoryBoundsAndShutdown(t *testing.T) {
	m, mailbox, _ := historyReplacement(t)
	for range maxWorkspaceHistory + 8 {
		m.recordWorkspaceHistory("bounded", m.docs)
	}
	if len(m.history.groups) != maxWorkspaceHistory {
		t.Fatal("unbounded group metadata")
	}
	ctx, cancel := context.WithCancel(context.Background())
	stop := m.startHistory(ctx, func(fn func()) bool { mailbox <- fn; return true })
	started, finished := make(chan struct{}), make(chan struct{})
	m.history.submit(func(ctx context.Context) error { close(started); <-ctx.Done(); close(finished); return ctx.Err() }, func(error) { t.Fatal("shutdown delivered history receipt") })
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start")
	}
	cancel()
	stop()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel worker")
	}
	for len(mailbox) > 0 {
		(<-mailbox)()
	}
	if m.history.submit != nil || m.history.busy {
		t.Fatal("shutdown retained active worker")
	}
}
