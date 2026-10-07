package main

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestTerminalOptionsRejectBeforeStartup(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"pty":{}}`, `{"location":2}`, `{"shellArgs":null}`, `{"shellArgs":[1]}`, `{"env":{"A=B":"x"}}`, `{"env":{"":"x"}}`, `{"name":"\u0000"}`, `{"strictEnv":"true"}`, `{"name":"` + strings.Repeat("界", 86) + `"}`} {
		if _, err := decodeTerminalOptions(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	options, err := decodeTerminalOptions(json.RawMessage(`{"cwd":"run","env":{"VALUE":"世界😀","REMOVE":null},"shellArgs":[],"hideFromUser":true,"strictEnv":true}`))
	if err != nil || !options.StrictEnv || options.Env["REMOVE"] != nil || *options.Env["VALUE"] != "世界😀" {
		t.Fatal(options, err)
	}
	_, err = decodeTerminalOptions(json.RawMessage(`{"shellArgs":"-e \"quoted\""}`))
	if (err == nil) != (runtime.GOOS == "windows") {
		t.Fatal("raw command line platform policy", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := prepareExtensionTerminal(ctx, t.TempDir(), options, true); err == nil {
		t.Fatal("cancelled request reached preparation")
	}
}

func TestTerminalEnvironmentDeletionStrictAndWindowsDriveEntry(t *testing.T) {
	value := "世界😀"
	base := []string{"PATH=first", "REMOVE=yes", "TERM=custom", "=C:=C:\\workspace"}
	got := terminalEnvironment(base, map[string]*string{"REMOVE": nil, "TERM": nil, "VALUE": &value}, false)
	want := []string{"=C:=C:\\workspace", "PATH=first", "VALUE=世界😀"}
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	if got := terminalEnvironment(base, map[string]*string{"VALUE": &value, "REMOVE": nil}, true); !reflect.DeepEqual(got, []string{"VALUE=世界😀"}) {
		t.Fatal(got)
	}
	if got := terminalEnvironment(base, nil, true); got == nil || len(got) != 0 {
		t.Fatal("strict empty environment must stay empty", got)
	}
	if runtime.GOOS == "windows" {
		got := terminalEnvironment([]string{"Path=first", "PATH=second"}, map[string]*string{"path": nil}, false)
		if len(got) != 0 {
			t.Fatal("case insensitive deletion", got)
		}
	}
}

func TestPreparedExtensionEnvironmentNullRemovesTerminalDefaults(t *testing.T) {
	config, err := prepareExtensionTerminal(context.Background(), t.TempDir(), terminalLaunchOptions{ShellPath: os.Args[0], Env: map[string]*string{"TERM": nil, "COLORTERM": nil, "TERM_PROGRAM": nil}}, true)
	if err != nil {
		t.Fatal(err)
	}
	defer config.Discard()
	if !config.StrictEnvironment {
		t.Fatal("final overrides would be replaced by process defaults")
	}
	for _, entry := range config.Environment {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "TERM") || strings.EqualFold(key, "COLORTERM") || strings.EqualFold(key, "TERM_PROGRAM") {
			t.Fatal("null default reappeared", entry)
		}
	}
}

func TestTerminalSnapshotGenerationAndImmutableRecords(t *testing.T) {
	m := &model{activeTerminal: 0, terminals: []*terminalTab{{wireID: "native:1", name: "User", options: terminalLaunchOptions{Name: "User"}}}}
	initial := m.terminalState()
	if initial.Generation == 0 || initial.ActiveID != "native:1" || m.terminalState().Generation != initial.Generation {
		t.Fatal("unchanged snapshot advances generation", initial)
	}
	m.terminals[0].interacted = true
	if next := m.terminalState(); next.Generation != initial.Generation+1 || !next.Terminals[0].Interacted {
		t.Fatal(next)
	}
	if initial.Terminals[0].Interacted {
		t.Fatal("snapshot changed after publication")
	}
}

func TestTerminalClosureSelectsVisibleSurvivor(t *testing.T) {
	m := &model{activeTerminal: 1, terminals: []*terminalTab{{wireID: "visible"}, {wireID: "hidden", hidden: true}}}
	m.selectVisibleTerminal()
	if m.currentTerminal().wireID != "visible" {
		t.Fatal("hidden survivor became active")
	}
	m.terminals[0].hidden = true
	m.selectVisibleTerminal()
	if m.currentTerminal() != nil || m.activeTerminal != -1 {
		t.Fatal("hidden terminal retained focus")
	}
}
