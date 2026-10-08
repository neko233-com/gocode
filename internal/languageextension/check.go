package languageextension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/extensions"
)

type CheckReport struct {
	Kind               string                  `json:"kind"`
	ID                 string                  `json:"extension_id"`
	Version            string                  `json:"extension_version"`
	ArchiveSHA256      string                  `json:"archive_sha256"`
	InventorySHA256    string                  `json:"inventory_sha256"`
	ServerVersion      string                  `json:"server_version"`
	DependencySHA256   string                  `json:"dependency_sha256"`
	TypeScriptVersion  string                  `json:"typescript_version,omitempty"`
	Adapter            string                  `json:"activation"`
	ProcessID          int                     `json:"server_pid"`
	ObservedProcessIDs []int                   `json:"observed_process_ids,omitempty"`
	ProcessClosed      bool                    `json:"server_reaped"`
	Gates              []string                `json:"gates"`
	Completion         string                  `json:"completion"`
	Diagnostic         string                  `json:"diagnostic"`
	Definition         languageserver.Location `json:"definition"`
	FormatEdits        int                     `json:"format_edits"`
}

func sameFileURI(raw, path string) bool {
	resolved, err := languageserver.PathFromURI(raw)
	if err != nil {
		return false
	}
	a, err := os.Stat(resolved)
	if err != nil {
		return false
	}
	b, err := os.Stat(path)
	if err != nil {
		return false
	}
	return os.SameFile(a, b)
}

