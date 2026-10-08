package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gocode/internal/languageextension"
	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/gocode/internal/uidispatch"
	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/extensions"
)

type extensionSettings = languageextension.State
type extensionViewState struct {
	catalog                              extensionCatalog
	detailUI                             extensionDetailState
	contributions                        map[string][]extensionCommand
	query, detail, tab                   string
	focused, focusedBefore, busy, reload bool
	scroll                               int
	settings                             extensionSettings
	running                              map[string]bool
	manage                               func(string, string)
}

func containsExtension(ids []string, id string) bool {
	for _, known := range ids {
		if strings.EqualFold(known, id) {
			return true
		}
	}
	return false
}
func setExtension(ids []string, id string, enabled bool) []string {
	result := make([]string, 0, len(ids)+1)
	for _, known := range ids {
		if !strings.EqualFold(known, id) {
			result = append(result, known)
		}
	}
	if enabled {
		result = append(result, strings.ToLower(id))
	}
	return result
}
func readExtensionSettings(root string) (extensionSettings, error) {
	return languageextension.ReadState(root)
}
func writeExtensionSettings(root string, settings extensionSettings) error {
	if len(settings.Disabled) > 512 || len(settings.Uninstall) > 512 {
		return errors.New("extension state exceeds limit")
	}
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(root, ".state-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	_, err = f.Write(b)
	err = errors.Join(err, f.Sync(), f.Close())
	if err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(root, ".gocode-state.json"))
}

// Deletion is deferred until the old Node host has exited. Only direct, real
// directories whose bounded manifest matches the explicitly uninstalled ID are
// eligible. Hidden bundled assets and symlink/reparse entries are excluded.
func removePendingExtensions(root string, settings *extensionSettings) error {
	if len(settings.Uninstall) == 0 {
		return nil
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if filepath.Dir(path) != root {
			return errors.New("extension removal escaped root")
		}
		f, err := os.Open(filepath.Join(path, "package.json"))
		if err != nil {
			continue
		}
		data, readErr := io.ReadAll(io.LimitReader(f, (256<<10)+1))
		f.Close()
		if readErr != nil || len(data) > 256<<10 {
			continue
		}
		var manifest extensions.Manifest
		if json.Unmarshal(data, &manifest) != nil {
			continue
		}
		id := manifest.Publisher + "." + manifest.Name
		if containsExtension(settings.Uninstall, id) && strings.EqualFold(entry.Name(), id+"-"+manifest.Version) {
			if err := os.RemoveAll(path); err != nil {
				return err
			}
		}
	}
	settings.Uninstall = nil
	return writeExtensionSettings(root, *settings)
}
func extensionInfos(installed []extensions.Extension) []extensionInfo {
	result := make([]extensionInfo, 0, len(installed))
	for _, e := range installed {
		name := e.Manifest.DisplayName
		if name == "" {
			name = e.ID()
		}
		result = append(result, extensionInfo{name, e.ID(), e.Manifest.Description, e.Manifest.Version})
		if languageextension.Kind(e.ID()) != "" {
			result[len(result)-1].Description = languageExtensionDescription(e)
		}
	}
	return result
}
func enabledExtensions(installed []extensions.Extension, settings extensionSettings) []extensions.Extension {
	result := make([]extensions.Extension, 0, len(installed))
	for _, e := range installed {
		if !containsExtension(settings.Disabled, e.ID()) && !containsExtension(settings.Uninstall, e.ID()) {
			result = append(result, e)
		}
	}
	return result
}
func (m *model) startExtensionManager(parent context.Context, cx *ui.Context, root string) func() {
	return m.bindExtensionManager(parent, cx.Dispatch, root)
}

