package localrelease

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/neko233-com/gocode/internal/languageextension"
	"github.com/neko233-com/gocode/internal/languageserver"
)

func sha256Text(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil && len(value) == 64 && value == strings.ToLower(value)
}

func validateGateSuccess(spec GateSpec, version, source string, stdout []byte) error {
	id := strings.TrimPrefix(spec.ID, "gui-")
	if spec.ID == "gui-version" {
		if strings.TrimSpace(string(stdout)) != "gocode "+version+" windows/amd64 "+source {
			return errors.New("actual GUI payload health did not report exact source/version/platform")
		}
		return nil
	}
	if id == "installed-go-service" || id == "installed-typescript-service" {
		kind := strings.TrimSuffix(strings.TrimPrefix(id, "installed-"), "-service")
		var report languageextension.CheckReport
		decoder := json.NewDecoder(bytes.NewReader(stdout))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&report); err != nil {
			return err
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
			return errors.New("trailing language service report")
		}
		pin, err := languageextension.Package(kind)
		if err != nil {
			return err
		}
		serverVersion, definitionLine, suffix := languageextension.TSVersion, 0, "/main.ts"
		if kind == "go" {
			serverVersion, definitionLine, suffix = languageserver.GoplsVersion, 1, "/main.go"
		}
		gates := []string{"original-vsix-inventory", "real-initialize-utf16", "actual-completion", "actual-definition", "actual-format-applied", "actual-type-diagnostic", "did-close"}
		if report.Kind != kind || report.ID != pin.ID || report.Version != pin.Version || report.ArchiveSHA256 != pin.SHA256 || !sha256Text(report.InventorySHA256) || !sha256Text(report.DependencySHA256) || report.ServerVersion != serverVersion || report.Adapter != "native-lsp-adapter" || report.ProcessID <= 0 || !report.ProcessClosed || report.Completion != "greeting" || report.FormatEdits <= 0 || !slices.Equal(report.Gates, gates) || report.Definition.Range.Start.Line != definitionLine || !strings.HasSuffix(report.Definition.URI, suffix) || !(strings.Contains(report.Diagnostic, "cannot use") || strings.Contains(report.Diagnostic, "not assignable")) {
			return errors.New("actual installed language-service report is incomplete or differs from pinned real adapter")
		}
		if kind == "typescript" && report.TypeScriptVersion != "5.6.3" {
			return errors.New("actual bundled TypeScript version differs")
		}
		if len(report.ObservedProcessIDs) == 0 || kind == "typescript" && len(report.ObservedProcessIDs) < 2 {
			return errors.New("actual language process tree proof is absent")
		}
		seen := map[int]bool{}
		for _, pid := range report.ObservedProcessIDs {
			if pid <= 0 || seen[pid] {
				return errors.New("invalid observed language process identities")
			}
			seen[pid] = true
		}
		if !seen[report.ProcessID] {
			return errors.New("actual server root is not included in observed process tree")
		}
		return nil
	}
	sentinel := map[string]string{
		"largefile": "gocode large-file acceptance passed:", "gib-browse": "gocode large-file acceptance passed:",
		"editor": "gocode editor acceptance passed:", "terminal": "gocode terminal acceptance passed:",
		"terminal-vsix": "Native terminal VSIX passed:", "scm": "Native Git passed:",
		"go-lsp": "gocode LSP acceptance passed:", "typescript-lsp": "gocode TypeScript LSP acceptance passed:",
		"filewatch": "gocode file watch acceptance passed:", "open": "gocode async open acceptance passed:",
		"ui":     "Native owned workbench GPU logo, complete measured Latin/Unicode captions, tab selection/close and unchanged source passed",
		"tabs":   "Native 40-tab clipping, wheel/drag routing, stable captured identity, ordered/MRU navigation and Control release passed; all source bytes unchanged",
		"search": "Native search passed:", "gib-search": "1024 MiB single-line search/navigation and unchanged source",
		"replace": "Native replacement passed:", "groups": "Native editor groups passed:",
		"groups-vsix": "Native editor VSIX passed:", "groups-large": "Native large editor groups passed:",
		"gib-split":         "actual 1073741824 bytes, shared bounded index",
		"windows-workbench": "Windows native File/shell/Unicode save/Quick Input/VSIX management and persistent VS Code/JetBrains keymaps including real double Shift passed",
		"auto-save":         "native Auto Save acceptance passed: 66 phases", "auto-save-minimized": "native minimized Auto Save acceptance passed:",
		"extension-detail": "native extension detail acceptance passed:", "popup-shadow": "native popup shadows passed:",
		"prior-rollback": "gocode smoke passed: native rendering + installed VSIX activation + command execution",
	}[id]
	if slices.Contains([]string{"save", "discard", "cancel", "external"}, id) {
		sentinel = "gocode native close acceptance passed: " + id
	}
	if sentinel == "" || !strings.Contains(string(stdout), sentinel) {
		return fmt.Errorf("actual fixture success sentinel is absent: %s", spec.ID)
	}
	return nil
}
