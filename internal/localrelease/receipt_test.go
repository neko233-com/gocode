package localrelease

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/nativeguard"
)

func receiptFixture() Receipt {
	version := "0.24.0"
	r := Receipt{Schema: 1, Version: version, Source: strings.Repeat("a", 40), InputDigest: strings.Repeat("e", 64), Platforms: []string{"windows/amd64"}, Complete: true, PrivateTMPAbsent: true, PrivateConfigAbsent: true, StageAbsent: true}
	for _, name := range WindowsArtifactNames(version) {
		r.Artifacts = append(r.Artifacts, File{Path: "artifacts/" + name, Bytes: 1, SHA256: strings.Repeat("b", 64)})
	}
	root := filepath.Join(os.TempDir(), "local-release-shape-fixture", "private-stage")
	for _, spec := range WindowsPlan() {
		name := "gocode-app.exe"
		if spec.GUI {
			name = "gocode-app-gui.exe"
		}
		entry := "versions/" + version + "/" + name
		program := File{Path: entry, Bytes: 1, SHA256: strings.Repeat("c", 64)}
		launch := Launch{FullPath: filepath.Join(root, filepath.FromSlash(entry)), ArchiveEntry: entry, Bytes: program.Bytes, SHA256: program.SHA256, UnchangedAfter: true}
		r.Gates = append(r.Gates, Gate{ID: spec.ID, Program: program, Launch: launch, Result: nativeguard.Report{PID: 1, Executable: launch.FullPath, Arguments: GateArguments(spec, root), ExitCode: 0, RootReaped: true, TreeClosed: true, DeadlineMS: spec.Timeout.Milliseconds()}})
	}
	for _, id := range RequiredChecks() {
		r.Checks = append(r.Checks, Check{ID: id})
	}
	return r
}

func TestRequiredRealServicePlanAndPriorRollbackCannotDisappear(t *testing.T) {
	r := receiptFixture()
	if err := r.validateShape(r.Version, r.Source); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, spec := range WindowsPlan() {
		ids = append(ids, spec.ID)
	}
	for _, id := range []string{"go-lsp", "typescript-lsp", "installed-go-service", "installed-typescript-service", "popup-shadow", "gui-popup-shadow"} {
		if !slices.Contains(ids, id) {
			t.Fatalf("actual required gate disappeared: %s", id)
		}
	}
	if !slices.Contains(RequiredChecks(), "real-prior-release-rollback") || len(PendingCommands()) != 0 {
		t.Fatal("actual prior rollback/service plan differs")
	}
	r.Checks = r.Checks[:len(r.Checks)-1]
	if err := r.validateShape(r.Version, r.Source); err == nil {
		t.Fatal("missing real prior rollback was accepted")
	}
	if err := verifyRollback(context.Background(), t.TempDir(), r.Version, r.Source, nil); err == nil {
		t.Fatal("fabricated prior rollback was accepted")
	}
	if err := validateGateSuccess(WindowsPlan()[0], r.Version, r.Source, []byte("gocode 0.24.0 windows/amd64 "+r.Source)); err == nil {
		t.Fatal("-version output counted as real native acceptance")
	}
	for _, spec := range WindowsPlan() {
		if spec.ID == "installed-go-service" || spec.ID == "installed-typescript-service" {
			if err := validateGateSuccess(spec, r.Version, r.Source, []byte(`{"server_reaped":true}`)); err == nil {
				t.Fatal("incomplete real LSP report accepted")
			}
		}
	}
}

func TestReceiptRejectsEarlyExitAndUnboundSameNamedProgram(t *testing.T) {
	for _, change := range []struct {
		name string
		edit func(*Receipt)
	}{
		{"early-version", func(r *Receipt) { r.Gates[0].Result.Arguments = append(r.Gates[0].Result.Arguments, "-version") }},
		{"same-basename-old-path", func(r *Receipt) { r.Gates[0].Result.Executable = filepath.Join(os.TempDir(), "old", "gocode-app.exe") }},
		{"launch-bytes", func(r *Receipt) { r.Gates[0].Launch.SHA256 = strings.Repeat("d", 64) }},
		{"GUI-used-console", func(r *Receipt) {
			for i := range r.Gates {
				if r.Gates[i].ID == "gui-version" {
					r.Gates[i].Program.Path = "versions/0.24.0/gocode-app.exe"
					break
				}
			}
		}},
		{"deadline", func(r *Receipt) { r.Gates[0].Result.DeadlineMS = 120000 }},
		{"missing-cleanup", func(r *Receipt) { r.PrivateTMPAbsent = false }},
	} {
		t.Run(change.name, func(t *testing.T) {
			r := receiptFixture()
			change.edit(&r)
			if err := r.validateShape(r.Version, r.Source); err == nil || strings.Contains(err.Error(), "unwired") {
				t.Fatalf("meaningful negative control did not reject before unrelated pending gate: %v", err)
			}
		})
	}
}

func TestBoundedAtomicIdempotentReceiptAndRealByteMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "receipt.json")
	if err := WriteJSON(path, map[string]string{"source": strings.Repeat("a", 40)}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	time.Sleep(15 * time.Millisecond)
	if err := WriteJSON(path, map[string]string{"source": strings.Repeat("a", 40)}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(path)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("identical persisted receipt changed modification time")
	}
	digest, err := HashFile(context.Background(), path, MaxReceiptBytes)
	if err != nil {
		t.Fatal(err)
	}
	digest.Path = "receipt.json"
	if err := VerifyFile(context.Background(), root, digest, MaxReceiptBytes); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("actual changed bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(context.Background(), root, digest, MaxReceiptBytes); err == nil {
		t.Fatal("changed real artifact accepted")
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", MaxReceiptBytes+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadReceipt(path); err == nil {
		t.Fatal("oversized receipt accepted")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Fatal("receipt persistence leaked temporary files")
	}
}
