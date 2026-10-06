package languageserver

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/neko233-com/gocode/internal/languageserver/lspfixture"
	"github.com/neko233-com/godesktop/editor"
)

func TestSupervisorServerProcess(t *testing.T) {
	if !lspfixture.RunIfRequested() {
		t.Skip("owned helper process only")
	}
}
func fixtureConfig(mode string) Config {
	return Config{Name: "fixture", Languages: []string{"go"}, Command: os.Args[0], Arguments: []string{"-test.run=^TestSupervisorServerProcess$", "--", lspfixture.Flag, mode}}
}
func nextEvent(t *testing.T, s *Supervisor, state string) Event {
	t.Helper()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case e, ok := <-s.Events():
			if !ok {
				t.Fatal("supervisor ended before", state)
			}
			if e.State == state {
				return e
			}
		case <-timer.C:
			t.Fatal("missing supervisor state", state)
		}
	}
}
func TestActualProcessCrashReinitializeAndManualRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s := Supervise(ctx, t.TempDir(), fixtureConfig("serve"))
	defer s.Close()
	first := nextEvent(t, s, "ready").Session
	path := filepath.Join(t.TempDir(), "main.go")
	b, _ := editor.New("package main\n// unsaved 😀\n")
	if err := first.Submit(func(ctx context.Context, c *Client) error { return c.Sync(ctx, path, b.Snapshot(), nil) }); err != nil {
		t.Fatal(err)
	}
	if e := nextEvent(t, s, "diagnostics"); !strings.Contains(e.Diagnostics.Diagnostics[0].Message, "unsaved 😀") {
		t.Fatal(e)
	}
	if err := first.Client.RPC.Call(ctx, "fixture/crash", nil, nil); err == nil {
		t.Fatal("owned process did not crash")
	}
	nextEvent(t, s, "restarting")
	second := nextEvent(t, s, "ready").Session
	if first == second || first.Valid() {
		t.Fatal("old process generation remained valid")
	}
	_, _ = b.ReplaceSelection("new unsaved ")
	if err := second.Submit(func(ctx context.Context, c *Client) error { return c.Sync(ctx, path, b.Snapshot(), nil) }); err != nil {
		t.Fatal(err)
	}
	e := nextEvent(t, s, "diagnostics")
	if *e.Diagnostics.Version != b.Version() || !strings.Contains(e.Diagnostics.Diagnostics[0].Message, "new unsaved") {
		t.Fatal("latest didOpen was not received", e)
	}
	var raw json.RawMessage
	if err := second.Client.Request(ctx, path, b.Snapshot(), editor.Position{}, "textDocument/hover", &raw); err != nil || !strings.Contains(HoverText(raw), "new unsaved") {
		t.Fatal(string(raw), err)
	}
	s.Restart()
	third := nextEvent(t, s, "ready").Session
	if third == second || second.Valid() {
		t.Fatal("manual restart retained previous connection")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if third.Valid() || s.Valid(Event{Session: third}) {
		t.Fatal("shutdown still accepts callbacks")
	}
}
func TestCrashBudgetManualRecoveryAndCancelledInitialization(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	repaired := filepath.Join(t.TempDir(), "runtime-repaired")
	s := Supervise(ctx, t.TempDir(), fixtureConfig("fail-until:"+repaired))
	defer s.Close()
	count := 0
	for e := range s.Events() {
		if e.State == "starting" {
			count++
		}
		if e.State == "failed" {
			break
		}
	}
	if count != 5 {
		t.Fatal("restart budget did not stop repeated process failures", count)
	}
	select {
	case e := <-s.Events():
		t.Fatal("kept starting after exhausted budget", e)
	case <-time.After(100 * time.Millisecond):
	}
	if err := os.WriteFile(repaired, []byte("repaired"), 0600); err != nil {
		t.Fatal(err)
	}
	s.Restart()
	if !nextEvent(t, s, "ready").Session.Valid() {
		t.Fatal("manual recovery after runtime repair failed")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(t.TempDir(), "initialize-started")
	hanging := Supervise(ctx, t.TempDir(), fixtureConfig("hang:"+marker))
	defer hanging.Close()
	nextEvent(t, hanging, "starting")
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("actual initialize did not start")
		case <-time.After(10 * time.Millisecond):
		}
	}
	start := time.Now()
	if err := hanging.Close(); err != nil || time.Since(start) > 3*time.Second {
		t.Fatal("initialization cancellation did not reap process", err)
	}
}

func TestActualDiagnosticPayloadBoundsAndUTF8(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	workspace := t.TempDir()
	s := Supervise(ctx, workspace, fixtureConfig("large-diagnostics"))
	defer s.Close()
	client := nextEvent(t, s, "ready").Session
	b, _ := editor.New(strings.Repeat("中文😀", 200))
	if err := client.Submit(func(ctx context.Context, c *Client) error {
		return c.Sync(ctx, filepath.Join(workspace, "main.go"), b.Snapshot(), nil)
	}); err != nil {
		t.Fatal(err)
	}
	e := nextEvent(t, s, "diagnostics")
	if len(e.Diagnostics.Diagnostics) != 2000 {
		t.Fatal("unbounded diagnostic item count")
	}
	total := 0
	for _, d := range e.Diagnostics.Diagnostics {
		total += len(d.Message)
		if len(d.Message) > 2048 || !utf8.ValidString(d.Message) {
			t.Fatal("message bounds or UTF-8 broken")
		}
	}
	if total > 256<<10 || total == 0 {
		t.Fatal("unbounded or missing diagnostic text", total)
	}
}
func TestRequestAndDocumentBoundsArePerSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s := Supervise(ctx, t.TempDir(), fixtureConfig("serve"))
	defer s.Close()
	session := nextEvent(t, s, "ready").Session
	for i := 0; i < 8; i++ {
		if !session.Request(func(ctx context.Context, _ *Client) { <-ctx.Done() }) {
			t.Fatal("slot missing")
		}
	}
	if session.Request(func(context.Context, *Client) { t.Error("ninth request ran") }) {
		t.Fatal("request count is unbounded")
	}
	entered := make(chan struct{})
	if err := session.Submit(func(ctx context.Context, _ *Client) error { close(entered); <-ctx.Done(); return ctx.Err() }); err != nil {
		t.Fatal(err)
	}
	<-entered
	for i := 0; i < 32; i++ {
		if err := session.Submit(func(context.Context, *Client) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if err := session.Submit(func(context.Context, *Client) error { return nil }); err != ErrBacklog {
		t.Fatal("document overflow did not restart only its session", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}
