// Package localrelease verifies locally built, signed and tested release bytes.
// It performs no public-network update discovery, signing or release upload.
package localrelease

import "time"

type GateSpec struct {
	ID      string        `json:"id"`
	Args    []string      `json:"args"`
	GUI     bool          `json:"gui"`
	Timeout time.Duration `json:"timeoutNanoseconds"`
}

// WindowsPlan is ordered and independent of the count of any older release.
// Every service command starts a real installed adapter. Prior-release rollback
// is a separate source-bound check, because it must execute the older archive.
func WindowsPlan() []GateSpec {
	plan := []GateSpec{{ID: "largefile", Args: []string{"-largefile-smoke"}, Timeout: 60 * time.Second}}
	for _, mode := range []string{"save", "discard", "cancel", "external"} {
		plan = append(plan, GateSpec{ID: mode, Args: []string{"-close-smoke", mode}, Timeout: 60 * time.Second})
	}
	for _, mode := range []string{"editor", "terminal", "terminal-vsix", "scm", "lsp", "filewatch", "open", "ui", "tabs", "search", "replace", "groups", "groups-vsix", "groups-large", "windows-workbench", "auto-save", "auto-save-minimized", "extension-detail", "popup-shadow"} {
		id := mode
		if id == "lsp" {
			id = "go-lsp"
		}
		plan = append(plan, GateSpec{ID: id, Args: []string{"-" + mode + "-smoke"}, Timeout: 60 * time.Second})
	}
	for _, fixture := range []struct{ id, flag, sizeFlag string }{
		{"gib-browse", "-largefile-smoke", "-largefile-smoke-mib"},
		{"gib-search", "-search-smoke", "-search-smoke-mib"},
		{"gib-split", "-groups-large-smoke", "-groups-large-mib"},
	} {
		plan = append(plan, GateSpec{ID: fixture.id, Args: []string{fixture.flag, fixture.sizeFlag, "1024"}, Timeout: 120 * time.Second})
	}
	plan = append(plan, GateSpec{ID: "gui-version", Args: []string{"-version"}, GUI: true, Timeout: 60 * time.Second})
	for _, fixture := range []string{"auto-save", "auto-save-minimized", "extension-detail", "popup-shadow"} {
		plan = append(plan, GateSpec{ID: "gui-" + fixture, Args: []string{"-" + fixture + "-smoke"}, GUI: true, Timeout: 60 * time.Second})
	}
	plan = append(plan,
		GateSpec{ID: "typescript-lsp", Args: []string{"-typescript-lsp-smoke"}, Timeout: 60 * time.Second},
		GateSpec{ID: "installed-go-service", Args: []string{"-language-extension-check", "go"}, Timeout: 60 * time.Second},
		GateSpec{ID: "installed-typescript-service", Args: []string{"-language-extension-check", "typescript"}, Timeout: 60 * time.Second},
	)
	return plan
}

func RequiredChecks() []string {
	return []string{"source-windows-amd64-repeat3", "source-no-cgo", "source-vet", "workflow-lint", "distribution-tests", "msi-install-upgrade-rollback-uninstall", "real-prior-release-rollback"}
}
