package main

import (
	"math"
	"testing"

	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func TestTabRevealClampAndManualOffset(t *testing.T) {
	for _, test := range []struct {
		offset, start, end, width, total float32
		reveal                           bool
		want                             float32
	}{
		{0, 800, 940, 300, 1000, true, 640},
		{640, 120, 240, 300, 1000, true, 120},
		{200, 400, 1200, 300, 1300, true, 400},
		{500, 0, 120, 300, 1000, false, 500},
		{900, 0, 120, 300, 1000, false, 700},
		{900, 0, 120, 1500, 1000, false, 0},
		{float32(math.NaN()), 0, 120, 300, 1000, false, 0},
	} {
		if got := tabOffset(test.offset, test.start, test.end, test.width, test.total, test.reveal); got != test.want {
			t.Fatalf("offset case %+v got %f", test, got)
		}
	}
}

func tabModel(t *testing.T) *model {
	t.Helper()
	m := &model{active: 0}
	for _, name := range []string{"first.go", "second.go", "third.go", "fourth.go"} {
		b, err := textbuffer.New("package main\n")
		if err != nil {
			t.Fatal(err)
		}
		m.docs = append(m.docs, &document{path: name, buffer: b})
	}
	return m
}

func TestHeldControlTabUsesFrozenMRUAndReleaseCommits(t *testing.T) {
	m := tabModel(t)
	for _, i := range []int{0, 2, 1, 3} {
		m.focusTab(m.docs[i])
	}
	key := func(shift bool) {
		mods := ui.ModifierControl
		if shift {
			mods |= ui.ModifierShift
		}
		if !m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 9, Modifiers: mods}) {
			t.Fatal("MRU key not consumed")
		}
	}
	key(false)
	if m.active != 1 {
		t.Fatal("first MRU hop", m.active)
	}
	key(false)
	if m.active != 2 {
		t.Fatal("held Control oscillated between two instead of older document", m.active)
	}
	key(true)
	if m.active != 1 {
		t.Fatal("reverse held MRU", m.active)
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyReleased, Key: 17})
	if m.tabs.switchOrder != nil {
		t.Fatal("release retained frozen pointers")
	}
	key(false)
	if m.active != 2 {
		t.Fatal("next gesture did not use updated MRU", m.active)
	}
	m.input(nil, ui.InputEvent{Kind: ui.InputCancelled})
	if m.tabs.switchOrder != nil {
		t.Fatal("focus cancellation retained switch order")
	}
}

func TestTabIdentitySurvivesIndexesAndRejectsReopenedDocument(t *testing.T) {
	m := tabModel(t)
	target := m.docs[2]
	key, closeKey := m.tabKey(target), m.tabCloseKey(target)
	m.focusTab(target)
	m.removeTab(0)
	if m.current() != target || m.tabKey(target) != key || m.tabCloseKey(target) != closeKey {
		t.Fatal("index movement changed identity")
	}
	m.closeDocumentTab(target)
	for _, d := range m.tabs.mru {
		if d == target {
			t.Fatal("closed document retained in MRU")
		}
	}
	if len(m.docs) != 2 {
		t.Fatal("identity close selected wrong document")
	}
	reopened := &document{path: target.path}
	if m.tabKey(reopened) == key {
		t.Fatal("reopened document reused old pointer-capture identity")
	}
	if len(m.docs[:cap(m.docs)]) > len(m.docs) && m.docs[:cap(m.docs)][len(m.docs)] != nil {
		t.Fatal("closed tab pointer retained in document backing slice")
	}
}

func TestOrderedKeyboardNavigationAndDirtyCloseGuard(t *testing.T) {
	m := tabModel(t)
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 33, Modifiers: ui.ModifierControl})
	if m.active != 3 {
		t.Fatal("previous wrap", m.active)
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 34, Modifiers: ui.ModifierControl})
	if m.active != 0 {
		t.Fatal("next wrap", m.active)
	}
	m.input(nil, ui.InputEvent{Kind: ui.Character, Key: 'X'})
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'W', Modifiers: ui.ModifierControl})
	if !m.closePrompt || len(m.docs) != 4 {
		t.Fatal("shortcut discarded dirty document")
	}
	m.cancelClose()
	m.terminalFocused = true
	if m.tabInput(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'W', Modifiers: ui.ModifierControl}) {
		t.Fatal("terminal word-delete shortcut closed editor")
	}
}
