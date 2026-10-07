package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gocode/internal/uidispatch"
	"github.com/neko233-com/gocode/internal/update"
	ui "github.com/neko233-com/godesktop"
)

func updatePaths() (string, string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	root, rootErr := update.InstalledRoot(exe)
	return root, filepath.Join(config, "gocode", "updates.json"), rootErr
}
func updateCommand(ctx context.Context, root string, config update.Config, apply, rollback bool) error {
	key, err := update.PublisherKey()
	if err != nil {
		return err
	}
	if rollback {
		return update.Rollback(ctx, root, key)
	}
	manager := &update.Manager{Key: key, Config: config}
	if !apply {
		candidate, err := manager.Check(ctx)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"version": candidate.Manifest.Version, "source": candidate.Manifest.Source, "metadata": candidate.MetadataURL, "githubReachable": candidate.GitHubMetadataReachable})
	}
	result, err := manager.Apply(ctx, root, runtime.GOOS+"/"+runtime.GOARCH)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func (m *model) startUpdates(parent context.Context, cx *ui.Context, root, path string, config update.Config) func() {
	ctx, cancel := context.WithCancel(parent)
	var workers sync.WaitGroup
	m.updatesConfig = config
	m.updateMirrorDraft = config.Mirror
	m.configureUpdates = func(config update.Config) {
		workers.Go(func() {
			err := update.WriteConfig(path, config)
			uidispatch.Retry(ctx, cx.Dispatch, func() {
				if err != nil {
					m.updateStatus = err.Error()
					return
				}
				m.updatesConfig = config
				m.updateMirrorDraft = config.Mirror
				m.updateStatus = "Update settings saved"
			})
		})
	}
	if root == "" {
		m.updateStatus = "Install or extract a versioned package to enable updates"
		return func() { cancel(); workers.Wait() }
	}
	key, err := update.PublisherKey()
	if err != nil {
		m.updateStatus = err.Error()
		return func() { cancel(); workers.Wait() }
	}
	m.updateStatus = "Updates ready"
	m.requestUpdate = func() {
		if m.updateBusy {
			return
		}
		m.updateBusy = true
		m.updateStatus = "Checking verified releases…"
		config := m.updatesConfig
		workers.Go(func() {
			c, stop := context.WithTimeout(ctx, 3*time.Minute)
			defer stop()
			manager := &update.Manager{Key: key, Config: config}
			result, err := manager.Apply(c, root, runtime.GOOS+"/"+runtime.GOARCH)
			uidispatch.Retry(ctx, cx.Dispatch, func() {
				m.updateBusy = false
				if err != nil {
					m.updateStatus = "Update check: " + err.Error()
					return
				}
				if result.Ready {
					m.updateStatus = "Version " + result.Version + " ready for your next launch"
				} else {
					m.updateStatus = "Up to date (" + result.Version + ")"
				}
				host := "GitHub"
				if result.DownloadURL != "" && !strings.HasPrefix(result.DownloadURL, "https://github.com/") {
					host = "accelerated route"
				}
				m.updateRoute = host
			})
		})
	}
	// Automatic work uses the same verified pipeline and never closes an editor.
	workers.Go(func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		uidispatch.Retry(ctx, cx.Dispatch, func() {
			if m.updatesConfig.Auto {
				m.requestUpdate()
			}
		})
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				uidispatch.Retry(ctx, cx.Dispatch, func() {
					if m.updatesConfig.Auto {
						m.requestUpdate()
					}
				})
			}
		}
	})
	return func() { cancel(); workers.Wait() }
}

func (m *model) updatesSidebar() *ui.Element {
	mode := m.updatesConfig.Mode
	children := []*ui.Element{label("UPDATES").FontSize(11).Height(28), label("Automatic: " + fmt.Sprint(m.updatesConfig.Auto)).Height(24), button("Toggle automatic updates", "updates-auto", func(*ui.Context) {
		config := m.updatesConfig
		config.Auto = !config.Auto
		if m.configureUpdates != nil {
			m.configureUpdates(config)
		}
	}).Height(28)}
	for _, value := range []string{"auto", "direct", "mirror"} {
		title := map[string]string{"auto": "Automatic route", "direct": "Direct GitHub", "mirror": "Manual mirror"}[value]
		if mode == value {
			title = "✓ " + title
		}
		children = append(children, button(title, "updates-mode-"+value, func(*ui.Context) {
			config := m.updatesConfig
			config.Mode = value
			if value == "mirror" {
				config.Mirror = m.updateMirrorDraft
			}
			if m.configureUpdates != nil {
				m.configureUpdates(config)
			}
		}).Height(28))
	}
	children = append(children, label("Mirror URL (Enter to save)").Height(24).FontSize(11), label(m.updateMirrorDraft+"▏").Height(30).Background(ui.RGB(0x313131)).Key("updates-mirror-input").OnClick(func(*ui.Context) { m.updateFocused = true; m.editing = false; m.chatFocused = false }), button("Check / prepare update", "updates-check", func(*ui.Context) {
		if m.requestUpdate != nil {
			m.requestUpdate()
		}
	}).Height(30), label(m.updateRoute).Height(24).Foreground(ui.RGB(muted)))
	status := wrapChatText(m.updateStatus, 210)
	for _, line := range status[:min(8, len(status))] {
		children = append(children, label(line).Height(20).FontSize(11))
	}
	children = append(children, spacer())
	return ui.Column(children...).PaddingXY(12, 0)
}
