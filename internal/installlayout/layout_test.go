package installlayout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func payload(t *testing.T, root, version string) {
	t.Helper()
	dir := filepath.Join(root, "versions", version)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"gocode-app", "gocode-app-gui"} {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0700); err != nil {
			t.Fatal(err)
		}
	}
}
func TestVersionActivationAndFailedStagePreservesPointer(t *testing.T) {
	root := t.TempDir()
	base := Manifest{Owner: Owner, Schema: Schema, Version: "0.4.0", Source: "initial"}
	data, _ := json.Marshal(base)
	if err := os.WriteFile(filepath.Join(root, "base.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	payload(t, root, "0.4.0")
	_, active, err := Resolve(root, true)
	if err != nil || active.Version != "0.4.0" {
		t.Fatal(active, err)
	}
	next := Manifest{Owner: Owner, Schema: Schema, Version: "0.5.0", Source: "next"}
	if err := Activate(root, next); err == nil {
		t.Fatal("incomplete payload activated")
	}
	payload(t, root, "0.5.0")
	if err := Activate(root, next); err != nil {
		t.Fatal(err)
	}
	_, active, err = Resolve(root, false)
	if err != nil || active.Version != "0.5.0" {
		t.Fatal(active, err)
	}
	if err := Activate(root, base); err != nil {
		t.Fatal(err)
	}
	_, active, err = Resolve(root, false)
	if err != nil || active.Version != "0.4.0" {
		t.Fatal("rollback", active, err)
	}
}
func TestRejectForeignRootAndTraversal(t *testing.T) {
	for _, version := range []string{"../x", "C:/Windows", "0.4.0/../x", "0.4", "0.4.0\x00", "0.4.0-.."} {
		if ValidVersion(version) {
			t.Fatal("invalid version", version)
		}
	}
	root := t.TempDir()
	if _, err := ValidateRoot(root); err == nil {
		t.Fatal("unowned root")
	}
	if err := os.WriteFile(filepath.Join(root, "base.json"), []byte(`{"owner":"other","schema":1,"version":"0.4.0"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Resolve(root, false); err == nil {
		t.Fatal("foreign root")
	}
}

func TestMSIUpgradeSupersedesOldMutablePointer(t *testing.T) {
	root := t.TempDir()
	base := Manifest{Owner: Owner, Schema: Schema, Version: "0.6.0"}
	data, _ := json.Marshal(base)
	if err := os.WriteFile(filepath.Join(root, "base.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	old := Manifest{Owner: Owner, Schema: Schema, Version: "0.5.0"}
	data, _ = json.Marshal(old)
	if err := os.WriteFile(filepath.Join(root, "current.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	payload(t, root, "0.6.0")
	_, active, err := Resolve(root, false)
	if err != nil || active.Version != "0.6.0" {
		t.Fatal("installer upgrade was masked by stale pointer", active, err)
	}
	payload(t, root, "0.7.0")
	if err := Activate(root, Manifest{Owner: Owner, Schema: Schema, Version: "0.7.0"}); err != nil {
		t.Fatal(err)
	}
	previous, err := Read(filepath.Join(root, "previous.json"))
	if err != nil || previous.Version != "0.6.0" {
		t.Fatal("rollback must retain the actual MSI baseline, not a stale pointer", previous, err)
	}
}
func TestSemanticVersionOrder(t *testing.T) {
	versions := []string{"0.4.0-alpha", "0.4.0-alpha.1", "0.4.0-alpha.beta", "0.4.0-beta", "0.4.0-beta.2", "0.4.0-beta.11", "0.4.0-rc.1", "0.4.0", "0.4.1", "0.10.0", "1.0.0"}
	for i, a := range versions {
		if !ValidVersion(a) {
			t.Fatal(a)
		}
		for j, b := range versions {
			n := CompareVersion(a, b)
			if (i < j && n >= 0) || (i == j && n != 0) || (i > j && n <= 0) {
				t.Fatalf("compare %s, %s = %d", a, b, n)
			}
		}
	}
	for _, a := range []string{"01.0.0", "0.4.0-a..b", "0.4.0-01"} {
		if ValidVersion(a) {
			t.Fatal("invalid semver", a)
		}
	}
}
