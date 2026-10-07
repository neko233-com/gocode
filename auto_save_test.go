package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ui "github.com/neko233-com/godesktop"
)

// Scheduling uses a controlled UI clock; the unchanged save actor writes real
// files and its receipt can be held while newer UI edits arrive.
func autoSaveModel(t *testing.T, mode string) (*model, chan func(), time.Time) {
	t.Helper()
	m := testModel(t)
	t.Cleanup(m.closeDocuments)
	parent, cancelParent := context.WithCancel(context.Background())
	request, cancelRequest := context.WithCancel(parent)
	t.Cleanup(cancelParent)
	t.Cleanup(cancelRequest)
	m.autoSave = autoSaveState{
		config: autoSaveConfig{Mode: mode, DelayMS: 1000},
		parent: parent, request: request, cancel: cancelRequest,
		arm: func(time.Time) {},
	}
	return m, saveActor(t, m), time.Now().Add(time.Hour)
}

func autoSaveOpen(t *testing.T, m *model, name, content string) *document {
	t.Helper()
	path := filepath.Join(m.workspace, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	m.open(path)
	if d := m.current(); d == nil || d.path != path || d.buffer == nil {
		t.Fatal("real editable fixture did not open", m.message)
	}
	return m.current()
}

func autoSaveDisk(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestAutoSaveDelayDebouncesUnicodeCRLFAndDuplicateTicks(t *testing.T) {
	m, mailbox, now := autoSaveModel(t, "afterDelay")
	d := autoSaveOpen(t, m, "世界😀.go", "package main\r\n// original\r\n")
	original := autoSaveDisk(t, d.path)
	text(m, "// 第一😀")
	m.autoSaveChanged(d, now)
	firstDue := now.Add(time.Second)
	m.autoSaveTick(firstDue.Add(-time.Nanosecond))
	if m.saveBusy || len(m.saveJobs) != 0 || autoSaveDisk(t, d.path) != original {
		t.Fatal("delay saved before its deadline")
	}
	text(m, "第二")
	m.autoSaveChanged(d, now.Add(700*time.Millisecond))
	want := d.buffer.Text()
	m.autoSaveTick(firstDue)
	if m.saveBusy || autoSaveDisk(t, d.path) != original {
		t.Fatal("earlier deadline ignored the latest edit")
	}
	m.autoSaveTick(now.Add(1700 * time.Millisecond))
	if !m.saveBusy {
		t.Fatal("due dirty file was not accepted")
	}
	saveAck(t, mailbox)()
	if d.dirty() || d.saveID != 1 || autoSaveDisk(t, d.path) != want || !strings.Contains(want, "第一😀第二") || !strings.Contains(want, "\r\n") {
		t.Fatal("latest Unicode/CRLF snapshot was not saved", d.saveID)
	}
	for range 4 {
		m.autoSaveTick(now.Add(time.Minute))
	}
	if m.saveBusy || len(m.saveJobs) != 0 || d.saveID != 1 || len(m.autoSave.pending) != 0 {
		t.Fatal("clean version was saved again")
	}
}

func TestAutoSaveHeldReceiptPreservesNewerEditsThenSavesLatest(t *testing.T) {
	m, mailbox, now := autoSaveModel(t, "afterDelay")
	d := m.current()
	text(m, "first😀")
	m.autoSaveChanged(d, now)
	first := d.buffer.Text()
	m.autoSaveTick(now.Add(time.Second))
	held := saveAck(t, mailbox)
	if autoSaveDisk(t, d.path) != first || !d.dirty() || d.saveID != 0 {
		t.Fatal("worker mutated UI before its receipt")
	}
	text(m, "newer世界")
	m.autoSaveChanged(d, now.Add(1200*time.Millisecond))
	want := d.buffer.Text()
	m.autoSaveTick(now.Add(10 * time.Second))
	if len(m.saveJobs) != 0 {
		t.Fatal("automatic save stacked behind its own active write")
	}
	held()
	if !d.dirty() || d.saveID != 0 || d.buffer.Text() != want || m.saveBusy {
		t.Fatal("old receipt marked newer edits clean")
	}
	m.autoSaveTick(now.Add(2200 * time.Millisecond))
	saveAck(t, mailbox)()
	if d.dirty() || d.saveID != 1 || autoSaveDisk(t, d.path) != want {
		t.Fatal("newer snapshot did not save after the old receipt")
	}
}

func TestAutoSaveFocusChangeSavesPreviousTabAndTerminalBlur(t *testing.T) {
	m, mailbox, now := autoSaveModel(t, "onFocusChange")
	first := m.current()
	text(m, "previous tab😀")
	m.autoSaveChanged(first, now)
	m.observeAutoSaveFocus(now)
	m.autoSaveTick(now.Add(time.Hour))
	if m.saveBusy {
		t.Fatal("focus mode saved while the editor retained focus")
	}
	second := autoSaveOpen(t, m, "focus-second.go", "package second\n")
	m.observeAutoSaveFocus(now.Add(time.Second))
	m.autoSaveTick(now.Add(time.Second))
	saveAck(t, mailbox)()
	if first.dirty() || first.saveID != 1 || m.current() != second || !strings.Contains(autoSaveDisk(t, first.path), "previous tab😀") {
		t.Fatal("tab blur did not save the background source")
	}
	text(m, "terminal blur世界")
	m.autoSaveChanged(second, now.Add(2*time.Second))
	m.observeAutoSaveFocus(now.Add(2 * time.Second))
	m.focusTerminal()
	m.observeAutoSaveFocus(now.Add(3 * time.Second))
	m.autoSaveTick(now.Add(3 * time.Second))
	saveAck(t, mailbox)()
	if second.dirty() || second.saveID != 1 || !m.terminalFocused || !strings.Contains(autoSaveDisk(t, second.path), "terminal blur世界") {
		t.Fatal("terminal focus did not save the editor source")
	}
}

func TestAutoSaveWindowChangeRequiresActualWindowEvent(t *testing.T) {
	for _, mode := range []string{"onWindowChange", "onFocusChange"} {
		t.Run(mode, func(t *testing.T) {
			m, mailbox, now := autoSaveModel(t, mode)
			first := m.current()
			m.replaceSelection(first, "first")
			m.autoSaveChanged(first, now)
			second := autoSaveOpen(t, m, "window-second.go", "package second\n")
			m.replaceSelection(second, "second😀")
			m.autoSaveChanged(second, now)
			m.input(nil, ui.InputEvent{Kind: ui.WindowFocusChanged, Focused: true})
			m.input(nil, ui.InputEvent{Kind: ui.InputCancelled})
			m.autoSaveTick(now.Add(time.Hour))
			if m.saveBusy || first.saveID != 0 || second.saveID != 0 {
				t.Fatal("pointer/keyboard cancellation was mistaken for window blur")
			}
			m.input(nil, ui.InputEvent{Kind: ui.WindowFocusChanged, Focused: false})
			for range 2 {
				m.autoSaveTick(now.Add(time.Hour))
				saveAck(t, mailbox)()
			}
			if first.dirty() || second.dirty() || first.saveID != 1 || second.saveID != 1 {
				t.Fatal("actual window blur did not save all dirty sources")
			}
			m.input(nil, ui.InputEvent{Kind: ui.WindowFocusChanged, Focused: false})
			m.autoSaveTick(now.Add(2 * time.Hour))
			if m.saveBusy || first.saveID != 1 || second.saveID != 1 {
				t.Fatal("duplicate window state rewrote clean sources")
			}
		})
	}
}

func TestAutoSaveFailureLatchesVersionAndPreservesDisk(t *testing.T) {
	for _, kind := range []string{"external", "read-only", "missing"} {
		t.Run(kind, func(t *testing.T) {
			m, mailbox, now := autoSaveModel(t, "afterDelay")
			d := m.current()
			original := autoSaveDisk(t, d.path)
			text(m, "local edits😀")
			m.autoSaveChanged(d, now)
			local, version := d.buffer.Text(), d.buffer.Version()
			wantDisk := original
			switch kind {
			case "external":
				wantDisk = "external editor 世界\n"
				if err := os.WriteFile(d.path, []byte(wantDisk), 0600); err != nil {
					t.Fatal(err)
				}
			case "read-only":
				if err := os.Chmod(d.path, 0444); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(d.path, 0644) })
			case "missing":
				if err := os.Remove(d.path); err != nil {
					t.Fatal(err)
				}
			}
			m.autoSaveTick(now.Add(time.Second))
			saveAck(t, mailbox)()
			entry := m.autoSave.pending[d]
			if entry == nil || entry.failedVersion != version || !d.dirty() || d.buffer.Text() != local || d.saveID != 0 || !strings.Contains(m.message, "Auto Save") {
				t.Fatal("failure discarded dirty source or lacked a failure latch", m.message)
			}
			message := m.message
			for range 4 {
				m.autoSaveChanged(d, now.Add(time.Minute))
				m.autoSaveTick(now.Add(time.Hour))
			}
			if m.saveBusy || len(m.saveJobs) != 0 || m.message != message {
				t.Fatal("unchanged failed version retried automatically")
			}
			if kind == "missing" {
				if _, err := os.Stat(d.path); !os.IsNotExist(err) {
					t.Fatal("automatic save recreated a deleted path", err)
				}
			} else if autoSaveDisk(t, d.path) != wantDisk {
				t.Fatal("failure overwrote disk source")
			}
			leftovers, err := filepath.Glob(filepath.Join(m.workspace, ".gocode-save-*"))
			if err != nil || len(leftovers) != 0 {
				t.Fatal("failed save left temporary files", leftovers, err)
			}
		})
	}
}

