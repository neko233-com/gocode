package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gitrepo "github.com/neko233-com/gocode/internal/git"
	ui "github.com/neko233-com/godesktop"
)

func gitFixture(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.name", "Gocode fixture"}, {"config", "user.email", "fixture@example.invalid"}, {"config", "commit.gpgsign", "false"}, {"config", "core.autocrlf", "false"}, {"config", "core.hooksPath", filepath.ToSlash(filepath.Join(directory, "no-hooks"))}} {
		command := exec.Command("git", append([]string{"-C", directory}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("%v %s", err, output)
		}
	}
	return directory
}
func TestRealGitDiffRowsRoundtripAndLineNumbers(t *testing.T) {
	for _, sample := range []struct{ name, before, after string }{{"same", "a\nb\n", "a\nb\n"}, {"add", "", "first 😀\n"}, {"delete", "before 世界\n", ""}, {"replace", "a\nbefore\nz\n", "a\nafter\nmore\nz\n"}, {"tail", "a\nb\nc\nd\ne\nf\ng\nh\n", "a\nb\nc\nd\ne\nf\ng\nh\nnew\n"}, {"middle", "a\nb\nc\nd\ne\nf\ng\nh\ni\n", "a\nb\nc\nd\ninsert\ne\nf\ng\nh\ni\n"}, {"eol", "a\r\nb\r\n", "a\r\nc\r\n"}, {"no-eol", "a\nb", "a\nc"}} {
		t.Run(sample.name, func(t *testing.T) {
			root := gitFixture(t)
			path := filepath.Join(root, "file.go")
			if err := os.WriteFile(path, []byte(sample.before), 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"add", "--", "file.go"}, {"commit", "-m", "Before"}} {
				if output, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
					t.Fatalf("%v %s", err, output)
				}
			}
			if err := os.WriteFile(path, []byte(sample.after), 0600); err != nil {
				t.Fatal(err)
			}
			r, err := gitrepo.Open(context.Background(), root)
			if err != nil {
				t.Fatal(err)
			}
			state, err := r.Status(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			diff := gitrepo.Diff{Before: sample.before, After: sample.after}
			if len(state.Entries) > 0 {
				diff, err = r.Diff(context.Background(), state, "file.go", false)
				if err != nil {
					t.Fatal(err)
				}
			}
			rows, err := buildDiffRows(diff)
			if err != nil {
				t.Fatal(err)
			}
			var left, right []string
			old, next := 0, 0
			for _, row := range rows {
				if row.oldLine > 0 {
					old++
					if row.oldLine != old {
						t.Fatal("old line gap", row)
					}
					left = append(left, row.left)
				}
				if row.newLine > 0 {
					next++
					if row.newLine != next {
						t.Fatal("new line gap", row)
					}
					right = append(right, row.right)
				}
			}
			normal := func(text string) string { return strings.ReplaceAll(strings.TrimSuffix(text, "\n"), "\r", "") }
			if strings.Join(left, "\n") != normal(sample.before) || strings.Join(right, "\n") != normal(sample.after) {
				t.Fatal("diff lost source", rows)
			}
		})
	}
}
func TestSCMWorkerRealStageCommitAndDirtyDocumentRejection(t *testing.T) {
	root := gitFixture(t)
	path := filepath.Join(root, "main.go")
	if err := os.WriteFile(path, []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.closeDocuments()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dispatch := make(chan func(), 32)
	stop := m.startSCM(ctx, func(fn func()) bool {
		select {
		case dispatch <- fn:
			return true
		case <-ctx.Done():
			return false
		}
	})
	defer stop()
	await := func() {
		t.Helper()
		timeout := time.After(10 * time.Second)
		for m.scm.busy {
			select {
			case fn := <-dispatch:
				fn()
			case <-timeout:
				t.Fatal("SCM worker timed out", m.scm.status)
			}
		}
	}
	await()
	if len(m.scm.snapshot.Entries) != 1 {
		t.Fatal(m.scm.status, m.scm.snapshot)
	}
	m.editing = true
	m.input(nil, ui.InputEvent{Kind: ui.Character, Key: 'X'})
	m.scmAction("stage", []string{"main.go"}, false)
	if m.scm.busy || !strings.Contains(m.message, "Save") {
		t.Fatal("unsaved source silently staged", m.message)
	}
	m.input(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'Z', Modifiers: ui.ModifierControl})
	m.scmAction("stage", []string{"main.go"}, false)
	await()
	if !m.scm.snapshot.Entries[0].Staged() {
		t.Fatal(m.scm.status)
	}
	// A real diff job may complete after the user selects a normal editor tab.
	m.scmAction("diff", []string{"main.go"}, true)
	m.focusTab(m.current())
	await()
	if m.scm.diff != nil || !m.editing {
		t.Fatal("late diff stole the user's newer editor focus")
	}
	m.scm.message = "Actual GUI controller 世界 😀"
	m.scmAction("commit", nil, false)
	await()
	if m.scm.snapshot.Head == "" || len(m.scm.snapshot.Entries) != 0 || m.scm.message != "" {
		t.Fatal(m.scm.status)
	}
}

func TestSCMDiffReadOnlyInputAndNormalTabRestoresEditing(t *testing.T) {
	root := gitFixture(t)
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := newModel(root)
	if err != nil {
		t.Fatal(err)
	}
	defer m.closeDocuments()
	m.scm.diff = &gitrepo.Diff{Path: "main.go"}
	m.scm.diffRows = make([]diffRow, 100)
	before := m.current().buffer.Text()
	for _, event := range []ui.InputEvent{{Kind: ui.Character, Key: 'X'}, {Kind: ui.KeyPressed, Key: 'Z', Modifiers: ui.ModifierControl}, {Kind: ui.KeyPressed, Key: 34}} {
		if !m.scmInput(nil, event) {
			t.Fatal("readonly diff did not own input", event)
		}
	}
	if m.current().buffer.Text() != before || m.scm.diffScroll != 400 {
		t.Fatal("diff input changed hidden source")
	}
	m.terminalFocused = true
	if m.scmInput(nil, ui.InputEvent{Kind: ui.KeyPressed, Key: 'Z', Modifiers: ui.ModifierControl}) {
		t.Fatal("diff stole terminal shortcut")
	}
	m.focusTab(m.current())
	if m.scm.diff != nil || !m.editing || m.terminalFocused {
		t.Fatal("normal tab retained diff")
	}
}
