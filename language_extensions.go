package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/gocode/internal/languageextension"
	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/godesktop/extensions"
)

// Only genuine, deliberately supported packages use this route. Their original
// JS/manifest commands are not advertised as callbacks in the partial Node host.
func languageHostExtensions(installed []extensions.Extension) []extensions.Extension {
	result := make([]extensions.Extension, 0, len(installed))
	for _, e := range installed {
		if languageextension.Kind(e.ID()) == "" {
			result = append(result, e)
		}
	}
	return result
}

func languageExtensionDescription(e extensions.Extension) string {
	kind := languageextension.Kind(e.ID())
	if kind == "" {
		return e.Manifest.Description
	}
	pin, _ := languageextension.Package(kind)
	if e.Manifest.Version != pin.Version {
		return e.Manifest.Description + " Native adapter supports pinned " + pin.Version + " only; original JS activation is not supported."
	}
	return e.Manifest.Description + " Native LSP adapter: completion, diagnostics, formatting, hover and definition. Original VSIX JS, debugger/testing and manifest commands are not activated."
}

func nativeLanguageExtensionConfigs(installed []extensions.Extension, state extensionSettings, tools string) ([]languageserver.Config, []string) {
	var configs []languageserver.Config
	var issues []string
	for _, e := range installed {
		if languageextension.Kind(e.ID()) == "" || containsExtension(state.Disabled, e.ID()) || containsExtension(state.Uninstall, e.ID()) {
			continue
		}
		config, err := languageextension.Config(e, tools)
		if err != nil {
			issues = append(issues, err.Error())
			continue
		}
		configs = append(configs, config)
	}
	return configs, issues
}

// Call before Run or from a worker. Explicit lsp.json remains independent. The
// implicit Go fallback is suppressed after an adapter installation even when
// disabled/uninstalled, so it cannot resurrect the adapter-owned process.
func loadWorkbenchLanguageConfigs(explicit, extensionRoot string, installed []extensions.Extension, state extensionSettings) ([]languageserver.Config, []string, error) {
	tools, err := languageextension.ManagedTools()
	if err != nil {
		return nil, nil, err
	}
	configs, issues := nativeLanguageExtensionConfigs(installed, state, tools)
	configPath := explicit
	if configPath == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return nil, nil, err
		}
		candidate := filepath.Join(config, "gocode", "lsp.json")
		if _, err = os.Stat(candidate); err == nil {
			configPath = candidate
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
	}
	adapterGo, err := suppressImplicitGoFallback(extensionRoot, installed)
	if err != nil {
		return nil, nil, err
	}
	if configPath != "" || !adapterGo {
		independent, err := languageserver.LoadConfig(configPath)
		if err != nil {
			return nil, nil, err
		}
		for _, c := range independent {
			if strings.HasPrefix(c.Name, "extension:") {
				return nil, nil, errors.New("extension: language server names are reserved for native VSIX adapters")
			}
		}
		configs = append(configs, independent...) // explicit providers retain request precedence
	}
	if len(configs) > 16 {
		return nil, nil, errors.New("at most 16 language servers can be configured, including native VSIX adapters")
	}
	return configs, issues, nil
}

// Worker/startup only: mirror the installed-or-persistent-marker contract even
// when Disabled, unsupported versions or Uninstall produce no active Go config.
func suppressImplicitGoFallback(root string, installed []extensions.Extension) (bool, error) {
	for _, e := range installed {
		if languageextension.Kind(e.ID()) == "go" {
			return true, nil
		}
	}
	_, err := os.Lstat(filepath.Join(root, ".gocode-language", "go.json"))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func installLanguageExtension(ctx context.Context, root, kind string) (languageextension.Receipt, error) {
	tools, err := languageextension.ManagedTools()
	if err != nil {
		return languageextension.Receipt{}, err
	}
	return languageextension.Install(ctx, &http.Client{Timeout: 60 * time.Second}, root, tools, kind)
}

// Worker-only: a successful explicit reinstall cancels this package's deferred
// deletion. Disabled is an independent user preference and remains unchanged.
func finishLanguageExtensionInstall(root string, state extensionSettings, receipt languageextension.Receipt) (extensionSettings, error) {
	verified, err := languageextension.VerifyInstalled(root, receipt.Kind)
	if err != nil {
		return state, err
	}
	if verified.ID != receipt.ID || verified.Version != receipt.Version || verified.ArchiveSHA256 != receipt.ArchiveSHA256 {
		return state, errors.New("explicit language installation receipt changed")
	}
	copy := extensionSettings{Disabled: append([]string(nil), state.Disabled...), Uninstall: append([]string(nil), state.Uninstall...)}
	if containsExtension(copy.Uninstall, verified.ID) {
		copy.Uninstall = setExtension(copy.Uninstall, verified.ID, false)
		if err = writeExtensionSettings(root, copy); err != nil {
			return state, err
		}
	}
	return copy, nil
}

func checkLanguageExtension(ctx context.Context, root, kind string) (languageextension.CheckReport, error) {
	tools, err := languageextension.ManagedTools()
	if err != nil {
		return languageextension.CheckReport{}, err
	}
	return languageextension.Check(ctx, root, tools, kind)
}

// These versioned native actions are added to the workbench command inventory.
// Reuse the extension manager's single admission slot and worker shutdown.
func (m *model) languageExtensionCommands() []workbenchCommand {
	var result []workbenchCommand
	for _, kind := range []string{"go", "typescript"} {
		pin, _ := languageextension.Package(kind)
		name := "Go"
		if kind == "typescript" {
			name = "TypeScript"
		}
		result = append(result, workbenchCommand{ID: "installLanguage:" + kind, Title: fmt.Sprintf("Install %s Language Support (%s %s, native LSP)", name, pin.ID, pin.Version), Enabled: m.extensionsView.manage != nil && !m.extensionsView.busy, Run: func() { m.extensionsView.manage("languageInstall", kind) }})
	}
	return result
}