func TestAutoSaveOffClosedAndReopenedIdentity(t *testing.T) {
	m, mailbox, now := autoSaveModel(t, "afterDelay")
	d := m.current()
	original := autoSaveDisk(t, d.path)
	text(m, "discarded local edit")
	m.autoSaveChanged(d, now)
	m.configureAutoSave(defaultAutoSaveConfig())
	m.autoSaveTick(now.Add(time.Hour))
	if m.saveBusy || len(m.autoSave.pending) != 0 || !d.dirty() || autoSaveDisk(t, d.path) != original {
		t.Fatal("disabled policy accepted a stale deadline")
	}
	m.configureAutoSave(autoSaveConfig{Mode: "afterDelay", DelayMS: 1000})
	m.autoSaveChanged(d, now)
	path := d.path
	m.removeTab(m.active) // An explicit discard closes this fixture identity.
	m.autoSaveTick(now.Add(time.Second))
	if m.saveBusy || autoSaveDisk(t, path) != original {
		t.Fatal("closed dirty identity was written")
	}
	m.open(path)
	reopened := m.current()
	if reopened == nil || reopened == d || reopened.dirty() || reopened.buffer.Text() != original {
		t.Fatal("reopen reused the discarded buffer")
	}
	text(m, "new identity😀")
	m.autoSaveChanged(reopened, now.Add(2*time.Second))
	m.autoSaveTick(now.Add(time.Second))
	if m.saveBusy {
		t.Fatal("old deadline saved a newly opened document")
	}
	m.autoSaveTick(now.Add(3 * time.Second))
	saveAck(t, mailbox)()
	if reopened.dirty() || reopened.saveID != 1 || !strings.Contains(autoSaveDisk(t, path), "new identity😀") {
		t.Fatal("new identity did not save at its own deadline")
	}
}

