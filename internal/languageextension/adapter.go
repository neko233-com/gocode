// Package languageextension activates explicitly supported, genuinely installed
// VSIX packages through native LSP adapters. It does not emulate their JavaScript
// activate functions or arbitrary vscode-languageclient APIs.
package languageextension

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/godesktop/extensions"
)

const TSVersion = "6.0.1"

type Pin struct {
	Kind, ID, Version, SHA256, URL string
}

func Package(kind string) (Pin, error) {
	switch kind {
	case "go":
		return Pin{kind, "golang.go", "0.50.0", "7b43b91174adc47c3bb0aa331faab32d2333cb4d12fba43ba7d2ad3fe35b07a6", "https://open-vsx.org/api/golang/Go/0.50.0/file/golang.Go-0.50.0.vsix"}, nil
	case "typescript":
		return Pin{kind, "vscode.typescript-language-features", "1.95.3", "b1494642a46d1b139ab799e2ee0ae2453bdcef6ed1a718acca034445cfd9d87f", "https://open-vsx.org/api/vscode/typescript-language-features/1.95.3/file/vscode.typescript-language-features-1.95.3.vsix"}, nil
	default:
		return Pin{}, errors.New("language extension must be go or typescript")
	}
}

func Kind(id string) string {
	switch strings.ToLower(id) {
	case "golang.go":
		return "go"
	case "vscode.typescript-language-features":
		return "typescript"
	}
	return ""
}

func ManagedTools() (string, error) {
	root, err := os.UserConfigDir()
	return filepath.Join(root, "gocode", "tools", "language-extensions"), err
}

// Config verifies the receipt and every installed file before selecting a
// process entry point. Call it on a worker, never on the native UI thread.
func Config(e extensions.Extension, tools string) (languageserver.Config, error) {
	kind := Kind(e.ID())
	pin, err := Package(kind)
	if err != nil {
		return languageserver.Config{}, err
	}
	if e.Manifest.Version != pin.Version {
		return languageserver.Config{}, fmt.Errorf("native %s adapter requires the original %s %s package", kind, pin.ID, pin.Version)
	}
	if filepath.Clean(e.Path) != installedPath(filepath.Dir(e.Path), pin) {
		return languageserver.Config{}, errors.New("language extension is outside its immutable version directory")
	}
	if _, err = VerifyInstalled(filepath.Dir(e.Path), kind); err != nil {
		return languageserver.Config{}, err
	}
	runtime, err := verifyRuntime(tools, kind)
	if err != nil {
		return languageserver.Config{}, err
	}
	config := languageserver.Config{Name: "extension:" + pin.ID, Command: runtime.Command, Arguments: runtime.Arguments}
	if kind == "go" {
		config.Languages = []string{"go"}
	} else {
		config.Languages = []string{"typescript", "typescriptreact", "javascript", "javascriptreact"}
		config.InitializationOptions = map[string]any{
			"hostInfo": "gocode native VSIX adapter", "disableAutomaticTypingAcquisition": true,
			"maxTsServerMemory": 512,
			"tsserver":          map[string]any{"path": filepath.Join(e.Path, "deps", "typescript", "lib", "tsserver.js"), "useSyntaxServer": "never", "logVerbosity": "off"},
		}
	}
	return config, nil
}

// EqualConfig compares immutable process inputs, not mutable session state.
func EqualConfig(a, b languageserver.Config) bool {
	a.ClientVersion, b.ClientVersion = "", ""
	x, ex := json.Marshal(a)
	y, ey := json.Marshal(b)
	return ex == nil && ey == nil && string(x) == string(y)
}
