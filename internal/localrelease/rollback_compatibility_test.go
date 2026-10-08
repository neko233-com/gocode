package localrelease

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/neko233-com/gocode/internal/languageextension"
)

func TestRollbackDisabledStatePreservesOnlyUnrelatedPreferences(t *testing.T) {
	before := languageextension.State{Disabled: []string{"independent.keep", "GOLANG.GO"}, Uninstall: []string{"pending.keep"}}
	want := languageextension.State{Disabled: []string{"independent.keep", "GOLANG.GO", "vscode.typescript-language-features"}, Uninstall: []string{"pending.keep"}}
	if got := rollbackDisabledState(before); !reflect.DeepEqual(got, want) || len(before.Disabled) != 2 {
		t.Fatal("owned acceptance changed unrelated or original state", got, before)
	}
	if err := verifyRollbackCompatibility(context.Background(), t.TempDir(), "stage", "0.23.0", nil); err == nil {
		t.Fatal("old receipt without explicit compatibility observations accepted")
	}
}

func TestActualRollbackCompatibilityRetainedProofAfterPrivateCleanup(t *testing.T) {
	profile := os.Getenv("GOCODE_ROLLBACK_TEST_PROFILE")
	if profile == "" {
		t.Skip("actual original native VSIX inventory requires an owned verified profile")
	}
	root := t.TempDir()
	stage := filepath.Join(root, "private-stage")
	extensions := filepath.Join(stage, "acceptance-extensions")
	for _, kind := range []string{"go", "typescript"} {
		receipt, err := languageextension.VerifyInstalled(profile, kind)
		if err != nil {
			t.Fatal(err)
		}
		pin, _ := languageextension.Package(kind)
		packageName := pin.ID + "-" + pin.Version
		for _, file := range receipt.Files {
			data, err := os.ReadFile(filepath.Join(profile, packageName, file.Path))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(extensions, packageName, file.Path)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		marker := filepath.Join(".gocode-language", kind+".json")
		data, err := os.ReadFile(filepath.Join(profile, marker))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(extensions, marker)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(extensions, marker), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteJSON(filepath.Join(extensions, ".gocode-state.json"), languageextension.State{Disabled: []string{"independent.keep"}, Uninstall: []string{"pending.keep"}}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	proof, err := prepareRollbackCompatibility(ctx, stage, "0.23.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := languageextension.CheckRollback(ctx, extensions, "0.23.0"); err != nil {
		t.Fatal("real retained disabled state still incompatible", err)
	}
	if err := finishRollbackCompatibility(ctx, stage, &proof); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "rollback-compatibility"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"original-state", "applied-state", "final-state"} {
		data, err := os.ReadFile(filepath.Join(stage, "rollback-compatibility", name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "rollback-compatibility", name+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.RemoveAll(stage); err != nil { // exact private child of t.TempDir
		t.Fatal(err)
	}
	if err := verifyRollbackCompatibility(ctx, root, stage, "0.23.0", &proof); err != nil {
		t.Fatal("retained actual state/inventory cannot be verified after owned cleanup", err)
	}
	for _, mutate := range []func(*RollbackCompatibility){
		func(p *RollbackCompatibility) { p.Mode = "full-go-ts-compatible" },
		func(p *RollbackCompatibility) { p.ExtensionRoot = profile },
		func(p *RollbackCompatibility) { p.TargetVersion = "0.24.0" },
		func(p *RollbackCompatibility) { p.DisabledIDs = p.DisabledIDs[:1] },
		func(p *RollbackCompatibility) { p.PackagesAfter = nil },
		func(p *RollbackCompatibility) {
			p.PackagesBefore[0].ArchiveSHA256 = "changed"
			p.PackagesAfter[0].ArchiveSHA256 = "changed"
		},
		func(p *RollbackCompatibility) {
			p.PackagesBefore[0].Files[0].Bytes++
			p.PackagesAfter[0].Files[0].Bytes++
		},
		func(p *RollbackCompatibility) { p.FinalState.SHA256 = p.OriginalState.SHA256 },
	} {
		data, _ := json.Marshal(proof)
		var changed RollbackCompatibility
		if err := json.Unmarshal(data, &changed); err != nil {
			t.Fatal(err)
		}
		mutate(&changed)
		if err := verifyRollbackCompatibility(ctx, root, stage, "0.23.0", &changed); err == nil {
			t.Fatal("altered compatibility state/source/inventory admitted")
		}
	}
	if err := os.WriteFile(filepath.Join(root, "rollback-compatibility", "final-state.json"), []byte(`{"disabled":[],"uninstall":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyRollbackCompatibility(ctx, root, stage, "0.23.0", &proof); err == nil {
		t.Fatal("retained state bytes tampering was accepted")
	}
	for _, kind := range []string{"go", "typescript"} {
		if _, err := languageextension.VerifyInstalled(profile, kind); err != nil {
			t.Fatal("original private source VSIX changed", err)
		}
	}
}
