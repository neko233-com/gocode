package localrelease

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/languageextension"
	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/gocode/internal/nativeguard"
	"github.com/neko233-com/godesktop/editor"
)

func healthProgram(t *testing.T) []byte {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "main.go")
	program := filepath.Join(root, "health.exe")
	if err := os.WriteFile(source, []byte(`package main
import ("fmt"; "os")
func main() {
 if len(os.Args) != 2 || os.Args[1] != "-version" { os.Exit(9) }
 fmt.Println("gocode 0.24.0 windows/amd64 aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
}
`), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := nativeguard.Run(ctx, nativeguard.Options{Command: []string{"go", "build", "-buildvcs=false", "-o", program, source}, Environment: append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off", "GOFLAGS="), Timeout: 30 * time.Second})
	if err != nil {
		t.Fatalf("pure CPU owned health fixture build: %v %s", err, result.Stderr)
	}
	data, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRealCPUOwnedLaunchBindsActualGUIProgramAndExactHealth(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("Windows archive/owned launcher contract")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "versions", "0.24.0")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(dir, "gocode-app-gui.exe")
	if err := os.WriteFile(program, healthProgram(t), 0700); err != nil {
		t.Fatal(err)
	}
	spec := GateSpec{ID: "gui-version", Args: []string{"-version"}, GUI: true, Timeout: 5 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	captured, bound, err := RunGate(ctx, root, "0.24.0", strings.Repeat("a", 40), spec, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	actual, err := HashFile(ctx, program, 32<<20)
	if err != nil || bound.Launch.FullPath != program || bound.Report.Executable != program || bound.Launch.SHA256 != actual.SHA256 || bound.Launch.Bytes != actual.Bytes || !bound.Launch.UnchangedAfter || bound.Report.PID <= 0 || !bound.Report.RootReaped || !bound.Report.TreeClosed || !strings.Contains(string(captured.Stdout), strings.Repeat("a", 40)) {
		t.Fatalf("actual executable/owned-process binding differs: %+v %v", bound, err)
	}
	// The same real process cannot prove another source despite an identical
	// basename, exit code and -version prefix.
	if _, _, err := RunGate(ctx, root, "0.24.0", strings.Repeat("b", 40), spec, os.Environ()); err == nil {
		t.Fatal("old same-named actual payload counted as the current source")
	}
}

func TestProductionProbeRealCPUExecutableAndActualChangedBytes(t *testing.T) {
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		t.Skip("Windows health fixture")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "versions", "0.24.0")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	data := healthProgram(t)
	staged := Staged{Version: "0.24.0", Source: strings.Repeat("a", 40), Platform: "windows/amd64"}
	for i, name := range []string{"gocode-app.exe", "gocode-app-gui.exe"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0700); err != nil {
			t.Fatal(err)
		}
		hash, err := HashFile(context.Background(), path, 32<<20)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			staged.Console = hash
		} else {
			staged.GUI = hash
		}
	}
	if err := ProbeStaged(context.Background(), root, staged); err != nil {
		t.Fatal(err)
	}
	wrong := staged
	wrong.Source = strings.Repeat("b", 40)
	if err := ProbeStaged(context.Background(), root, wrong); err == nil {
		t.Fatal("production health accepted actual wrong source")
	}
	if err := os.WriteFile(staged.GUI.Path, []byte("changed actual GUI bytes"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := ProbeStaged(context.Background(), root, staged); err == nil {
		t.Fatal("production health ignored changed GUI payload")
	}
}

func TestActualLanguageReportValidatorRejectsIncompleteOrWrongPinnedService(t *testing.T) {
	for _, kind := range []string{"go", "typescript"} {
		pin, err := languageextension.Package(kind)
		if err != nil {
			t.Fatal(err)
		}
		version, line, extension := languageserver.GoplsVersion, 1, "go"
		if kind == "typescript" {
			version, line, extension = languageextension.TSVersion, 0, "ts"
		}
		valid := languageextension.CheckReport{Kind: kind, ID: pin.ID, Version: pin.Version, ArchiveSHA256: pin.SHA256, InventorySHA256: strings.Repeat("a", 64), DependencySHA256: strings.Repeat("b", 64), ServerVersion: version, TypeScriptVersion: "5.6.3", Adapter: "native-lsp-adapter", ProcessID: 123, ObservedProcessIDs: []int{123, 124}, ProcessClosed: true, Completion: "greeting", Diagnostic: "value is not assignable", FormatEdits: 2, Gates: []string{"original-vsix-inventory", "real-initialize-utf16", "actual-completion", "actual-definition", "actual-format-applied", "actual-type-diagnostic", "did-close"}, Definition: languageserver.Location{URI: "file:///actual/main." + extension, Range: editor.Range{Start: editor.Position{Line: line}}}}
		spec := GateSpec{ID: "installed-" + kind + "-service"}
		data, _ := json.Marshal(valid)
		if err := validateGateSuccess(spec, "0.24.0", strings.Repeat("a", 40), data); err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct {
			name   string
			change func(*languageextension.CheckReport)
		}{
			{"old-vsix", func(r *languageextension.CheckReport) { r.Version = "0.0.1" }},
			{"unreaped", func(r *languageextension.CheckReport) { r.ProcessClosed = false }},
			{"no-format", func(r *languageextension.CheckReport) { r.FormatEdits = 0 }},
			{"reordered", func(r *languageextension.CheckReport) {
				r.Gates = slices.Clone(r.Gates)
				r.Gates[1], r.Gates[2] = r.Gates[2], r.Gates[1]
			}},
			{"duplicate-pid", func(r *languageextension.CheckReport) { r.ObservedProcessIDs = []int{123, 123} }},
			{"wrong-utf16-definition", func(r *languageextension.CheckReport) { r.Definition.Range.Start.Line = 9 }},
		} {
			t.Run(kind+"/"+test.name, func(t *testing.T) {
				changed := valid
				test.change(&changed)
				data, _ := json.Marshal(changed)
				if err := validateGateSuccess(spec, "0.24.0", strings.Repeat("a", 40), data); err == nil {
					t.Fatal("invalid actual language report accepted")
				}
			})
		}
	}
}
