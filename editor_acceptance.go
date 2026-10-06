package main

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/neko233-com/gocode/internal/copilotservice"
	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/extensions"
)

func verifyNativeEditor(ctx context.Context, cx *ui.Context, m *model, host *extensions.Host) error {
	var edit struct {
		Applied bool
		Version int
		Text    string
	}
	if err := host.Call(ctx, "execute", map[string]string{"command": "gocode.edit"}, &edit); err != nil {
		return err
	}
	if !edit.Applied || edit.Version != 2 || !strings.HasPrefix(edit.Text, "// VSIX 编辑 😀\r\n") {
		return errors.New("VSIX did not edit the actual native buffer")
	}
	type result struct {
		state *documentState
		err   error
	}
	done := make(chan result, 1)
	if !cx.Dispatch(func() {
		d := m.current()
		if d == nil {
			done <- result{err: errors.New("no native active document")}
			return
		}
		if err := m.save(); err != nil {
			done <- result{err: err}
			return
		}
		m.editing = true
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 'Z', Modifiers: ui.ModifierControl})
		if !d.buffer.Dirty() || strings.HasPrefix(d.buffer.Text(), "// VSIX") {
			done <- result{err: errors.New("native undo did not restore original document")}
			return
		}
		m.input(cx, ui.InputEvent{Kind: ui.KeyPressed, Key: 'Y', Modifiers: ui.ModifierControl})
		if d.buffer.Dirty() || d.buffer.Text() != edit.Text {
			done <- result{err: errors.New("native redo did not restore saved revision")}
			return
		}
		done <- result{state: stateOf(d)}
	}) {
		return errors.New("native UI closed during acceptance")
	}
	var state *documentState
	select {
	case value := <-done:
		if value.err != nil {
			return value.err
		}
		state = value.state
	case <-ctx.Done():
		return ctx.Err()
	}
	data, err := os.ReadFile(state.Path)
	if err != nil || string(data) != edit.Text {
		return errors.New("native CRLF save differs from acknowledged extension edit")
	}
	if err := host.Call(ctx, "syncDocument", map[string]any{"document": state, "kind": "focus"}, nil); err != nil {
		return err
	}
	var items []struct{ Label, Detail string }
	if err := host.Call(ctx, "provideCompletionItems", map[string]any{"uri": copilotservice.FileURI(state.Path), "position": state.Selection.Active}, &items); err != nil {
		return err
	}
	if len(items) != 1 || items[0].Label != "nativePrint" || items[0].Detail != "document-version:4" {
		return errors.New("extension completion did not observe native undo/redo version")
	}
	return nil
}
