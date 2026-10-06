package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	workspaceSearch "github.com/neko233-com/gocode/internal/search"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func searchActor(t *testing.T, m *model, run func(context.Context, string, workspaceSearch.Query, map[string]workspaceSearch.Overlay) workspaceSearch.Report) chan func() {
	t.Helper()
	mailbox := make(chan func(), 64)
	stop := m.startSearch(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, run)
	t.Cleanup(stop)
	return mailbox
}
func TestSearchLatestQueryFrozenOverlayAndCancelledReceipt(t *testing.T) {
	m := testModel(t)
	d := m.current()
	mailbox := searchActor(t, m, nil)
	m.search.query = workspaceSearch.Query{Text: "package", CaseSensitive: true}
	m.search.submit()
	old := saveAck(t, mailbox)
	m.search.query.Text = "func"
	m.search.submit()
	newer := saveAck(t, mailbox)
	old()
	if !m.search.busy || len(m.search.report.Matches) != 0 {
		t.Fatal("old receipt replaced the pending query")
	}
	newer()
	if len(m.search.report.Matches) != 1 || m.search.report.Matches[0].Version != d.buffer.Version() || m.search.report.Matches[0].Identity != d.tabID {
		t.Fatal(m.search.report)
	}
	m.search.query.Text = "package"
	m.search.submit()
	held := saveAck(t, mailbox)
	m.stopSearch()
	held()
	if m.search.busy || !strings.Contains(m.search.status, "cancelled") {
		t.Fatal("cancelled query was adopted")
	}
	m.search.query.Text = ""
	m.search.submit()
	if len(m.search.rows) != 0 || m.search.status != "" {
		t.Fatal("empty query retained old rows")
	}
}
func TestSearchNavigationUTF16AndStaleFocusGuard(t *testing.T) {
	m := testModel(t)
	d := m.current()
	mailbox := searchActor(t, m, nil)
	m.search.query = workspaceSearch.Query{Text: "func", CaseSensitive: true}
	m.search.submit()
	saveAck(t, mailbox)()
	m.activateSearchResult(nil, 0)
	held := saveAck(t, mailbox)
	before := d.buffer.Selection()
	m.documentEvent("focus", d, textbuffer.ChangeEvent{})
	held()
	if d.buffer.Selection() != before {
		t.Fatal("old navigation moved the caret after a newer focus gesture")
	}
	m.activateSearchResult(nil, 0)
	saveAck(t, mailbox)()
	if d.buffer.Selection().Range() != m.search.report.Matches[0].Range || m.search.focus != -2 {
		t.Fatal("result did not select its protocol range or retain editor focus")
	}
	m.editing = true
	text(m, "newer")
	if len(m.search.report.Matches) != 0 {
		t.Fatal("edit retained stale results")
	}
	body, _ := os.ReadFile(d.path)
	if strings.Contains(string(body), "newer") {
		t.Fatal("search wrote source")
	}
}
func TestSearchQueryInputIsolationAndDebounceCancelled(t *testing.T) {
	m := testModel(t)
	d := m.current()
	before := d.buffer.Text()
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'F', Modifiers: ui.ModifierControl | ui.ModifierShift})
	text(m, "😀needle")
	if m.search.query.Text != "😀needle" || d.buffer.Text() != before {
		t.Fatal("query typed into editor")
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'A', Modifiers: ui.ModifierControl})
	m.readClipboard = func() (string, error) { return "case", nil }
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'V', Modifiers: ui.ModifierControl})
	if m.search.query.Text != "case" {
		t.Fatal("input select/paste")
	}
	key(m, 37)
	key(m, 8)
	if m.search.query.Text != "cae" {
		t.Fatal("caret backspace", m.search.query.Text)
	}
	key(m, 9)
	text(m, "**/*.go")
	if m.search.query.Include != "**/*.go" {
		t.Fatal("include field")
	}
	m.search.query.Text = "needle"
	m.search.focus = -2
	m.editing = true
	text(m, "editable")
	if !strings.Contains(d.buffer.Text(), "editable") || m.search.query.Text != "needle" {
		t.Fatal("search activity stole editor typing")
	}
	mailbox := searchActor(t, m, func(c context.Context, _ string, _ workspaceSearch.Query, _ map[string]workspaceSearch.Overlay) workspaceSearch.Report {
		<-c.Done()
		return workspaceSearch.Report{Err: c.Err()}
	})
	m.search.submit()
	m.stopSearch()
	select {
	case fn := <-mailbox:
		fn()
	case <-time.After(time.Second):
		t.Fatal("cancelled worker did not stop")
	}
}

func TestFirstNextResultStartsAtFirstMatch(t *testing.T) {
	m := testModel(t)
	m.showSearch(nil)
	m.search.focus = -2
	m.search.report.Matches = make([]workspaceSearch.Match, 3)
	if !m.searchInput(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 115}) || m.search.selected != 0 {
		t.Fatal("first F4 skipped the first match")
	}
	m.searchInput(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 115})
	if m.search.selected != 1 {
		t.Fatal("next F4 did not advance")
	}
	m.search.selected = -1
	m.searchInput(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 115, Modifiers: ui.ModifierShift})
	if m.search.selected != 2 {
		t.Fatal("first reverse result did not start at last match")
	}
}
