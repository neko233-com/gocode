package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func TestEditorGroupsSharedIdentityIndependentSelectionAndHistory(t *testing.T) {
	m := testModel(t)
	d := m.current()
	d.buffer, _ = textbuffer.New("😀α\r\nsecond\r\nend")
	m.moveCursor(d, 1, 3, false)
	d.scroll = 1
	changes, opens := 0, 0
	m.onDocument = func(kind string, _ *document, _ textbuffer.ChangeEvent) {
		if kind == "change" {
			changes++
		}
		if kind == "open" {
			opens++
		}
	}
	m.splitEditor(false)
	gs := m.allGroups()
	if len(gs) != 2 || len(m.docs) != 1 || gs[0].current != d || gs[1].current != d || m.current() != d || opens != 0 {
		t.Fatal("split duplicated a resource/protocol open")
	}
	m.focusGroup(gs[0].id)
	m.moveCursor(d, 0, 1, false)
	d.scroll = 0
	m.focusGroup(gs[1].id)
	if d.buffer.Selection().Active != (textbuffer.Position{Line: 1, Character: 3}) || d.scroll != 1 {
		t.Fatal("switch lost independent caret/scroll")
	}
	m.replaceSelection(d, "界\n")
	if changes != 1 || d.buffer.Text() != "😀α\r\nsec界\r\nond\r\nend" {
		t.Fatal("shared edit/CRLF protocol differs", d.buffer.Text(), changes)
	}
	m.focusGroup(gs[0].id)
	if d.buffer.Selection().Active != (textbuffer.Position{Line: 0, Character: 2}) || d.scroll != 0 {
		t.Fatal("inactive UTF-16 view moved incorrectly")
	}
	m.localHistory(d, false)
	if d.buffer.Dirty() || d.buffer.Text() != "😀α\r\nsecond\r\nend" {
		t.Fatal("views do not share saved undo history")
	}
	m.localHistory(d, true)
	if !d.buffer.Dirty() {
		t.Fatal("redo lost save point")
	}
}

func TestEditorGroupTabsMRUCloseScopeAndLimits(t *testing.T) {
	m := tabModel(t)
	first := m.current()
	m.splitEditor(false)
	right := m.findGroup(m.groups.active)
	m.focusTab(m.docs[1])
	m.focusTab(first)
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 9, Modifiers: ui.ModifierControl})
	if m.current() != m.docs[1] {
		t.Fatal("MRU escaped group membership")
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyReleased, Key: 17})
	m.focusTab(first)
	m.replaceSelection(first, "dirty ")
	m.closeDocumentTab(first)
	if m.closePrompt || !m.ownsDocument(first) || len(right.docs) != 1 || m.current() != m.docs[1] {
		t.Fatal("closing shared view asked to discard shared source")
	}
	m.focusGroup(m.allGroups()[0].id)
	m.closeDocumentTab(first)
	if !m.closePrompt || len(m.closePlans()) != 1 {
		t.Fatal("last dirty view skipped save confirmation")
	}
	m.cancelClose()
	for range maxEditorGroups + 3 {
		m.splitEditor(true)
	}
	if len(m.allGroups()) != maxEditorGroups || !strings.Contains(m.message, "9 editor") {
		t.Fatal("group/large-page state unbounded")
	}
}

func TestEditorGroupCloseSavesOnlyUniqueDirtyDocuments(t *testing.T) {
	m := tabModel(t)
	shared := m.current()
	m.splitEditor(false)
	right := m.findGroup(m.groups.active)
	m.focusTab(m.docs[1])
	unique := m.current()
	// Move the resource out of the first group, leaving one unique view.
	left := m.allGroups()[0]
	m.focusGroup(left.id)
	m.focusTab(unique)
	m.closeDocumentTab(unique)
	m.focusGroup(right.id)
	m.replaceSelection(unique, "unique ")
	m.replaceSelection(shared, "shared ")
	m.requestCloseGroup(right.id)
	if !m.closePrompt || m.closeGroupID != right.id || len(m.closePlans()) != 1 || m.closePlans()[0].document != unique {
		t.Fatal("group close would save/discard unrelated shared edits")
	}
	m.cancelClose()
	if m.findGroup(right.id) == nil {
		t.Fatal("cancel closed group")
	}
	m.requestCloseGroup(right.id)
	m.finishClose(nil)
	if m.findGroup(right.id) != nil || m.ownsDocument(unique) || !m.ownsDocument(shared) || !shared.dirty() {
		t.Fatal("group discard lost a resource used by another view")
	}
}