func TestAutoSaveShutdownRejectsAlreadyDispatchedTimer(t *testing.T) {
	m, _, _ := autoSaveModel(t, "afterDelay")
	d := m.current()
	original := autoSaveDisk(t, d.path)
	m.autoSave.config.DelayMS = 100
	text(m, "keep unsaved on shutdown")
	mailbox := make(chan func(), 1)
	stop := m.startAutoSaveActor(context.Background(), func(fn func()) bool { mailbox <- fn; return true })
	var held func()
	select {
	case held = <-mailbox:
	case <-time.After(5 * time.Second):
		stop()
		t.Fatal("timer did not dispatch its owned receipt")
	}
	stop()
	held()
	if m.saveBusy || !d.dirty() || d.saveID != 0 || m.autoSave.running || m.autoSave.arm != nil || len(m.autoSave.pending) != 0 || autoSaveDisk(t, d.path) != original {
		t.Fatal("shutdown allowed an obsolete dispatched callback to write")
	}
}

func TestAutoSaveUntitledLargeAndServiceSizePolicy(t *testing.T) {
	m, mailbox, now := autoSaveModel(t, "afterDelay")
	m.newTextFile()
	untitled := m.current()
	text(m, "unnamed😀")
	m.autoSaveChanged(untitled, now)
	m.autoSaveTick(now.Add(time.Hour))
	if m.saveBusy || !untitled.dirty() || m.autoSave.pending[untitled] != nil {
		t.Fatal("automatic save accepted an untitled document")
	}
	largePath := filepath.Join(m.workspace, "read-only-large.txt")
	f, err := os.Create(largePath)
	if err != nil {
		t.Fatal(err)
	}
	err = f.Truncate(editableFileLimit + 1)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatal(err, closeErr)
	}
	m.open(largePath)
	large := m.current()
	if large == nil || large.buffer != nil || large.large == nil {
		t.Fatal("large real fixture was not read-only", m.message)
	}
	m.autoSaveChanged(large, now)
	m.autoSaveTick(now.Add(time.Hour))
	if m.saveBusy || m.autoSave.pending[large] != nil {
		t.Fatal("automatic save accepted a large-file browser")
	}
	// Editable files above the separate 2 MiB service cap still save normally.
	d := autoSaveOpen(t, m, "editable-over-service-limit.txt", strings.Repeat("line\r\n", (2<<20)/6+1))
	if d.serviceEligible() {
		t.Fatal("real fixture failed to exceed the language-service cap")
	}
	text(m, "world😀")
	m.autoSaveChanged(d, now)
	m.autoSaveTick(now.Add(time.Second))
	saveAck(t, mailbox)()
	if d.dirty() || d.saveID != 1 || !strings.Contains(autoSaveDisk(t, d.path), "world😀") {
		t.Fatal("Auto Save incorrectly reused the smaller service cap")
	}
}

