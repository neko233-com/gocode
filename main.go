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
	"runtime/pprof"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/neko233-com/gocode/internal/copilotservice"
	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/gocode/internal/terminal"
	"github.com/neko233-com/gocode/internal/update"
	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/extensions"
)

//go:embed bundled/*
var bundled embed.FS

//go:embed tools/copilot-runtime/package.json tools/copilot-runtime/package-lock.json
var copilotPackage embed.FS

const bundledID = "gocode.hello-native"
const bundledVersion = "0.4.0"

func installBundled(root string) error {
	installed, err := extensions.List(root)
	if err != nil {
		return err
	}
	for _, e := range installed {
		if e.ID() == bundledID && e.Manifest.Version == bundledVersion {
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

// Keep builtin payloads versioned and hidden from the shared user VSIX store.
// A rollback's older installer must still see its own builtin version there.
func workbenchExtensions(root string) ([]extensions.Extension, error) {
	managed := filepath.Join(root, ".gocode-bundled", bundledVersion)
	if err := installBundled(managed); err != nil {
		return nil, err
	}
	user, err := extensions.List(root)
	if err != nil {
		return nil, err
	}
	result := make([]extensions.Extension, 0, len(user)+1)
	for _, extension := range user {
		if !strings.EqualFold(extension.ID(), bundledID) {
			result = append(result, extension)
		}
	}
	known, err := extensions.List(managed)
	if err != nil {
		return nil, err
	}
	return append(result, known...), nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() (runErr error) {
	extensionGalleryURL := flag.String("extension-gallery-url", "", "Authorized VS Gallery base URL (Microsoft Marketplace requires separate service authorization)")
	keymapPreset := flag.String("keymap", "", "Built-in keyboard preset: vscode or jetbrains (session override)")
	showVersion := flag.Bool("version", false, "Print application version, platform and source commit")
	terminalRuntimeCheck := flag.Bool("terminal-runtime-check", false, "Verify the exact official ConPTY binaries embedded in this Windows executable")
	installTerminalTools := flag.Bool("install-terminal-tools", false, "Extract/install the pinned free official ConPTY runtime in gocode's owned Windows cache")
	updateCheck := flag.Bool("update-check", false, "Verify publisher release metadata and print the latest version")
	applyUpdate := flag.Bool("update", false, "Verify, stage, health-check and select an update for the next launch")
	rollbackUpdate := flag.Bool("update-rollback", false, "Restore the previous verified installed version")
	configureUpdates := flag.Bool("configure-updates", false, "Save update route/automatic settings and exit")
	updateMode := flag.String("update-mode", "", "Update route: auto, direct, mirror")
	updateMirror := flag.String("update-mirror", "", "Manual HTTPS prefix for GitHub downloads")
	updateAuto := flag.String("updates-auto", "", "Enable or disable automatic updates: true/false")
	workspace := flag.String("workspace", ".", "Workspace directory")
	windowWidth := flag.Int("window-width", 1280, "Initial native window width in DIP")
	windowHeight := flag.Int("window-height", 820, "Initial native window height in DIP")
	extensionDir := flag.String("extensions-dir", "", "Local VSIX installation directory")
	install := flag.String("install-extension", "", "Install a trusted local VSIX and exit")
	catalogCheck := flag.String("extension-catalog-check", "", "Verify actual Open VSX search and Windows x64/universal version metadata without installing")
	smoke := flag.Bool("smoke", false, "Verify a native frame and a real extension command, then exit")
	editorSmoke := flag.Bool("editor-smoke", false, "Verify native versioned VSIX edits, save, undo/redo and completion in a disposable workspace")
	closeSmoke := flag.String("close-smoke", "", "Verify native save/discard/cancel/external close protection in a disposable workspace")
	terminalSmoke := flag.Bool("terminal-smoke", false, "Verify real shell input highlighting, ANSI colors, keyboard, resize, interrupt and exit in a native owned workspace")
	terminalVSIXSmoke := flag.Bool("terminal-vsix-smoke", false, "Verify real VSIX-created native terminal processes, input, focus, pixels and lifecycle")
	scmSmoke := flag.Bool("scm-smoke", false, "Verify actual Git stage/unstage/commit and native side-by-side diff in an owned repository")
	replaceSmoke := flag.Bool("replace-smoke", false, "Verify native workspace replacement preview, stale disk guards, actual saves and undo in owned files")
	groupsSmoke := flag.Bool("groups-smoke", false, "Verify owned native editor splits, shared edits, independent caret/scroll, sash drag and scoped group saves")
	groupsVSIXSmoke := flag.Bool("groups-vsix-smoke", false, "Verify real VSIX editor views, columns, focus, selection, reveal, disposal and native save")
	groupsLargeSmoke := flag.Bool("groups-large-smoke", false, "Verify independent file-backed split pages and shared index lifetime")
	groupsLargeMiB := flag.Int("groups-large-mib", 32, "Native large split fixture size, 16–10240 MiB")
	filewatchSmoke := flag.Bool("filewatch-smoke", false, "Verify real external replacements, dirty conflict, native reload confirmation, VSIX and optional configured LSP in an owned workspace")
	largeSmoke := flag.Bool("largefile-smoke", false, "Verify file-backed browsing and direct long-line byte navigation in a native window")
	largeSmokeMiB := flag.Int("largefile-smoke-mib", 16, "Size of the native large-file acceptance fixture (16 to 10240 MiB)")
	goLine := flag.Int64("goto-line", 0, "Open at a 1-based line number")
	goByte := flag.Int64("goto-byte", -1, "Open a large file at a 0-based byte offset")
	lspConfig := flag.String("lsp-config", "", "User-owned JSON array of language server configurations")
	lspEnabled := flag.Bool("lsp", true, "Run configured standard language servers")
	lspSmoke := flag.Bool("lsp-smoke", false, "Verify real LSP formatting/hover/definition/completion/diagnostics in a native disposable workspace")
	openSmoke := flag.Bool("open-smoke", false, "Verify native typing/resize/cancel and awaited VSIX opens during delayed disk workers")
	uiSmoke := flag.Bool("ui-smoke", false, "Verify owned native workbench logo, complete tab captions and tab controls")
	extensionDetailSmoke := flag.Bool("extension-detail-smoke", false, "Verify native extension detail scrolling, measured fonts, real VSIX commands and management in an owned workspace")
	windowsWorkbenchSmoke := flag.Bool("windows-workbench-smoke", false, "Verify real Windows File menus, shell dialogs, quick input and VSIX management")
	autoSaveSmoke := flag.Bool("auto-save-smoke", false, "Verify native Auto Save modes, real disk writes, activation, conflicts and Revert File")
	autoSaveMinimizedSmoke := flag.Bool("auto-save-minimized-smoke", false, "Verify real Auto Save writes and UI receipts while an owned native window stays minimized")
	flag.String("auto-save", "", "Session Auto Save mode: off, afterDelay, onFocusChange, onWindowChange")
	flag.Int("auto-save-delay", 0, "Session Auto Save delay in milliseconds (100–600000)")
	tabsSmoke := flag.Bool("tabs-smoke", false, "Verify native tab overflow, wheel/drag routing, identity and held-Control navigation")
	searchSmoke := flag.Bool("search-smoke", false, "Verify native workspace search, unsaved/regex/ignore results, stale selection and large-file navigation")
	searchSmokeMiB := flag.Int("search-smoke-mib", 16, "Native search fixture size in MiB (16–10240)")
	installGopls := flag.Bool("install-gopls", false, "Install the pinned official gopls in gocode's per-user tools directory and exit")
	installCopilot := flag.Bool("install-copilot", false, "Install pinned official Copilot sidecars in gocode's per-user tools directory (requires Node.js/npm)")
	copilotRoot := flag.String("copilot-runtime", "", "Directory containing the pinned Copilot node_modules")
	copilotCheck := flag.Bool("copilot-check", false, "Verify official LSP/SDK initialization and authentication without an AI prompt")
	copilotSmoke := flag.Bool("copilot-smoke", false, "Verify official Copilot chat with a synthetic prompt (requires Copilot access)")
	copilotUISmoke := flag.Bool("copilot-ui-smoke", false, "Verify rendered Copilot suggestion, native Tab acceptance and chat in a disposable workspace")
	copilotEnabled := flag.Bool("copilot", true, "Connect installed official Copilot sidecars in the native workbench")
	flag.Parse()
	if *catalogCheck != "" {
		return checkLiveExtensionCatalog(*catalogCheck)
	}
	if *windowsWorkbenchSmoke {
		return runWindowsWorkbenchAcceptance()
	}
	if *extensionDetailSmoke {
		return runExtensionDetailAcceptance()
	}
	if *autoSaveSmoke {
		return runAutoSaveAcceptance()
	}
	if *autoSaveMinimizedSmoke {
		return runMinimizedAutoSaveAcceptance()
	}
	if *scmSmoke {
		return runSCMAcceptance()
	}
	if *terminalVSIXSmoke {
		return runTerminalVSIXAcceptance()
	}
	if *groupsLargeSmoke {
		return runLargeGroupsAcceptance(*groupsLargeMiB)
	}
	if *groupsSmoke {
		return runEditorGroupsAcceptance()
	}
	if *groupsVSIXSmoke {
		return runEditorGroupsVSIXAcceptance()
	}
	if *searchSmoke {
		return runSearchAcceptance(*searchSmokeMiB)
	}
	if *replaceSmoke {
		return runReplacementAcceptance()
	}
	if *tabsSmoke {
		return runEditorTabsAcceptance()
	}
	if *uiSmoke {
		return runWorkbenchUIAcceptance()
	}
	if *terminalRuntimeCheck {
		if err := terminal.VerifyEmbeddedConPTY(); err != nil {
			return err
		}
		fmt.Println("Verified official ConPTY", terminal.ConPTYVersion)
		return nil
	}
	if *installTerminalTools {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		path, err := terminal.EnsureConPTY(ctx, "")
		if err == nil {
			fmt.Println("Installed official ConPTY", terminal.ConPTYVersion, path)
		}
		return err
	}
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
	if *closeSmoke != "" {
		if *closeSmoke != "save" && *closeSmoke != "discard" && *closeSmoke != "cancel" && *closeSmoke != "external" {
			return errors.New("close-smoke must be save, discard, cancel or external")
		}
		fixture, err := os.MkdirTemp("", "gocode-close-acceptance-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(fixture)
		if err = os.WriteFile(filepath.Join(fixture, "main.go"), []byte(closeFixture), 0600); err != nil {
			return err
		}
		*workspace, paths = fixture, nil
		*copilotEnabled, *lspEnabled = false, false
	}
	if *terminalSmoke {
		if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
			return err
		}
		fixture, err := os.MkdirTemp("", "gocode-terminal-native-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(fixture)
		if err := os.WriteFile(filepath.Join(fixture, "main.go"), []byte("package main\nfunc main() {}\n"), 0600); err != nil {
			return err
		}
		*workspace, paths = fixture, nil
		*copilotEnabled, *lspEnabled = false, false
	}
	if *filewatchSmoke {
		fixture, err := os.MkdirTemp("", "gocode-filewatch-native-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(fixture)
		if err := os.WriteFile(filepath.Join(fixture, "main.go"), []byte(watchFixture), 0600); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(fixture, "go.mod"), []byte("module example.com/gocode-watch\n\ngo 1.27.0\n"), 0600); err != nil {
			return err
		}
		*workspace, paths = fixture, nil
		*copilotEnabled = false
	}
	if *extensionDir == "" {
		config, err := os.UserConfigDir()
		if err != nil {
			return err
		}
		*extensionDir = filepath.Join(config, "gocode", "extensions")
	}
	if *openSmoke {
		fixture, err := os.MkdirTemp("", "gocode-open-native-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(fixture)
		path := filepath.Join(fixture, "main.go")
		if err := os.WriteFile(path, []byte("package main\nfunc main() {}\n"), 0600); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(fixture, "README.md"), []byte(openReadmeFixture), 0600); err != nil {
			return err
		}
		*workspace, paths = fixture, []string{path}
		*copilotEnabled, *lspEnabled = false, false
	}
	if *install != "" {
		e, err := extensions.Install(*extensionDir, *install)
		if err != nil {
			return err
		}
		fmt.Println("Installed", e.ID(), e.Manifest.Version)
		return nil
	}
	m, err := newWorkspaceModel(*workspace)
	if err != nil {
		return err
	}
	defer m.closeDocuments()
	m.logo, err = loadWorkbenchLogo()
	if err != nil {
		return err
	}
	m.terminalAcceptance = *terminalSmoke
	gotoQuery := ""
	if *goByte >= 0 {
		gotoQuery = fmt.Sprintf(":%d", *goByte)
	} else if *goLine > 0 {
		gotoQuery = fmt.Sprint(*goLine)
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
	if *filewatchSmoke && *lspEnabled && len(languageConfigs) == 0 {
		return errors.New("native file watch LSP acceptance requires installed gopls; use -lsp=false for VSIX-only acceptance")
	}
	m.readClipboard, m.writeClipboard = ui.ReadClipboard, ui.WriteClipboard
	if *extensionGalleryURL != "" {
		if _, err = galleryURL(*extensionGalleryURL); err != nil {
			return err
		}
		m.extensionGallery = *extensionGalleryURL
	}
	keyboardPath, err := keyboardConfigPath()
	if err != nil {
		return err
	}
	if *keymapPreset != "" {
		if !validKeymap(*keymapPreset) {
			return errors.New("-keymap must be vscode or jetbrains")
		}
		m.keyboard.profile = *keymapPreset
	} else {
		var keyboardErr error
		m.keyboard.profile, keyboardErr = readKeymap(keyboardPath)
		if keyboardErr != nil {
			m.message = "Keyboard shortcuts: " + keyboardErr.Error()
		}
	}
	if *copilotCheck || *copilotSmoke {
		return checkCopilot(*copilotRoot, m.workspace, *copilotSmoke)
	}
	autoSavePath, err := autoSaveConfigPath()
	if err != nil {
		return err
	}
	m.autoSave.config, err = readAutoSaveConfig(autoSavePath)
	if err != nil {
		m.autoSave.config = defaultAutoSaveConfig()
		m.message = "Auto Save settings: " + err.Error()
	}
	if m.autoSave.config, err = autoSaveSessionConfig(m.autoSave.config, flag.CommandLine); err != nil {
		return err
	}
	installed, err := workbenchExtensions(*extensionDir)
	if err != nil {
		return err
	}
	extensionState, err := readExtensionSettings(*extensionDir)
	if err != nil {
		return err
	}
	if err = removePendingExtensions(*extensionDir, &extensionState); err != nil {
		return err
	}
	installed, err = workbenchExtensions(*extensionDir)
	if err != nil {
		return err
	}
	m.extensionsView.settings = extensionState
	m.extensionsView.contributions = extensionContributions(installed)
	m.extensionsView.running = map[string]bool{}
	activeExtensions := enabledExtensions(installed, extensionState)
	for _, e := range activeExtensions {
		m.extensionsView.running[strings.ToLower(e.ID())] = true
	}
	commandIDs := map[string]bool{}
	for _, e := range installed {
		name := e.Manifest.DisplayName
		if name == "" {
			name = e.ID()
		}
		m.installed = append(m.installed, extensionInfo{name, e.ID(), e.Manifest.Description, e.Manifest.Version})
		if !m.extensionsView.running[strings.ToLower(e.ID())] {
			continue
		}
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
	defer func() {
		if m.afterWindowClosed != nil {
			m.closeDocuments()
			runErr = errors.Join(runErr, m.afterWindowClosed())
		}
	}()
	host, hostErr := extensions.Start(hostCtx, m.workspace, activeExtensions)
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
			err := m.awaitExtensions(ctx)
			if err == nil {
				err = host.Call(ctx, "execute", map[string]string{"command": id}, nil)
			}
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
	var closeSaves func()
	var closeTerminals func()
	var closeWatches func()
	var closeOpens func()
	var closeWorkspace func()
	var closeSearch func()
	var closeHistory func()
	var closeSCM func()
	var closeFileActions, closeExtensionManager, closeExtensionCatalog, closeKeyboard func()
	var closeAutoSave, closeAutoSaveSettings func()
	defer func() {
		if closeAutoSave != nil {
			closeAutoSave()
		}
		if closeAutoSaveSettings != nil {
			closeAutoSaveSettings()
		}
		if closeKeyboard != nil {
			closeKeyboard()
		}
		if closeExtensionCatalog != nil {
			closeExtensionCatalog()
		}
		if closeFileActions != nil {
			closeFileActions()
		}
		if closeExtensionManager != nil {
			closeExtensionManager()
		}
		if closeSCM != nil {
			closeSCM()
		}
		if closeHistory != nil {
			closeHistory()
		}
		if closeSearch != nil {
			closeSearch()
		}
		if closeWorkspace != nil {
			closeWorkspace()
		}
		if closeOpens != nil {
			closeOpens()
		}
		if closeWatches != nil {
			closeWatches()
		}
		if closeTerminals != nil {
			closeTerminals()
		}
		if closeSaves != nil {
			closeSaves()
		}
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
	bootstrap := make(chan struct{})
	var bootstrapReady bool
	var bootstrapError error
	var openingAcceptance *openAcceptance
	if *openSmoke {
		openingAcceptance = newOpenAcceptance()
	}
	var aiAcceptance copilotAcceptance
	var largeAcceptance largefileAcceptance
	var languageAcceptance lspAcceptance
	closingAcceptance := closeAcceptance{mode: *closeSmoke}
	var shellAcceptance terminalAcceptance
	watchingAcceptance := filewatchAcceptance{requireLSP: *filewatchSmoke && *lspEnabled}
	deadline := 25 * time.Second
	if *copilotUISmoke {
		deadline = 90 * time.Second
	}
	if *lspSmoke {
		deadline = 90 * time.Second
	}
	if *terminalSmoke {
		deadline = 60 * time.Second
	}
	if *openSmoke {
		deadline = 60 * time.Second
	}
	if *filewatchSmoke {
		deadline = 90 * time.Second
	}
	if *largeSmoke && *largeSmokeMiB > 16 {
		deadline = 2 * time.Minute
	}
	watchdog := time.AfterFunc(deadline, func() {
		if *smoke || *copilotUISmoke || *largeSmoke || *lspSmoke || *closeSmoke != "" || *terminalSmoke || *filewatchSmoke || *openSmoke {
			fmt.Fprintln(os.Stderr, "gocode smoke timed out")
			if *terminalSmoke {
				_ = pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
			}
			os.Exit(2)
		}
	})
	defer watchdog.Stop()
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(m.workspace), Width: float32(*windowWidth), Height: float32(*windowHeight), Background: ui.RGB(editor), CustomTitlebar: true, Input: m.input, CloseRequested: m.requestWindowClose}, func(viewContext *ui.Context) *ui.Element {
		if !started {
			cx = viewContext
			m.native = viewContext
			var read func(context.Context, string) (*document, error)
			if *openSmoke {
				read = openingAcceptance.read
			}
			closeOpens = m.startFileOpens(hostCtx, viewContext.Dispatch, read)
			closeSearch = m.startSearch(hostCtx, viewContext.Dispatch, nil)
			closeSCM = m.startSCM(hostCtx, viewContext.Dispatch)
			closeHistory = m.startHistory(hostCtx, viewContext.Dispatch)
			closeSaves = m.startDocumentSaves(hostCtx, viewContext)
			closeFileActions = m.startFileActions(hostCtx, viewContext)
			closeKeyboard = m.startKeyboardSettings(viewContext, keyboardPath)
			closeAutoSave = m.startAutoSaveActor(hostCtx, viewContext.Dispatch)
			closeAutoSaveSettings = m.startAutoSaveSettings(hostCtx, viewContext.Dispatch, autoSavePath)
			closeExtensionManager = m.startExtensionManager(hostCtx, viewContext, *extensionDir)
			closeExtensionCatalog = m.startExtensionCatalog(hostCtx, viewContext.Dispatch, nil)
			m.bindWorkbenchRelaunch(hostCtx, viewContext, *extensionDir, *copilotRoot, *lspConfig, *copilotEnabled, *lspEnabled)
			closeWatches = m.startDocumentWatch(hostCtx, viewContext.Dispatch)
			closeTerminals = m.startTerminals(hostCtx, viewContext)
			closeIcon = applyAppIcon("gocode — " + filepath.Base(m.workspace))
			if !*smoke && !*largeSmoke && !*lspSmoke && !*copilotUISmoke && *closeSmoke == "" && !*terminalSmoke && !*filewatchSmoke && !*openSmoke {
				closeUpdates = m.startUpdates(hostCtx, viewContext, updateRoot, updateConfigPath, updateConfig)
				editing := m.editing
				m.newTerminal()
				m.terminalFocused, m.editing = false, editing
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
			var scan func(context.Context, string) workspaceScan
			if *openSmoke {
				scan = openingAcceptance.scan
			}
			closeWorkspace = m.startWorkspace(hostCtx, viewContext.Dispatch, paths, gotoQuery, scan, func(err error) {
				bootstrapError, bootstrapReady = err, true
				close(bootstrap)
				if err != nil {
					m.message = err.Error()
					if *smoke || *largeSmoke || *lspSmoke || *copilotUISmoke || *closeSmoke != "" || *terminalSmoke || *filewatchSmoke || *openSmoke {
						viewContext.Quit()
					}
				}
			})
			if host != nil {
				go func() {
					select {
					case <-initialized:
					case <-hostCtx.Done():
						return
					}
					for event := range host.Events {
						event := event
						if event.Type == "diagnostics" {
							physical, err := canonicalRPCPath(hostCtx, m.workspace, event.Path)
							if err != nil {
								continue
							}
							event.Path = physical
						}
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
					case <-bootstrap:
					case <-hostCtx.Done():
						return
					}
					if bootstrapError != nil {
						return
					}
					select {
					case <-initialized:
					case <-hostCtx.Done():
						return
					}
					ctx, stop := context.WithTimeout(hostCtx, 10*time.Second)
					defer stop()
					var value string
					err := m.awaitExtensions(ctx)
					if err == nil {
						err = host.Call(ctx, "execute", map[string]string{"command": "gocode.hello"}, &value)
					}
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
		if *copilotUISmoke && bootstrapReady && bootstrapError == nil {
			aiAcceptance.step(viewContext, m)
		}
		if *largeSmoke && bootstrapReady && bootstrapError == nil {
			largeAcceptance.step(viewContext, m)
		}
		if *lspSmoke && bootstrapReady && bootstrapError == nil {
			languageAcceptance.step(viewContext, m)
		}
		if *closeSmoke != "" && bootstrapReady && bootstrapError == nil {
			closingAcceptance.step(viewContext, m)
		}
		if *terminalSmoke && bootstrapReady && bootstrapError == nil {
			shellAcceptance.step(viewContext, m)
		}
		if *filewatchSmoke && bootstrapReady && bootstrapError == nil {
			watchingAcceptance.step(viewContext, m)
		}
		if *openSmoke && bootstrapReady && bootstrapError == nil {
			openingAcceptance.step(viewContext, m, hostCtx, host, initialized)
		}

		return m.view(viewContext)
	})
	if err != nil {
		return err
	}
	if bootstrapError != nil {
		return bootstrapError
	}
	if *openSmoke {
		if err := openingAcceptance.verify(m); err != nil {
			return err
		}
		fmt.Println("gocode async open acceptance passed: real disk/scan workers + native typing/resize/cancel + awaited VSIX + stale focus guards + unchanged source")
	}
	if *terminalSmoke {
		if !shellAcceptance.verified {
			return fmt.Errorf("terminal native acceptance: %s", shellAcceptance.failure)
		}
		fmt.Println("gocode terminal acceptance passed: real shell input syntax colors + native ANSI truecolor + keyboard + grid resize + Ctrl+C + exit + unchanged editor")
	}
	if *filewatchSmoke {
		if err := watchingAcceptance.verify(m); err != nil {
			return err
		}
		fmt.Printf("gocode file watch acceptance passed: actual atomic replacements + clean reload + dirty conflict + native cancel/confirm + EOL undo/redo + VSIX changes; configured LSP=%t\n", watchingAcceptance.requireLSP)
	}
	if *closeSmoke != "" {
		if err := closingAcceptance.verify(m); err != nil {
			return err
		}
		fmt.Println("gocode native close acceptance passed:", *closeSmoke)
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
		fmt.Println("gocode LSP acceptance passed: real formatting + hover + definition + completion + diagnostics + owned gopls crash/reinitialize + unsaved replay + recovered hover/completion")
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
