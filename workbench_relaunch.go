package main

import (
	"context"
	"os"
	"os/exec"
	"strconv"

	ui "github.com/neko233-com/godesktop"
)

func (m *model) bindWorkbenchRelaunch(parent context.Context, cx *ui.Context, extensionRoot, copilotRoot, lspConfig string, copilot, lsp bool) {
	m.relaunch = func(workspace string, closeOld bool) {
		args := []string{"-workspace", workspace, "-extensions-dir", extensionRoot, "-copilot-runtime", copilotRoot, "-lsp-config", lspConfig}
		args = append(args, "-keymap", m.keymapProfile())
		autoSave := m.autoSaveConfig()
		args = append(args, "-auto-save", autoSave.Mode, "-auto-save-delay", strconv.Itoa(autoSave.DelayMS))
		if m.extensionGallery != "" {
			args = append(args, "-extension-gallery-url", m.extensionGallery)
		}
		if !copilot {
			args = append(args, "-copilot=false")
		}
		if !lsp {
			args = append(args, "-lsp=false")
		}
		if workspace == m.workspace {
			for _, d := range m.docs {
				if !d.untitled {
					args = append(args, d.path)
				}
			}
		}
		launch := func() error {
			executable, err := os.Executable()
			if err != nil {
				return err
			}
			cmd := exec.Command(executable, args...)
			cmd.Dir = workspace
			configureWorkbenchProcess(cmd)
			if err = cmd.Start(); err != nil {
				return err
			}
			go func() { _ = cmd.Wait() }()
			return nil
		}
		if closeOld {
			// Deferred until native UI, disk workers, services and old Node host
			// exit. New-process uninstall cannot race the old extension runtime.
			m.afterWindowClosed = launch
			cx.Quit()
			return
		}
		go func() {
			if parent.Err() != nil {
				return
			}
			err := launch()
			if err != nil {
				cx.Dispatch(func() { m.message = "Open window: " + err.Error() })
			}
		}()
	}
}