func TestAutoSaveSlotsBoundAndManualQueuePriority(t *testing.T) {
	t.Run("slots", func(t *testing.T) {
		m, _, now := autoSaveModel(t, "afterDelay")
		var last *document
		for i := 0; i <= maxAutoSaveDocuments; i++ {
			last = autoSaveOpen(t, m, "slot-"+time.Unix(int64(i), 0).Format("150405")+".txt", "original\n")
			m.replaceSelection(last, "dirty")
			m.autoSaveChanged(last, now)
		}
		if len(m.autoSave.pending) != maxAutoSaveDocuments || m.autoSave.pending[last] != nil || !last.dirty() || !strings.Contains(m.message, "128") {
			t.Fatal("dirty scheduling slots were not bounded", len(m.autoSave.pending), m.message)
		}
		closed := m.docs[1]
		m.removeTab(1)
		if m.autoSave.pending[closed] != nil || len(m.autoSave.pending) != maxAutoSaveDocuments-1 {
			t.Fatal("closing a dirty fixture did not immediately release its slot")
		}
		m.autoSaveChanged(last, now)
		if m.autoSave.pending[last] == nil || len(m.autoSave.pending) != maxAutoSaveDocuments {
			t.Fatal("a freed slot remained unavailable until an old timer tick")
		}
	})
	t.Run("manual", func(t *testing.T) {
		m, mailbox, now := autoSaveModel(t, "afterDelay")
		automatic := m.current()
		text(m, "automatic pending")
		m.autoSaveChanged(automatic, now)
		manual := autoSaveOpen(t, m, "manual-first.go", "package manual\n")
		text(m, "manual first")
		m.requestSave(context.Background(), []*document{manual}, nil)
		held := saveAck(t, mailbox)
		text(m, "manual latest")
		m.requestSave(context.Background(), []*document{manual}, nil)
		m.autoSaveTick(now.Add(time.Hour))
		if len(m.saveJobs) != 1 || !automatic.dirty() || automatic.saveID != 0 {
			t.Fatal("automatic job joined or displaced the manual queue")
		}
		held()
		m.autoSaveTick(now.Add(time.Hour))
		if len(m.saveJobs) != 0 || !automatic.dirty() || automatic.saveID != 0 {
			t.Fatal("automatic save bypassed the next active manual write")
		}
		saveAck(t, mailbox)()
		if manual.dirty() || !automatic.dirty() || m.saveBusy {
			t.Fatal("manual queue did not drain before automatic source")
		}
		m.autoSaveTick(now.Add(time.Hour))
		saveAck(t, mailbox)()
		if automatic.dirty() || automatic.saveID != 1 || !strings.Contains(autoSaveDisk(t, automatic.path), "automatic pending") {
			t.Fatal("automatic work did not resume after the manual queue")
		}
	})
}

