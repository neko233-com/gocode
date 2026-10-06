package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	ui "github.com/neko233-com/godesktop"
)

func testModel(t *testing.T) *model {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# workspace"), 0644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(root)
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func key(m *model, k int) { m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: k}) }
func text(m *model, s string) {
	for _, r := range s {
		m.input(nil, ui.InputEvent{Kind: ui.Character, Key: int(r)})
	}
}
func TestUnicodeEditAndSave(t *testing.T) {
	m := testModel(t)
	d := m.current()
	d.column = len([]rune(d.buffer.Line(0)))
	key(m, 13)
	text(m, "// 你好 😀")
	if d.buffer.Line(1) != "// 你好 😀" || !d.buffer.Dirty() {
		t.Fatalf("Unicode insertion: %#v", d)
	}
	key(m, 8)
	if d.buffer.Line(1) != "// 你好 " {
		t.Fatal("backspace split Unicode rune")
	}
	key(m, 36)
	key(m, 46)
	if d.buffer.Line(1) != "/ 你好 " {
		t.Fatal("delete failed")
	}
	if err := m.save(); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(d.path)
	if !strings.Contains(string(data), "/ 你好 ") || d.buffer.Dirty() {
		t.Fatalf("save %q dirty=%v", data, d.buffer.Dirty())
	}
	key(m, 36)
	key(m, 8)
	if d.buffer.LineCount() != 4 {
		t.Fatal("line merge failed")
	}
	key(m, 9)
	if !strings.Contains(d.buffer.Line(0), "    ") {
		t.Fatal("indent failed")
	}
}
func TestTabsAndFileValidation(t *testing.T) {
	m := testModel(t)
	first := m.current()
	m.open(filepath.Join(m.workspace, "README.md"))
	m.open(first.path)
	if len(m.docs) != 2 || m.current() != first {
		t.Fatal("duplicate file tab")
	}
	text(m, "changed")
	m.closeTab(0)
	if len(m.docs) != 2 || !m.closePrompt || m.closeTarget != first {
		t.Fatal("unsaved tab closed")
	}
	m.cancelClose()
	first.buffer.MarkSaved()
	m.closeTab(0)
	if len(m.docs) != 1 || m.current() == nil {
		t.Fatal("tab selection lost")
	}
	m.closeTab(0)
	if m.current() != nil {
		t.Fatal("last tab failed")
	}
	path := filepath.Join(m.workspace, "binary")
	_ = os.WriteFile(path, []byte{0, 1, 2}, 0644)
	m.open(path)
	if m.current() != nil || !strings.Contains(m.message, "Binary") {
		t.Fatal("binary file opened")
	}
	m.open(filepath.Join(m.workspace, "missing"))
	if m.message == "" {
		t.Fatal("missing file unreported")
	}
}
func TestCommandsAndSearchInput(t *testing.T) {
	m := testModel(t)
	if !m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'P', Modifiers: ui.ModifierControl}) || !m.palette {
		t.Fatal("command palette shortcut")
	}
	text(m, "native")
	if m.query != "native" {
		t.Fatal("palette input")
	}
	key(m, 8)
	key(m, 27)
	if m.palette || m.query != "" {
		t.Fatal("palette escape")
	}
	m.showSearch(nil)
	text(m, "main")
	if m.search.query.Text != "main" {
		t.Fatal("search input")
	}
	m.activity = "files"
	m.input(nil, ui.InputEvent{Kind: ui.Scroll, Y: -200})
	if m.current().scroll != m.current().buffer.LineCount()-1 {
		t.Fatal("scroll clamping")
	}
	for _, line := range []string{"func main() {", `fmt.Println("hello") // comment`, "// 你好", "var n = 123", "\treturn nil"} {
		var combined strings.Builder
		for _, f := range highlight(line) {
			combined.WriteString(f.text)
		}
		if combined.String() != strings.ReplaceAll(line, "\t", "    ") {
			t.Fatalf("highlight lost text %q", line)
		}
	}
	if m.view(&ui.Context{}) == nil {
		t.Fatal("empty view")
	}
}
