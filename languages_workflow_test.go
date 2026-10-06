package main

import (
	"encoding/json"
	"strings"
	"testing"

	textbuffer "github.com/neko233-com/godesktop/editor"
)

func TestCompletionAppliesPrimaryAndImportAtomically(t *testing.T) {
	m := testModel(t)
	d := m.current()
	d.buffer, _ = textbuffer.New("package main\n\nfunc main() { Pri }\n")
	m.moveCursor(d, 2, 17, false)
	raw := json.RawMessage(`{"items":[{"label":"Println","textEdit":{"range":{"start":{"line":2,"character":14},"end":{"line":2,"character":17}},"newText":"fmt.Println"},"additionalTextEdits":[{"range":{"start":{"line":1,"character":0},"end":{"line":1,"character":0}},"newText":"import \"fmt\"\n"}]}]}`)
	m.applyLSPResult("gopls", "textDocument/completion", d, d.buffer.Version(), d.cursor(), raw)
	if len(m.completions) != 1 {
		t.Fatal("completion list")
	}
	m.chooseCompletion(m.completions[0])
	if d.buffer.Text() != "package main\nimport \"fmt\"\n\nfunc main() { fmt.Println }\n" {
		t.Fatalf("atomic import edit %q", d.buffer.Text())
	}
	change, ok := d.buffer.Undo()
	if !ok {
		t.Fatal("completion has no undo")
	}
	m.changed(d, change, nil)
	if d.buffer.Text() != "package main\n\nfunc main() { Pri }\n" {
		t.Fatal("completion import was not undone atomically")
	}
}

func TestServiceSnapshotPolicyAndCompletionMerge(t *testing.T) {
	m := testModel(t)
	d := m.current()
	d.buffer, _ = textbuffer.New(strings.Repeat("x", 2<<20+1))
	if d.serviceEligible() || stateOf(d) != nil {
		t.Fatal("oversized document serialized to LSP or VSIX")
	}
	d.buffer, _ = textbuffer.New("x")
	d.serviceVersion = 0
	if !d.serviceEligible() {
		t.Fatal("eligibility not refreshed")
	}
	pos := d.cursor()
	one := completionSuggestion{d.path, 1, pos, completionItem{Label: json.RawMessage(`"extension"`)}}
	two := completionSuggestion{d.path, 1, pos, completionItem{Label: json.RawMessage(`"server"`)}}
	m.publishCompletions("lsp:gopls", []completionSuggestion{two})
	m.publishCompletions("extensions", []completionSuggestion{one})
	if len(m.completions) != 2 || m.completions[0].item.title() != "extension" || m.completions[1].item.title() != "server" {
		t.Fatal("completion sources overwrite or reorder nondeterministically")
	}
}