func (m *model) bindExtensionManager(parent context.Context, dispatch func(func()) bool, root string) func() {
	ctx, cancel := context.WithCancel(parent)
	var workers sync.WaitGroup
	m.extensionsView.manage = func(action, id string) {
		if m.extensionsView.busy {
			m.message = "An extension operation is already in progress"
			return
		}
		if action == "uninstall" && strings.EqualFold(id, bundledID) {
			return
		}
		settings := extensionSettings{Disabled: append([]string(nil), m.extensionsView.settings.Disabled...), Uninstall: append([]string(nil), m.extensionsView.settings.Uninstall...)}
		m.extensionsView.busy = true
		var catalog *catalogExtension
		if action == "catalogInstall" {
			for _, e := range m.extensionsView.catalog.results {
				if e.id() == id {
					copy := e
					catalog = &copy
					break
				}
			}
			if catalog == nil {
				m.extensionsView.busy = false
				m.message = "Extension search result expired; search again"
				return
			}
		}
		workers.Go(func() {
			var err error
			var installed []extensions.Extension
			var nativeConfigs []languageserver.Config
			var languageIssues []string
			var suppressAutomaticGo bool
			if ctx.Err() != nil {
				err = ctx.Err()
			} else {
				switch action {
				case "languageInstall":
					c, stop := context.WithTimeout(ctx, 120*time.Second)
					var receipt languageextension.Receipt
					receipt, err = installLanguageExtension(c, root, id)
					if err == nil {
						settings, err = finishLanguageExtensionInstall(root, settings, receipt)
					}
					stop()
				case "catalogInstall":
					c, stop := context.WithTimeout(ctx, 60*time.Second)
					archive, cleanup, failure := downloadCatalogVSIX(c, newCatalogClient(), root, *catalog)
					if failure == nil {
						var installed extensions.Extension
						err = validateCatalogVSIX(archive, *catalog)
						if err == nil {
							installed, err = extensions.Install(root, archive)
						}
						if err == nil && !strings.EqualFold(installed.ID(), catalog.id()) {
							err = errors.New("installed VSIX identity differs from catalog")
						}
						cleanup()
					} else {
						err = failure
					}
					stop()
				case "install":
					_, err = extensions.Install(root, id)
				case "enable":
					settings.Disabled = setExtension(settings.Disabled, id, false)
					err = writeExtensionSettings(root, settings)
				case "disable":
					settings.Disabled = setExtension(settings.Disabled, id, true)
					err = writeExtensionSettings(root, settings)
				case "uninstall":
					settings.Uninstall = setExtension(settings.Uninstall, id, true)
					err = writeExtensionSettings(root, settings)
				default:
					err = fmt.Errorf("unknown extension operation %q", action)
				}
			}
			if err == nil {
				installed, err = workbenchExtensions(root)
			}
			if err == nil {
				suppressAutomaticGo, err = suppressImplicitGoFallback(root, installed)
			}
			if err == nil {
				tools, failure := languageextension.ManagedTools()
				if failure != nil {
					err = failure
				} else {
					nativeConfigs, languageIssues = nativeLanguageExtensionConfigs(installed, settings, tools)
				}
			}
			uidispatch.Retry(ctx, dispatch, func() {
				if ctx.Err() != nil {
					return
				}
				m.extensionsView.busy = false
				if err != nil {
					m.message = err.Error()
					return
				}
				m.installed = extensionInfos(installed)
				m.extensionsView.contributions = extensionContributions(installed)
				m.extensionsView.settings = settings
				m.extensionsView.reload = true
				if failure := m.reconcileLanguageExtensionConfigs(nativeConfigs, suppressAutomaticGo); failure != nil {
					languageIssues = append(languageIssues, failure.Error())
				}
				m.message = strings.Join(languageIssues, "; ")
				// Publish only data. A late installation/settings acknowledgement
				// must not close a newer palette or steal editor/sidebar focus.
			})
		})
	}
	return func() {
		cancel()
		finished := make(chan struct{})
		go func() { workers.Wait(); close(finished) }()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
		}
	}
}
func extensionContributions(installed []extensions.Extension) map[string][]extensionCommand {
	result := map[string][]extensionCommand{}
	for _, e := range installed {
		if languageextension.Kind(e.ID()) != "" {
			continue
		}
		for _, c := range e.Manifest.Contributes.Commands {
			result[e.ID()] = append(result[e.ID()], extensionCommand{c.Command, c.Title})
		}
	}
	return result
}