func TestAutoSaveMissingWriterDisarmsExpiredDeadline(t *testing.T) {
	m, _, now := autoSaveModel(t, "afterDelay")
	d := m.current()
	text(m, "unavailable writer")
	m.autoSaveChanged(d, now)
	writer := m.requestSave
	m.requestSave = nil
	var armed time.Time
	m.autoSave.arm = func(deadline time.Time) { armed = deadline }
	m.autoSaveTick(now.Add(time.Hour))
	if !armed.IsZero() || m.saveBusy || !d.dirty() {
		t.Fatal("missing writer repeatedly armed an expired deadline", armed)
	}
	// Restoring the writer and explicitly arming resumes the preserved dirty work.
	m.requestSave = writer
	m.armAutoSave(now.Add(time.Hour))
	if armed.IsZero() {
		t.Fatal("restored writer could not resume dirty scheduling")
	}
}

func TestAutoSaveOwnedModalBlurDoesNotCreateFocusSave(t *testing.T) {
	for _, mode := range []string{"onFocusChange", "onWindowChange", "afterDelay"} {
		for _, modal := range []string{"file", "close", "reload", "history"} {
			t.Run(mode+"/"+modal, func(t *testing.T) {
				m, mailbox, now := autoSaveModel(t, mode)
				d := m.current()
				original := autoSaveDisk(t, d.path)
				text(m, "preserved through modal cancel")
				m.autoSaveChanged(d, now)
				m.autoSaveWindowFocus(true, now)
				switch modal {
				case "file":
					m.fileActions.busy = true
				case "close":
					m.beginClose(d)
				case "reload":
					m.reloadPrompt = d
				case "history":
					m.history.prompt = &historyPrompt{}
				}
				m.autoSaveWindowFocus(false, now.Add(time.Second))
				m.autoSaveTick(now.Add(time.Hour))
				if m.saveBusy || autoSaveDisk(t, d.path) != original || !d.dirty() {
					t.Fatal("owned modal activation wrote through its confirmation")
				}
				m.fileActions.busy = false
				m.cancelClose()
				m.reloadPrompt, m.history.prompt = nil, nil
				m.editing = true
				m.autoSaveWindowFocus(true, now.Add(2*time.Second))
				m.autoSaveTick(now.Add(2 * time.Hour))
				if mode == "afterDelay" {
					saveAck(t, mailbox)()
					if d.dirty() || d.saveID != 1 {
						t.Fatal("pre-existing delay did not resume after modal cancel")
					}
				} else if m.saveBusy || d.saveID != 0 || !d.dirty() || autoSaveDisk(t, d.path) != original {
					t.Fatal("modal cancellation triggered an unrequested focus save")
				}
			})
		}
	}
}

func TestAutoSaveFocusChangeBetweenGroupsOfSameDocument(t *testing.T) {
	m, mailbox, now := autoSaveModel(t, "onFocusChange")
	d := m.current()
	m.ensureGroups()
	firstGroup := m.groups.active
	text(m, "shared source😀")
	m.autoSaveChanged(d, now)
	m.observeAutoSaveFocus(now)
	secondGroup := m.splitEditorGroup(firstGroup, false, true)
	if secondGroup == nil {
		t.Fatal("real shared editor group was not created")
	}
	m.focusGroup(secondGroup.id)
	m.observeAutoSaveFocus(now.Add(time.Second))
	if m.current() != d || m.groups.active == firstGroup {
		t.Fatal("group switch did not retain the same document")
	}
	m.autoSaveTick(now.Add(time.Second))
	saveAck(t, mailbox)()
	if d.dirty() || d.saveID != 1 || len(m.docs) != 1 || !strings.Contains(autoSaveDisk(t, d.path), "shared source😀") {
		t.Fatal("same-document editor blur was ignored or duplicated the resource")
	}
	m.focusGroup(firstGroup)
	m.observeAutoSaveFocus(now.Add(2 * time.Second))
	m.autoSaveTick(now.Add(time.Hour))
	if m.saveBusy || d.saveID != 1 {
		t.Fatal("switching clean shared views rewrote the file")
	}
}

