package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	workspaceSearch "github.com/neko233-com/gocode/internal/search"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func replacementModel(t *testing.T) (*model, chan func(), chan func()) {
	t.Helper()
	m := testModel(t)
	if err := os.WriteFile(filepath.Join(m.workspace, "second.txt"), []byte("main 😀\r\nmain\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	searchMailbox := searchActor(t, m, nil)
	saveMailbox := saveActor(t, m)
	m.search.query = workspaceSearch.Query{Text: "main", Include: "**/*.{go,txt}", CaseSensitive: true}
	m.search.replaceShown = true
	m.search.replacement = "界😀"
	m.showSearch(nil)
	return m, searchMailbox, saveMailbox
}
func replacementPreview(t *testing.T, m *model, mailbox chan func()) {
	t.Helper()
	m.search.submit()
	saveAck(t, mailbox)()
	m.previewReplacement(nil)
	saveAck(t, mailbox)()
	if m.search.replacePlan == nil {
		t.Fatal("preview failed", m.search.replaceStatus)
	}
}
func TestWorkspaceReplacePublishesAtomicBuffersThenRealSavesAndUndo(t *testing.T) {
	m, mailbox, saves := replacementModel(t)
	d := m.current()
	m.replaceSelection(d, "// unsaved main\n")
	before := d.buffer.Text()
	diskBefore, _ := os.ReadFile(d.path)
	replacementPreview(t, m, mailbox)
	plan := m.search.replacePlan
	if plan.Count != 5 || len(plan.Files) != 2 {
		t.Fatal("complete unsaved/closed result plan missing", plan.Count)
	}
	if raw, _ := os.ReadFile(d.path); string(raw) != string(diskBefore) || d.buffer.Text() != before {
		t.Fatal("preview wrote a document")
	}
	m.applyReplacement(nil)
	held := saveAck(t, mailbox)
	if d.buffer.Text() != before || len(m.docs) != 1 {
		t.Fatal("worker disk preflight mutated the UI")
	}
	held()
	if strings.Contains(d.buffer.Text(), "main") || !strings.Contains(d.buffer.Text(), "unsaved 界😀") || m.current() != d || len(m.docs) != 2 || !m.search.replaceSaving {
		t.Fatal("all-buffer commit/focus did not preserve unsaved content")
	}
	saveAck(t, saves)()
	if m.search.replaceSaving || d.buffer.Dirty() || !strings.Contains(m.search.replaceStatus, "Replaced 5") {
		t.Fatal(m.search.replaceStatus)
	}
	closed := m.findDocument(filepath.Join(m.workspace, "second.txt"))
	if closed == nil || closed.buffer.Dirty() || closed.buffer.Text() != "界😀 😀\r\n界😀\r\n" {
		t.Fatal("closed file transaction/save lost CRLF")
	}
	for _, doc := range []*document{d, closed} {
		raw, _ := os.ReadFile(doc.path)
		if string(raw) != doc.buffer.Text() {
			t.Fatal("real disk acknowledgement differs")
		}
	}
	d.buffer.Undo()
	if d.buffer.Text() != before || !d.buffer.Dirty() {
		t.Fatal("workspace replacement undo erased earlier unsaved changes")
	}
}

func TestWorkspaceReplaceRejectsHeldReceiptsAndDiskChanges(t *testing.T) {
	for _, scenario := range []string{"query", "edit", "caret", "reopened", "disk"} {
		t.Run(scenario, func(t *testing.T) {
			m, mailbox, _ := replacementModel(t)
			d := m.current()
			before := d.buffer.Text()
			replacementPreview(t, m, mailbox)
			if scenario == "disk" {
				os.WriteFile(filepath.Join(m.workspace, "second.txt"), []byte("external main"), 0600)
			}
			m.applyReplacement(nil)
			held := saveAck(t, mailbox)
			switch scenario {
			case "query":
				m.search.query.Text = "package"
				m.searchChanged(nil)
			case "edit":
				m.replaceSelection(d, "new ")
				before = d.buffer.Text()
			case "caret":
				d.buffer.SetSelection(textbuffer.Selection{Anchor: textbuffer.Position{Line: 1}, Active: textbuffer.Position{Line: 1}})
			case "reopened":
				buffer, _ := textbuffer.New(before)
				reopened := &document{path: d.path, buffer: buffer, diskHash: d.diskHash, diskKnown: true}
				m.docs[0] = reopened
				m.rememberDocument(d.path, reopened)
			}
			held()
			if d.buffer.Text() != before || m.search.replaceSaving || len(m.docs) != 1 {
				t.Fatal("stale replacement mutated/adopted documents", m.search.replaceStatus)
			}
			if raw, _ := os.ReadFile(d.path); string(raw) != before && scenario != "edit" {
				t.Fatal("stale replacement wrote disk")
			}
		})
	}
}

func TestWorkspaceReplacePreflightsAfterOpenNotifications(t *testing.T) {
	m, mailbox, _ := replacementModel(t)
	d := m.current()
	before := d.buffer.Text()
	replacementPreview(t, m, mailbox)
	m.onDocument = func(kind string, _ *document, _ textbuffer.ChangeEvent) {
		if kind == "open" {
			d.buffer.ReplaceSelection("callback ")
		}
	}
	m.applyReplacement(nil)
	saveAck(t, mailbox)()
	if d.buffer.Text() != "callback "+before || m.search.replaceSaving || !strings.Contains(m.search.replaceStatus, "aborted") {
		t.Fatal("notification interleaved with all-buffer commit")
	}
	if other := m.findDocument(filepath.Join(m.workspace, "second.txt")); other == nil || other.buffer.Dirty() {
		t.Fatal("resolving an unopened file changed source")
	}
}

func TestWorkspaceReplaceLateSaveConflictPreservesUndoableDirtySource(t *testing.T) {
	m, mailbox, saves := replacementModel(t)
	replacementPreview(t, m, mailbox)
	originalSave := m.requestSave
	var delayed func()
	m.requestSave = func(ctx context.Context, documents []*document, done func(error)) {
		delayed = func() { originalSave(ctx, documents, done) }
	}
	m.applyReplacement(nil)
	saveAck(t, mailbox)()
	if delayed == nil {
		t.Fatal("replacement did not publish a save request")
	}
	path := filepath.Join(m.workspace, "second.txt")
	os.WriteFile(path, []byte("external changed"), 0600)
	delayed()
	saveAck(t, saves)()
	other := m.findDocument(path)
	if other == nil || !other.buffer.Dirty() || !strings.Contains(m.search.replaceStatus, "1 unsaved") {
		t.Fatal("late save conflict discarded replacement", m.search.replaceStatus)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "external changed" {
		t.Fatal("late external write was overwritten")
	}
	if _, ok := other.buffer.Undo(); !ok || other.buffer.Text() != "main 😀\r\nmain\r\n" {
		t.Fatal("failed save lost replacement undo")
	}
}

func TestReplacementFieldDoesNotRescanOrEditDocument(t *testing.T) {
	m, mailbox, _ := replacementModel(t)
	m.search.submit()
	saveAck(t, mailbox)()
	d := m.current()
	source, generation, count := d.buffer.Text(), m.search.generation, len(m.search.report.Matches)
	m.search.focus = 3
	m.search.caret = 0
	m.search.replacement = ""
	text(m, `$1\n界`)
	if m.search.replacement != `$1\n界` || m.search.generation != generation || len(m.search.report.Matches) != count || d.buffer.Text() != source {
		t.Fatal("replacement input invalidated results or edited source")
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 9})
	if m.search.focus != 1 {
		t.Fatal("replace-to-include Tab order is incorrect")
	}
}

func TestReplacementCapturedPressDoesNotApplyAChangedPreview(t *testing.T) {
	m, mailbox, _ := replacementModel(t)
	replacementPreview(t, m, mailbox)
	d := m.current()
	before := d.buffer.Text()
	m.search.replacePressed = true
	m.search.replaceCapture = m.search.replacePlan
	m.search.replacePressGeneration = m.search.replaceGeneration
	m.search.replacement = "changed"
	m.replacementChanged()
	m.previewReplacement(nil)
	saveAck(t, mailbox)()
	m.replacementButton(nil)
	if m.search.replaceSaving || m.search.replaceBusy || d.buffer.Text() != before || !strings.Contains(m.search.replaceStatus, "changed during click") {
		t.Fatal("captured old review applied the new preview")
	}
}