func TestEditorGroupDelayedOpenRetainsOriginAndRejectsClosedGroup(t *testing.T) {
	for _, closed := range []bool{false, true} {
		t.Run(string(rune('0'+intBool(closed))), func(t *testing.T) {
			m := testModel(t)
			m.splitEditor(false)
			origin := m.groups.active
			path := filepath.Join(m.workspace, "group-open.txt")
			os.WriteFile(path, []byte("real group file\n"), 0600)
			mailbox := make(chan func(), 32)
			entered, release := make(chan struct{}), make(chan struct{})
			stop := m.startFileOpens(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, func(ctx context.Context, p string) (*document, error) {
				close(entered)
				<-release
				return loadDocument(ctx, p)
			})
			t.Cleanup(stop)
			m.requestOpen(context.Background(), path, nil)
			<-entered
			m.focusGroup(m.allGroups()[0].id)
			current := m.current()
			if closed {
				m.dropEditorGroup(origin)
			}
			close(release)
			saveAck(t, mailbox)()
			saveAck(t, mailbox)()
			if m.current() != current {
				t.Fatal("old group read stole new focus")
			}
			if closed {
				if m.findDocument(path) != nil {
					t.Fatal("closed group receipt adopted an orphan document")
				}
			} else {
				g := m.findGroup(origin)
				d := m.findDocument(path)
				if d == nil || !containsDocument(g.docs, d) {
					t.Fatal("background open lost originating group")
				}
			}
		})
	}
}
func intBool(v bool) int {
	if v {
		return 1
	}
	return 0
}

func TestEditorGroupCapturedToolbarRejectsChangedMembership(t *testing.T) {
	m := tabModel(t)
	m.splitEditor(false)
	g := m.findGroup(m.groups.active)
	m.groups.pressed = g
	m.groups.pressGeneration = g.generation
	m.focusTab(m.docs[1])
	if m.acceptGroupAction(g) || !strings.Contains(m.message, "changed during click") || len(m.allGroups()) != 2 {
		t.Fatal("captured toolbar action accepted a different group review")
	}
	m.groups.pressed = g
	m.groups.pressGeneration = g.generation
	if !m.acceptGroupAction(g) {
		t.Fatal("unchanged reviewed toolbar action rejected")
	}
}

func TestEditorGroupsLargeViewsShareIndexButOwnPagesAndLifetime(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "large.txt")
	os.WriteFile(path, []byte(strings.Repeat("page line\n", 2<<20)), 0600)
	d, err := loadDocument(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	m := &model{workspace: root, docs: []*document{d}, active: 0}
	t.Cleanup(m.closeDocuments)
	m.splitEditor(false)
	gs := m.allGroups()
	base := d.largeBase
	other := gs[1].views[d].large
	if other == base || other.file != base.file {
		t.Fatal("split duplicated index or shared mutable paging state")
	}
	m.navigateLarge(d, 0, 128, true)
	m.captureActiveView()
	m.focusGroup(gs[0].id)
	if d.large.byteMode || d.large.byteOffset != 0 {
		t.Fatal("byte navigation leaked into another view")
	}
	m.requestCloseGroup(gs[0].id)
	if base.ctx.Err() != nil || other.ctx.Err() != nil || !m.ownsDocument(d) {
		t.Fatal("closing owner view cancelled a surviving reader")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	w, err := other.file.Bytes(ctx, 128, 64)
	if err != nil || w.Text == "" {
		t.Fatal("shared file closed while another view owns it", err)
	}
	m.closeDocumentTab(d)
	if base.ctx.Err() == nil || other.ctx.Err() == nil {
		t.Fatal("last close did not cancel all view readers")
	}
}