func TestAutoSaveSettingsDelayInputAndInvalidCommit(t *testing.T) {
	m, _, _ := autoSaveModel(t, "off")
	d := m.current()
	original := d.buffer.Text()
	persisted := 0
	m.autoSave.persist = func(autoSaveConfig) { persisted++ }
	m.autoSave.delayFocused, m.autoSave.delayDraft = true, "1000"
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'A', Modifiers: ui.ModifierControl})
	if m.autoSave.delayDraft != "" {
		t.Fatal("Ctrl+A did not replace the delay draft")
	}
	text(m, "2500")
	m.input(nil, ui.InputEvent{Kind: ui.Character, Key: 'x'})
	m.input(nil, ui.InputEvent{Kind: ui.Character, Key: '9', Modifiers: ui.ModifierControl})
	if m.autoSave.delayDraft != "2500" || d.buffer.Text() != original {
		t.Fatal("delay typing accepted non-digits or changed source text")
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 13})
	if m.autoSaveConfig() != (autoSaveConfig{Mode: "off", DelayMS: 2500}) || m.autoSave.delayFocused || persisted != 1 {
		t.Fatal("Enter failed to validate and persist the delay", m.autoSaveConfig())
	}
	want := m.autoSaveConfig()
	for _, draft := range []string{"", "0", "99", "600001", "9999999", "invalid"} {
		m.autoSave.delayFocused, m.autoSave.delayDraft = true, draft
		m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 13})
		if m.autoSaveConfig() != want || m.autoSave.delayFocused || persisted != 1 || m.message == "" {
			t.Fatal("invalid Enter changed the persistent delay", draft, m.autoSaveConfig())
		}
	}
	m.autoSave.delayFocused, m.autoSave.delayDraft = true, ""
	text(m, "1234567890")
	if m.autoSave.delayDraft != "1234567" {
		t.Fatal("delay draft exceeded its seven-digit bound", m.autoSave.delayDraft)
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 8})
	if m.autoSave.delayDraft != "123456" {
		t.Fatal("Backspace did not edit the delay draft")
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 27})
	if m.autoSave.delayFocused || m.autoSaveConfig() != want || persisted != 1 || d.buffer.Text() != original {
		t.Fatal("Escape saved the draft or changed source text")
	}
	m.configureAutoSave(want)
	if persisted != 1 {
		t.Fatal("identical configuration was republished")
	}
}

func TestAutoSaveSettingsDoesNotCaptureModalKeyboard(t *testing.T) {
	m, _, _ := autoSaveModel(t, "off")
	d := m.current()
	text(m, "dirty source")
	m.autoSave.delayFocused, m.autoSave.delayDraft = true, "2500"
	m.beginClose(d)
	saves := 0
	m.saveForClose = func() { saves++ }
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 13})
	if saves != 1 || m.autoSaveConfig().DelayMS != 1000 || !d.dirty() {
		t.Fatal("delay editor consumed Enter from the real close confirmation")
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 27})
	if m.closePrompt || !m.autoSave.delayFocused || !d.dirty() {
		t.Fatal("Escape failed to cancel close and restore the pending delay draft")
	}
	m.reloadPrompt, m.reloadBusy = d, true
	d.reloadID = 7
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 27})
	if m.reloadPrompt != nil || m.reloadBusy || d.reloadID != 0 || m.autoSaveConfig().DelayMS != 1000 || !d.dirty() {
		t.Fatal("delay editor captured Escape from a pending Revert read")
	}
}

func TestAutoSaveSettingsShutdownDrainsLatestConfig(t *testing.T) {
	m, _, _ := autoSaveModel(t, "off")
	path := filepath.Join(t.TempDir(), "gocode", "autosave.json")
	mailbox := make(chan func(), 8)
	stop := m.startAutoSaveSettings(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, path)
	configs := []autoSaveConfig{{"afterDelay", 100}, {"onFocusChange", 2500}, {"onWindowChange", 600000}, {"off", 1000}}
	for range 3 {
		for _, config := range configs {
			m.configureAutoSave(config)
		}
	}
	stop()
	if got, err := readAutoSaveConfig(path); err != nil || got != configs[len(configs)-1] || m.autoSave.persist != nil {
		t.Fatal("shutdown failed to persist the newest coalesced settings", got, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != "autosave.json" {
		t.Fatal("settings shutdown left temporary files", entries, err)
	}
	select {
	case callback := <-mailbox:
		callback()
		t.Fatal("real settings persistence reported a failure", m.message)
	default:
	}
}
