package main

import (
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"

	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/neko233-com/gocode/internal/copilotservice"
	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/gocode/internal/update"
	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/extensions"
)

//go:embed bundled/*
var bundled embed.FS

//go:embed tools/copilot-runtime/package.json tools/copilot-runtime/package-lock.json
var copilotPackage embed.FS

func installBundled(root string) error {
	installed, err := extensions.List(root)
	if err != nil {
		return err
	}
	for _, e := range installed {
		if e.ID() == "gocode.hello-native" && e.Manifest.Version == "0.3.0" {
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
	showVersion := flag.Bool("version", false, "Print application version, platform and source commit")
	updateCheck := flag.Bool("update-check", false, "Verify publisher release metadata and print the latest version")
	applyUpdate := flag.Bool("update", false, "Verify, stage, health-check and select an update for the next launch")
	rollbackUpdate := flag.Bool("update-rollback", false, "Restore the previous verified installed version")
	configureUpdates := flag.Bool("configure-updates", false, "Save update route/automatic settings and exit")
	updateMode := flag.String("update-mode", "", "Update route: auto, direct, mirror")
	updateMirror := flag.String("update-mirror", "", "Manual HTTPS prefix for GitHub downloads")
	updateAuto := flag.String("updates-auto", "", "Enable or disable automatic updates: true/false")
	workspace := flag.String("workspace", ".", "Workspace directory")
	extensionDir := flag.String("extensions-dir", "", "Local VSIX installation directory")
	install := flag.String("install-extension", "", "Install a trusted local VSIX and exit")
	smoke := flag.Bool("smoke", false, "Verify a native frame and a real extension command, then exit")
	editorSmoke := flag.Bool("editor-smoke", false, "Verify native versioned VSIX edits, save, undo/redo and completion in a disposable workspace")
	largeSmoke := flag.Bool("largefile-smoke", false, "Verify file-backed browsing and direct long-line byte navigation in a native window")
	largeSmokeMiB := flag.Int("largefile-smoke-mib", 16, "Size of the native large-file acceptance fixture (16 to 10240 MiB)")
	goLine := flag.Int64("goto-line", 0, "Open at a 1-based line number")
	goByte := flag.Int64("goto-byte", -1, "Open a large file at a 0-based byte offset")
	lspConfig := flag.String("lsp-config", "", "User-owned JSON array of language server configurations")
	lspEnabled := flag.Bool("lsp", true, "Run configured standard language servers")
	lspSmoke := flag.Bool("lsp-smoke", false, "Verify real LSP formatting/hover/definition/completion/diagnostics in a native disposable workspace")
	installGopls := flag.Bool("install-gopls", false, "Install the pinned official gopls in gocode's per-user tools directory and exit")
	installCopilot := flag.Bool("install-copilot", false, "Install pinned official Copilot sidecars in gocode's per-user tools directory (requires Node.js/npm)")
	copilotRoot := flag.String("copilot-runtime", "", "Directory containing the pinned Copilot node_modules")
	copilotCheck := flag.Bool("copilot-check", false, "Verify official LSP/SDK initialization and authentication without an AI prompt")
	copilotSmoke := flag.Bool("copilot-smoke", false, "Verify official Copilot chat with a synthetic prompt (requires Copilot access)")
	copilotUISmoke := flag.Bool("copilot-ui-smoke", false, "Verify rendered Copilot suggestion, native Tab acceptance and chat in a disposable workspace")
	copilotEnabled := flag.Bool("copilot", true, "Connect installed official Copilot sidecars in the native workbench")
	flag.Parse()
	if *showVersion {
		fmt.Printf("gocode %s %s/%s %s\n", appVersion(), runtime.GOOS, runtime.GOARCH, buildCommit())
		return nil
	}
	updateRoot, updateConfigPath, updateRootErr := updatePaths()
	updateConfig, err := update.LoadConfig(updateConfigPath)
	if err != nil {
		return err
	}
	if *updateMode != "" {
		updateConfig.Mode = *updateMode
	}
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "update-mirror" {
			updateConfig.Mirror = *updateMirror
		}
	})
	if *updateAuto != "" {
		value, err := strconv.ParseBool(*updateAuto)
		if err != nil {
			return err
		}
		updateConfig.Auto = value
	}
	if *configureUpdates {
		return update.WriteConfig(updateConfigPath, updateConfig)
	}
	if *applyUpdate || *rollbackUpdate || *updateCheck {
		if !*updateCheck && updateRootErr != nil {
			return updateRootErr
		}
		ctx, stop := context.WithTimeout(context.Background(), 3*time.Minute)
		defer stop()
		return updateCommand(ctx, updateRoot, updateConfig, *applyUpdate, *rollbackUpdate)
	}
	if *installGopls {
		ctx, stop := context.WithTimeout(context.Background(), 10*time.Minute)
		defer stop()
		path, err := languageserver.InstallGopls(ctx)
		if err == nil {
			fmt.Println("Installed gopls", languageserver.GoplsVersion, path)
		}
		return err
	}
	if *installCopilot {
		ctx, stop := context.WithTimeout(context.Background(), 10*time.Minute)
		defer stop()
		packageJSON, err := copilotPackage.ReadFile("tools/copilot-runtime/package.json")
		if err != nil {
			return err
		}
		lockJSON, err := copilotPackage.ReadFile("tools/copilot-runtime/package-lock.json")
		if err != nil {
			return err
		}
		path, err := copilotservice.InstallRuntime(ctx, packageJSON, lockJSON)
		if err == nil {
			fmt.Println("Installed pinned official Copilot runtime", path)
		}
		return err
	}
	paths := flag.Args()
	if len(paths) > 0 {
		first, err := filepath.Abs(paths[0])
		if err != nil {
			return err
		}
		info, err := os.Stat(first)
		if err != nil {
			return err
		}
		explicitWorkspace := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "workspace" {
				explicitWorkspace = true
			}
		})
		if info.IsDir() {
			if len(paths) > 1 {
				return errors.New("pass a directory or one or more files")
			}
			*workspace = first
			paths = nil
		} else if !explicitWorkspace {
			*workspace = filepath.Dir(first)
		}
	}
	if *largeSmoke {
		if *largeSmokeMiB < 16 || *largeSmokeMiB > 10240 {
			return errors.New("large-file acceptance size must be 16 to 10240 MiB")
		}
		fixture, err := os.MkdirTemp("", "gocode-large-native-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(fixture)
		path := filepath.Join(fixture, "huge.txt")
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		block := bytes.Repeat([]byte("0123456789abcdef"), 4096)
		for i := 0; i < *largeSmokeMiB*16; i++ {
			if _, err = f.Write(block); err != nil {
				f.Close()
				return err
			}
		}
		_, err = f.WriteString(" NATIVE_LARGEFILE_TAIL\nlast line\n")
		err = errors.Join(err, f.Close())
		if err != nil {
			return err
		}
		*workspace = fixture
		paths = []string{path}
		*copilotEnabled = false
	}
	if *lspSmoke {
		fixture, err := os.MkdirTemp("", "gocode-lsp-native-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(fixture)
		if err := os.WriteFile(filepath.Join(fixture, "go.mod"), []byte("module example.com/gocode-acceptance\n\ngo 1.27.0\n"), 0600); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(fixture, "main.go"), []byte("package main\nfunc greeting() string{return \"hello\"}\nfunc main(){\n_ = greeting()\n_ = missing\n}\n"), 0600); err != nil {
			return err
		}
		*workspace = fixture
		paths = nil
		*copilotEnabled = false
	}
	if *copilotUISmoke {
		fixture, err := os.MkdirTemp("", "gocode-copilot-ui-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(fixture)
		if err = os.WriteFile(filepath.Join(fixture, "main.go"), []byte("package main\n\n// add returns the sum of two integers.\nfunc add(a,b int) int {\n    "), 0644); err != nil {
			return err
		}
		*workspace = fixture
	}
	if *editorSmoke {
		fixture, err := os.MkdirTemp("", "gocode-editor-acceptance-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(fixture)
		if err = os.WriteFile(filepath.Join(fixture, "main.go"), []byte("package main\r\n// TODO acceptance fixture\r\nfunc main() {}\r\n"), 0644); err != nil {
			return err
		}
		*workspace = fixture
		*smoke = true
	}
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
	defer m.closeDocuments()
	for _, path := range paths {
		m.open(path)
		if m.findDocument(path) == nil {
			return errors.New(m.message)
		}
	}
	if *goByte >= 0 {
		m.goTo(fmt.Sprintf(":%d", *goByte))
	} else if *goLine > 0 {
		m.goTo(fmt.Sprint(*goLine))
	}
	var languageConfigs []languageserver.Config
	if *lspEnabled {
		languageConfigs, err = languageserver.LoadConfig(*lspConfig)
		if err != nil {
			return err
		}
	}
	if *lspSmoke && len(languageConfigs) == 0 {
		return errors.New("LSP native acceptance requires installed gopls or -lsp-config")
	}
	m.readClipboard, m.writeClipboard = ui.ReadClipboard, ui.WriteClipboard
	if *copilotCheck || *copilotSmoke {
		return checkCopilot(*copilotRoot, m.workspace, *copilotSmoke)
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
	}
	var cx *ui.Context
	var initialized <-chan struct{}
	m.execute = func(id string) {
		if host == nil {
			m.message = "Extension host is unavailable"
			return
		}
		go func() {
			if initialized != nil {
				select {
				case <-initialized:
				case <-hostCtx.Done():
					return
				}
			}
			ctx, stop := context.WithTimeout(hostCtx, 10*time.Second)
			defer stop()
			err := host.Call(ctx, "execute", map[string]string{"command": id}, nil)
			if err != nil {
				cx.Dispatch(func() { m.message = err.Error() })
			}
		}()
	}
	var started bool
	var closeCopilot func()
	var closeLanguages func()
	var closeUpdates func()
	var closeIcon func()
	defer func() {
		if closeUpdates != nil {
			closeUpdates()
		}
		if closeIcon != nil {
			closeIcon()
		}
		if closeLanguages != nil {
			closeLanguages()
		}
		if closeCopilot != nil {
			closeCopilot()
		}
	}()
	var verified atomic.Bool
	var aiAcceptance copilotAcceptance
	var largeAcceptance largefileAcceptance
	var languageAcceptance lspAcceptance
	deadline := 25 * time.Second
	if *copilotUISmoke {
		deadline = 90 * time.Second
	}
	if *lspSmoke {
		deadline = 60 * time.Second
	}
	if *largeSmoke && *largeSmokeMiB > 16 {
		deadline = 2 * time.Minute
	}
	watchdog := time.AfterFunc(deadline, func() {
		if *smoke || *copilotUISmoke || *largeSmoke || *lspSmoke {
			fmt.Fprintln(os.Stderr, "gocode smoke timed out")
			os.Exit(2)
		}
	})
	defer watchdog.Stop()
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(m.workspace), Width: 1280, Height: 820, Background: ui.RGB(editor), CustomTitlebar: true, Input: m.input}, func(viewContext *ui.Context) *ui.Element {
		if !started {
			cx = viewContext
			m.native = viewContext
			closeIcon = applyAppIcon("gocode — " + filepath.Base(m.workspace))
			if !*smoke && !*largeSmoke && !*lspSmoke && !*copilotUISmoke {
				closeUpdates = m.startUpdates(hostCtx, viewContext, updateRoot, updateConfigPath, updateConfig)
			}
			started = true
			if host != nil {
				initialized = m.startExtensions(hostCtx, viewContext, host, filepath.Join(filepath.Dir(*extensionDir), "state"))
			}
			if *copilotEnabled && !*smoke {
				closeCopilot = m.startCopilot(hostCtx, viewContext, *copilotRoot)
			}
			if len(languageConfigs) > 0 {
				closeLanguages = m.startLanguages(hostCtx, viewContext, languageConfigs)
			}
			if host != nil {
				go func() {
					select {
					case <-initialized:
					case <-hostCtx.Done():
						return
					}
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
							case "clear":
								m.output = nil
							case "diagnostics":
								m.setDiagnostics(event.Path, event.Channel, event.Data)
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
					select {
					case <-initialized:
					case <-hostCtx.Done():
						return
					}
					ctx, stop := context.WithTimeout(hostCtx, 10*time.Second)
					defer stop()
					var value string
					err := host.Call(ctx, "execute", map[string]string{"command": "gocode.hello"}, &value)
					if err != nil || value != "hello-native" {
						fmt.Fprintln(os.Stderr, "extension smoke failed", err, value)
						viewContext.Quit()
						return
					}
					if *editorSmoke {
						if err := verifyNativeEditor(ctx, viewContext, m, host); err != nil {
							fmt.Fprintln(os.Stderr, "editor acceptance failed:", err)
							viewContext.Quit()
							return
						}
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
		if *copilotUISmoke {
			aiAcceptance.step(viewContext, m)
		}
		if *largeSmoke {
			largeAcceptance.step(viewContext, m)
		}
		if *lspSmoke {
			languageAcceptance.step(viewContext, m)
		}

		return m.view(viewContext)
	})
	if err != nil {
		return err
	}
	if *largeSmoke {
		if !largeAcceptance.verified {
			return fmt.Errorf("large-file native acceptance: %s", largeAcceptance.failure)
		}
		fmt.Println("gocode large-file acceptance passed: file-backed native page + direct long-line byte navigation + unchanged source")
	}
	if *lspSmoke {
		if !languageAcceptance.verified {
			return fmt.Errorf("LSP native acceptance: %s", languageAcceptance.failure)
		}
		fmt.Println("gocode LSP acceptance passed: real formatting + hover + definition + completion + versioned diagnostics and clearing")
	}
	if *copilotUISmoke {
		if !aiAcceptance.verified {
			return fmt.Errorf("native Copilot acceptance: %s", aiAcceptance.failure)
		}
		fmt.Println("gocode Copilot UI acceptance passed: rendered official inline suggestion + native Tab edit + official SDK chat + cancellation and retry")
	}
	if *smoke {
		if !verified.Load() {
			return errors.New("extension command was not verified")
		}
		if cx == nil || cx.RenderedFrames() < 2 {
			return errors.New("native frame submissions were not verified")
		}
		fmt.Println("gocode smoke passed: native rendering + installed VSIX activation + command execution")
		if *editorSmoke {
			fmt.Println("gocode editor acceptance passed: UTF-16 VSIX edit + CRLF save + undo/redo + versioned completion")
		}
	}
	return nil
}
