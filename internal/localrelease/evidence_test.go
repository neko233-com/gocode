package localrelease

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActualBoundedMetadataRefusesMissingOriginAndMalformedStagedJSON(t *testing.T) {
	root := t.TempDir()
	info := filepath.Join(root, "v0.17.0.info")
	if err := os.WriteFile(info, []byte(`{"Version":"v0.17.0","Time":"2026-10-08T00:00:00Z"}`), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := readBounded(info, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateFrameworkOrigin(data, "v0.17.0", strings.Repeat("a", 40)); err == nil || !strings.Contains(err.Error(), "fresh private GOMODCACHE") {
		t.Fatalf("missing real Origin was replaced by claimed sum: %v", err)
	}
	actual := map[string]any{"Version": "v0.17.0", "Origin": map[string]string{"VCS": "git", "URL": "https://github.com/neko233-com/godesktop", "Hash": strings.Repeat("a", 40), "Ref": "refs/tags/v0.17.0"}}
	if err := WriteJSON(info, actual); err != nil {
		t.Fatal(err)
	}
	data, err = readBounded(info, 64<<10)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateFrameworkOrigin(data, "v0.17.0", strings.Repeat("a", 40)); err != nil {
		t.Fatal(err)
	}
	if err := validateFrameworkOrigin(data, "v0.17.0", strings.Repeat("b", 40)); err == nil {
		t.Fatal("actual tag Origin mismatch accepted")
	}
	path := filepath.Join(root, "staged.json")
	for _, body := range []string{`{"version":"0.24.0","source":"aaa","unknown":true}`, `{} {}`, strings.Repeat("x", MaxReceiptBytes+1)} {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadStaged(path); err == nil {
			t.Fatal("unbounded/trailing/unknown staged metadata accepted")
		}
	}
}

func TestExternalActualSourceEvidenceRejectsOldDigestUnreapedAndMissingRollback(t *testing.T) {
	inputs, source := strings.Repeat("b", 64), strings.Repeat("a", 40)
	valid := CheckEvidence{Schema: 1, ID: "source-windows-amd64-repeat3", Version: "0.24.0", Source: source, Command: []string{"powershell.exe", "-File", "scripts/test-windows-amd64.ps1"}, PID: 123, ExitCode: 0, ElapsedMS: 829761, RootReaped: true, TreeClosed: true, InputDigest: inputs}
	if err := validateCheckEvidence(valid, valid.ID, valid.Version, source, inputs); err != nil {
		t.Fatal(err)
	}
	for _, edit := range []func(*CheckEvidence){func(e *CheckEvidence) { e.InputDigest = strings.Repeat("c", 64) }, func(e *CheckEvidence) { e.RootReaped = false }, func(e *CheckEvidence) { e.Source = strings.Repeat("d", 40) }, func(e *CheckEvidence) { e.Command = nil }, func(e *CheckEvidence) { e.Command = []string{strings.Repeat("x", 4097)} }} {
		changed := valid
		edit(&changed)
		if err := validateCheckEvidence(changed, valid.ID, valid.Version, source, inputs); err == nil {
			t.Fatal("actual external source evidence mismatch accepted")
		}
	}
	valid.ID = "real-prior-release-rollback"
	if err := validateCheckEvidence(valid, valid.ID, valid.Version, source, inputs); err == nil {
		t.Fatal("old version health alone counted as genuine rollback")
	}
	valid.Rollback = &Rollback{Version: "0.24.0", Source: source}
	if err := verifyRollback(context.Background(), t.TempDir(), valid.Version, source, valid.Rollback); err == nil {
		t.Fatal("new release passed as prior signed archive")
	}
	encoded, err := json.Marshal(valid)
	if err != nil || len(encoded) > MaxReceiptBytes {
		t.Fatal(err)
	}
}

func TestMSIActualPackageInputsCannotUseOldSyntheticArchive(t *testing.T) {
	root := t.TempDir()
	var inputs []File
	for _, name := range WindowsArtifactNames("0.24.0") {
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, []byte("actual candidate "+name), 0600); err != nil {
			t.Fatal(err)
		}
		file, err := HashFile(context.Background(), path, 1024)
		if err != nil {
			t.Fatal(err)
		}
		file.Path = name
		inputs = append(inputs, file)
	}
	if err := validatePackageInputs(inputs, inputs); err != nil {
		t.Fatal(err)
	}
	old := append([]File(nil), inputs...)
	old[0].Path = "0.0.2.msi"
	if err := validatePackageInputs(old, inputs); err == nil {
		t.Fatal("synthetic installer counted as exact candidate")
	}
	changed := append([]File(nil), inputs...)
	changed[1].SHA256 = strings.Repeat("c", 64)
	if err := validatePackageInputs(changed, inputs); err == nil {
		t.Fatal("same-named different candidate ZIP accepted")
	}
	if err := os.WriteFile(filepath.Join(root, inputs[0].Path), []byte("late original MSI mutation"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyFile(context.Background(), root, inputs[0], 1024); err == nil {
		t.Fatal("changed original candidate MSI accepted")
	}
}
