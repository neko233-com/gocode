package main

import (
	"archive/zip"
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"

	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/extensions"
)

//go:embed bundled/*
var bundled embed.FS

func installBundled(root string) error {
	installed, err := extensions.List(root)
	if err != nil {
		return err
	}
	for _, e := range installed {
		if e.ID() == "gocode.hello-native" {
			return nil
		}
	}
	f, err := os.CreateTemp("", "gocode-sample-*.vsix")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	z := zip.NewWriter(f)
	for _, p := range []string{"package.json", "extension.cjs"} {
		data, _ := bundled.ReadFile("bundled/" + p)
		w, err := z.Create("extension/" + p)
		if err != nil {
			f.Close()
			return err
		}
		if _, err = w.Write(data); err != nil {
			f.Close()
			return err
		}
	}
	if err = errors.Join(z.Close(), f.Close()); err != nil {
		return err
	}
	_, err = extensions.Install(root, name)
	return err
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	workspace := flag.String("workspace", ".", "Workspace directory")
	extensionDir := flag.String("extensions-dir", "", "Local VSIX installation directory")
	install := flag.String("install-extension", "", "Install a trusted local VSIX and exit")
	smoke := flag.Bool("smoke", false, "Verify a native frame and a real extension command, then exit")
	flag.Parse()
	if *extensionDir == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		*extensionDir = filepath.Join(config, "gocode", "extensions")
	}
	if *install != "" {
		e, err := extensions.Install(*extensionDir, *install)
		if err != nil {
			return err
		}
		fmt.Println("Installed", e.ID(), e.Manifest.Version)
		return nil
	}
	m, err := newModel(*workspace)
	if err != nil {
		return err
	}
	if err = installBundled(*extensionDir); err != nil {
		return err
	}
	installed, err := extensions.List(*extensionDir)
	if err != nil {
		return err
	}
	commandIDs := map[string]bool{}
	for _, e := range installed {
		name := e.Manifest.DisplayName
		if name == "" {
			name = e.ID()
		}
		m.installed = append(m.installed, extensionInfo{name, e.ID(), e.Manifest.Description, e.Manifest.Version})
		for _, c := range e.Manifest.Contributes.Commands {
			if c.Command == "" || commandIDs[c.Command] {
				continue
			}
			commandIDs[c.Command] = true
			title := c.Title
			if title == "" {
				title = c.Command
			}
			m.commands = append(m.commands, extensionCommand{c.Command, title})
		}
	}
	hostCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host, hostErr := extensions.Start(hostCtx, m.workspace, installed)
	if hostErr != nil {
		m.message = hostErr.Error()
		if *smoke {
			return hostErr
		}
	} else {
		defer host.Close()
		ctx, stop := context.WithTimeout(hostCtx, 10*time.Second)
		err = host.Call(ctx, "initialize", nil, nil)
		stop()
		if err != nil {
			m.message = err.Error()
			if *smoke {
				return err
			}
		}
	}
	var cx *ui.Context
	m.execute = func(id string) {
		if host == nil {
			m.message = "Extension host is unavailable"
			return
		}
		go func() {
			ctx, stop := context.WithTimeout(hostCtx, 10*time.Second)
			defer stop()
			err := host.Call(ctx, "execute", map[string]string{"command": id}, nil)
			if err != nil {
				cx.Dispatch(func() { m.message = err.Error() })
			}
		}()
	}
	var started bool
	var verified atomic.Bool
	watchdog := time.AfterFunc(25*time.Second, func() {
		if *smoke {
			fmt.Fprintln(os.Stderr, "gocode smoke timed out")
			os.Exit(2)
		}
	})
	defer watchdog.Stop()
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(m.workspace), Width: 1280, Height: 820, Background: ui.RGB(editor), CustomTitlebar: true, Input: m.input}, func(viewContext *ui.Context) *ui.Element {
		if !started {
			cx = viewContext
			started = true
			if host != nil {
				go func() {
					for event := range host.Events {
						event := event
						viewContext.Dispatch(func() {
							switch event.Type {
							case "information", "warning", "error":
								m.message = event.Text
							case "output":
								m.output = append(m.output, event.Text)
								if len(m.output) > 200 {
									m.output = m.output[len(m.output)-200:]
								}
							case "panel":
								m.panel = "OUTPUT"
								m.showPanel = true
							case "status":
								m.status = event.Text
							case "open":
								m.open(event.Path)
							}
						})
					}
				}()
			}
			if *smoke {
				go func() {
					ctx, stop := context.WithTimeout(hostCtx, 10*time.Second)
					defer stop()
					var value string
					err := host.Call(ctx, "execute", map[string]string{"command": "gocode.hello"}, &value)
					if err != nil || value != "hello-native" {
						fmt.Fprintln(os.Stderr, "extension smoke failed", err, value)
						viewContext.Quit()
						return
					}
					viewContext.Dispatch(func() { verified.Store(true) })
				}()
			}
		}
		if *smoke && verified.Load() {
			if viewContext.RenderedFrames() >= 2 {
				viewContext.Quit()
			} else {
				viewContext.Invalidate()
			}
		}

		return m.view(viewContext)
	})
	if *smoke {
		if !verified.Load() {
			return errors.New("extension command was not verified")
		}
		if cx == nil || cx.RenderedFrames() < 2 {
			return errors.New("native frame submissions were not verified")
		}
		fmt.Println("gocode smoke passed: native rendering + installed VSIX activation + command execution")
	}
	return err
}