// Check uses actual installed VSIX bytes and a real process/server handshake.
// Its private workspace is removed only after the server is explicitly closed.
// No Node extension-host activation or native UI/GPU execution is implied.
func Check(parent context.Context, extensionRoot, tools, kind string) (report CheckReport, failure error) {
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	receipt, err := VerifyInstalled(extensionRoot, kind)
	if err != nil {
		return report, err
	}
	installed, err := extensions.List(extensionRoot)
	if err != nil {
		return report, err
	}
	var selected extensions.Extension
	for _, e := range installed {
		if strings.EqualFold(e.ID(), receipt.ID) && e.Manifest.Version == receipt.Version {
			selected = e
			break
		}
	}
	if selected.Path == "" {
		return report, errors.New("verified language extension is not selected in the actual installed inventory")
	}
	config, err := Config(selected, tools)
	if err != nil {
		return report, err
	}
	runtime, err := verifyRuntime(tools, kind)
	if err != nil {
		return report, err
	}
	workspace, err := os.MkdirTemp("", "gocode-language-extension-")
	if err != nil {
		return report, err
	}
	defer os.RemoveAll(workspace) // exclusively this invocation's owned workspace
	var source, name string
	var completionPosition, definitionPosition editor.Position
	if kind == "go" {
		source = "package main\nfunc add(a int,b int)int{return a+b}\nvar answer = add(1,2)\nvar bad int = \"wrong\"\nfunc main(){var greeting = \"😀\"; _ = greeting}\n"
		name = "main.go"
		completionPosition = editor.Position{Line: 4, Character: len("func main(){var greeting = \"😀\"; _ = ") + 4 - 2} // emoji is two UTF-16 units, four UTF-8 bytes
		definitionPosition = editor.Position{Line: 2, Character: 14}
		if err = os.WriteFile(filepath.Join(workspace, "go.mod"), []byte("module gocode-language-check\n\ngo 1.27.0\n"), 0600); err != nil {
			return report, err
		}
	} else {
		source = "export function add(a:number,b:number){return a+b;}\nexport const answer=add(1,2);\nexport const bad:number=\"wrong\";\nexport const greeting=\"😀\";\nexport const other=greeting;\n"
		name = "main.ts"
		completionPosition = editor.Position{Line: 4, Character: len("export const other=gree")}
		definitionPosition = editor.Position{Line: 1, Character: 20}
		if err = os.WriteFile(filepath.Join(workspace, "tsconfig.json"), []byte(`{"compilerOptions":{"strict":true,"noEmit":true},"include":["*.ts"]}`), 0600); err != nil {
			return report, err
		}
	}
	path := filepath.Join(workspace, name)
	if err = os.WriteFile(path, []byte(source), 0600); err != nil {
		return report, err
	}
	buffer, _ := editor.New(source)
	client, err := languageserver.Start(ctx, workspace, config)
	if err != nil {
		return report, err
	}
	var reapWitness func() error
	report = CheckReport{Kind: kind, ID: receipt.ID, Version: receipt.Version, ArchiveSHA256: receipt.ArchiveSHA256, Adapter: "native-lsp-adapter", ProcessID: client.ProcessID(), Gates: []string{"original-vsix-inventory", "real-initialize-utf16"}}
	report.InventorySHA256, report.ServerVersion = inventory(receipt.Files), runtime.Version
	for _, file := range runtime.Files {
		if kind == "go" && strings.HasPrefix(file.Path, "gopls") || kind == "typescript" && filepath.ToSlash(file.Path) == "node_modules/typescript-language-server/lib/cli.mjs" {
			report.DependencySHA256 = file.SHA256
		}
	}
	if kind == "typescript" {
		report.TypeScriptVersion = "5.6.3"
	} // complete pinned VSIX inventory includes its genuine package.json
	defer func() {
		closeErr := client.Close()
		var witnessErr error
		if reapWitness != nil {
			witnessErr = reapWitness()
		}
		failure = errors.Join(failure, closeErr, witnessErr)
		report.ProcessClosed = client.ProcessClosed() && closeErr == nil && witnessErr == nil
		if !report.ProcessClosed {
			failure = errors.Join(failure, errors.New("actual language server root was not reaped"))
		}
	}()
	if report.ProcessID <= 0 {
		return report, errors.New("language server has no actual owned process identity")
	}
	for _, method := range []string{"textDocument/completion", "textDocument/definition", "textDocument/formatting"} {
		if !client.Supports(method) {
			return report, fmt.Errorf("actual server did not advertise %s", method)
		}
	}
	if err = client.Sync(ctx, path, buffer.Snapshot(), nil); err != nil {
		return report, err
	}
	var raw json.RawMessage
	if err = client.Request(ctx, path, buffer.Snapshot(), completionPosition, "textDocument/completion", &raw); err != nil {
		return report, err
	}
	items, err := languageserver.Completions(raw)
	if err != nil {
		return report, err
	}
	for _, item := range items {
		var label string
		_ = json.Unmarshal(item.Label, &label)
		if label == "greeting" {
			report.Completion = label
			break
		}
	}
	if report.Completion == "" {
		return report, fmt.Errorf("actual completion has no expected greeting symbol (%d items)", len(items))
	}
	report.Gates = append(report.Gates, "actual-completion")
	report.ObservedProcessIDs, reapWitness, err = ObserveProcesses(client, kind)
	if err != nil {
		return report, err
	}
	if err = client.Request(ctx, path, buffer.Snapshot(), definitionPosition, "textDocument/definition", &raw); err != nil {
		return report, err
	}
	locations, err := languageserver.Locations(raw)
	if err != nil || len(locations) == 0 {
		return report, errors.Join(err, errors.New("actual definition returned no locations"))
	}
	report.Definition = locations[0]
	wantLine := 0
	if kind == "go" {
		wantLine = 1
	}
	if !sameFileURI(report.Definition.URI, path) || report.Definition.Range.Start.Line != wantLine {
		return report, errors.New("actual definition did not resolve the installed-server workspace symbol")
	}
	report.Gates = append(report.Gates, "actual-definition")
	if err = client.Request(ctx, path, buffer.Snapshot(), editor.Position{}, "textDocument/formatting", &raw); err != nil {
		return report, err
	}
	var edits []editor.Edit
	if err = json.Unmarshal(raw, &edits); err != nil || len(edits) == 0 {
		return report, errors.Join(err, errors.New("actual formatting returned no edits"))
	}
	if _, err = buffer.Apply(edits, nil); err != nil {
		return report, err
	}
	if buffer.Snapshot().Text() == source {
		return report, errors.New("actual formatting left source unchanged")
	}
	report.FormatEdits = len(edits)
	report.Gates = append(report.Gates, "actual-format-applied")
	for report.Diagnostic == "" {
		select {
		case notification, ok := <-client.RPC.Notifications():
			if !ok {
				return report, errors.New("language server closed before actual diagnostic")
			}
			if notification.Method != "textDocument/publishDiagnostics" {
				continue
			}
			var p languageserver.PublishDiagnostics
			if json.Unmarshal(notification.Params, &p) != nil || !sameFileURI(p.URI, path) {
				continue
			}
			for _, d := range p.Diagnostics {
				if d.Severity == 1 && (strings.Contains(d.Message, "cannot use") || strings.Contains(d.Message, "not assignable")) {
					report.Diagnostic = d.Message
					break
				}
			}
		case <-ctx.Done():
			return report, fmt.Errorf("no actual type error diagnostic: %w", ctx.Err())
		}
	}
	report.Gates = append(report.Gates, "actual-type-diagnostic")
	if err = client.CloseDocument(ctx, path); err != nil {
		return report, err
	}
	report.Gates = append(report.Gates, "did-close")
	return report, nil
}
