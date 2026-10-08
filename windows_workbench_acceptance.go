//go:build windows && cgo

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/extensions"
	"github.com/neko233-com/godesktop/testing/winprobe"
	"golang.org/x/sys/windows"
)

func runWindowsWorkbenchAcceptance() error {
	os.Setenv("GODESKTOP_READBACK", "1")
	os.Setenv("GODESKTOP_TEST_INPUT_ISOLATION", "1")
	root, err := os.MkdirTemp("", "gocode-windows-workbench-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	readme := filepath.Join(root, "README.md")
	saved := filepath.Join(root, "保存副本.go")
	if err = os.WriteFile(readme, []byte("# Real Windows File menu\r\nUnicode 世界 😀\r\n"), 0600); err != nil {
		return err
	}
	archive := filepath.Join(root, "fixture.vsix")
	f, err := os.Create(archive)
	if err != nil {
		return err
	}
	writer := zip.NewWriter(f)
	manifest := `{"publisher":"fixture","name":"workbench","displayName":"Windows Workbench Fixture","description":"Real VSIX management and contributed commands","version":"0.1.0","main":"extension.cjs","activationEvents":["*"],"contributes":{"commands":[{"command":"owned.verify","title":"Verify Windows Extension"}]}}`
	for _, entry := range []struct{ name, text string }{{"package.json", manifest}, {"extension.cjs", `exports.activate=c=>c.subscriptions.push(require('vscode').commands.registerCommand('owned.verify',()=> 'real extension result'));`}} {
		part, e := writer.Create("extension/" + entry.name)
		if e != nil {
			return e
		}
		if _, e = part.Write([]byte(entry.text)); e != nil {
			return e
		}
	}
	if err = errors.Join(writer.Close(), f.Close()); err != nil {
		return err
	}
	extRoot := filepath.Join(root, "extensions")
	fixture, err := extensions.Install(extRoot, archive)
	if err != nil {
		return err
	}
	m, err := newWorkspaceModel(root)
	if err != nil {
		return err
	}
	m.files = []string{"README.md"}
	m.showPanel = false
	m.logo, err = loadWorkbenchLogo()
	if err != nil {
		return err
	}
	m.installed = extensionInfos([]extensions.Extension{fixture})
	m.extensionsView.contributions = extensionContributions([]extensions.Extension{fixture})
	m.extensionsView.running = map[string]bool{"fixture.workbench": true}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer m.closeDocuments()
	var stopSaves, stopOpens, stopFiles, stopManager, stopKeyboard func()
	var keyboardWriterFinished <-chan struct{}
	defer func() {
		cancel()
		for _, stop := range []func(){stopKeyboard, stopManager, stopFiles, stopOpens, stopSaves} {
			if stop != nil {
				stop()
			}
		}
	}()
	phase, frame, busy := 0, uint64(8), false
	capturePhase := -1
	var failure error
	var native *ui.Context
	watchdog := time.AfterFunc(60*time.Second, func() { fmt.Fprintf(os.Stderr, "Windows workbench acceptance timed out\n"); os.Exit(2) })
	defer watchdog.Stop()
	width, height := float32(1280), float32(820)
	smallWindow := os.Getenv("GOCODE_TEST_SMALL_WINDOW") == "1"
	if smallWindow {
		width, height = 1024, 728
	}
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(root), Width: width, Height: height, CustomTitlebar: true, Input: m.input, Background: ui.RGB(editor)}, func(cx *ui.Context) *ui.Element {
		if native == nil {
			native = cx
			m.native = cx
			stopOpens = m.startFileOpens(ctx, cx.Dispatch, nil)
			stopSaves = m.startDocumentSaves(ctx, cx)
			stopFiles = m.startFileActions(ctx, cx)
			stopManager = m.startExtensionManager(ctx, cx, extRoot)
			stopKeyboard, keyboardWriterFinished = m.bindKeyboardSettingsWithDrain(context.Background(), cx.Dispatch, filepath.Join(root, "keyboard.json"))
			window, probeErr := winprobe.Find("gocode — "+filepath.Base(root), uint32(os.Getpid()))
			if probeErr == nil {
				var clientWidth, clientHeight int
				clientWidth, clientHeight, probeErr = window.ClientSize()
				if probeErr == nil {
					dpi := window.DPI()
					fmt.Printf("native workbench condition pid=%d requested_dip=%.0fx%.0f window_dpi=%d client_px=%dx%d small=%t\n", os.Getpid(), width, height, dpi, clientWidth, clientHeight, smallWindow)
					if os.Getenv("GOCODE_TEST_DPIUNAWARE") == "1" && dpi != 96 {
						probeErr = fmt.Errorf("owned workbench DPI is %d, expected actual 96", dpi)
					}
				}
			}
			if probeErr != nil {
				failure = fmt.Errorf("owned workbench native condition: %w", probeErr)
				cx.Quit()
			}
		}
		advance := func(err error) {
			busy = false
			if err != nil {
				failure = fmt.Errorf("phase %d (fileBusy=%t menu=%s message=%q): %w", phase, m.fileActions.busy, m.menu.name, m.message, err)
				cx.Quit()
				return
			}
			phase++
			frame = cx.RenderedFrames() + 5
		}
		click := func(key string) {
			b, ok := cx.ElementBounds(key)
			if !ok {
				failure = fmt.Errorf("phase %d missing control %s", phase, key)
				cx.Quit()
				return
			}
			busy = true
			activateFileWatchControl(cx, root, b, func() {}, advance)
		}
		menuClick := func(id string) {
			for i, e := range m.menuEntries(m.menu.name) {
				if e.ID == id {
					click(menuRowKey(0, i))
					return
				}
			}
			failure = fmt.Errorf("missing menu command %s", id)
			cx.Quit()
		}
		key := func(k, mods int) { busy = true; tabsNativeKey(cx, root, k, mods, true, advance) }
		tap := func(k int) {
			busy = true
			tabsNativeKey(cx, root, k, 0, true, func(e error) {
				if e != nil {
					advance(e)
					return
				}
				tabsNativeKey(cx, root, k, 0, false, advance)
			})
		}
		text := func(s string) { busy = true; searchNativeText(cx, root, s, advance) }
		capture := func(stage string) bool {
			// An asynchronous acknowledgement can arrive after the old frame
			// threshold. Wait for completed submissions of the acknowledged UI.
			if capturePhase != phase {
				capturePhase = phase
				frame = cx.RenderedFrames() + 3
				return false
			}
			pixels, _, e := captureWorkbenchGPU(cx, root)
			if e == nil {
				dir := os.Getenv("GOCODE_WINDOWS_WORKBENCH_SCREENSHOTS")
				if dir == "" {
					dir = filepath.Join(".cache", "windows-workbench")
				}
				e = os.MkdirAll(dir, 0755)
				if e == nil {
					var file *os.File
					file, e = os.Create(filepath.Join(dir, stage+".png"))
					if e == nil {
						e = errors.Join(png.Encode(file, pixels), file.Close())
					}
				}
			}
			if e != nil {
				failure = e
				cx.Quit()
				return false
			}
			return true
		}
		if !busy && cx.RenderedFrames() >= frame && failure == nil {
			switch phase {
			case 0:
				click("menu-File")
			case 1:
				if m.menu.name != "File" || m.palette {
					failure = errors.New("File opened wrong surface")
					cx.Quit()
					break
				}
				b, ok := cx.ElementBounds(menuRowKey(0, 0))
				if !ok || b.Height != 24 {
					failure = errors.New("menu row is not upstream 24 DIP")
					cx.Quit()
					break
				}
				if capture("file-menu") {
					menuClick("newFile")
				}
			case 2:
				if m.current() == nil || !m.current().untitled {
					failure = errors.New("File > New Text File did not create an editor")
					cx.Quit()
					break
				}
				text("// 原生 File 世界 😀")
			case 3:
				click("menu-File")
			case 4:
				menuClick("saveAs")
			case 5:
				busy = true
				go func() { e := chooseOwnedWindowsDialog(root, saved, false); cx.Dispatch(func() { advance(e) }) }()
			case 6:
				if m.fileActions.busy {
					break
				}
				if m.current() == nil || m.current().path != saved || m.current().dirty() || m.current().untitled {
					failure = fmt.Errorf("Save As did not acknowledge exact editor: %s", m.message)
					cx.Quit()
					break
				}
				if capture("saved-untitled") {
					click("menu-File")
				}
			case 7:
				menuClick("openFile")
			case 8:
				busy = true
				go func() { e := chooseOwnedWindowsDialog(root, readme, false); cx.Dispatch(func() { advance(e) }) }()
			case 9:
				if m.openBusy || m.fileActions.busy {
					break
				}
				if m.current() == nil || m.current().path != readme {
					failure = errors.New("native Open File did not open selected disk file")
					cx.Quit()
					break
				}
				if capture("opened-file") {
					click("menu-File")
				}
			case 10:
				menuClick("openFile")
			case 11:
				busy = true
				go func() { e := chooseOwnedWindowsDialog(root, "", true); cx.Dispatch(func() { advance(e) }) }()
			case 12:
				if m.fileActions.busy {
					break
				}
				if m.current().path != readme {
					failure = errors.New("cancelled Open File changed active document")
					cx.Quit()
					break
				}
				key('P', ui.ModifierControl|ui.ModifierShift)
			case 13:
				text("New Text")
			case 14:
				if !m.palette || !strings.HasPrefix(m.query, ">") || len(m.quickItems()) != 1 {
					failure = errors.New("command palette query was not filtered")
					cx.Quit()
					break
				}
				if capture("command-palette") {
					key(13, 0)
				}
			case 15:
				if !m.current().untitled {
					failure = errors.New("palette Enter did not execute command")
					cx.Quit()
					break
				}
				key('P', ui.ModifierControl)
			case 16:
				text("README")
			case 17:
				if !m.palette || strings.HasPrefix(m.query, ">") || len(m.quickItems()) != 1 {
					failure = errors.New("Ctrl+P did not open file picker")
					cx.Quit()
					break
				}
				if capture("quick-open") {
					key(13, 0)
				}
			case 18:
				if m.current().path != readme {
					break
				}
				click("activity-extensions")
			case 19:
				text("@installed workbench")
			case 20:
				if len(m.filteredExtensions()) != 1 {
					failure = errors.New("extension search did not filter installed VSIX")
					cx.Quit()
					break
				}
				if capture("extensions-installed") {
					click("extension-card-fixture.workbench")
				}
			case 21:
				if m.extensionsView.detail != "fixture.workbench" {
					failure = errors.New("extension card did not open native details")
					cx.Quit()
					break
				}
				if capture("extension-details") {
					click("extension-toggle")
				}
			case 22:
				if m.extensionsView.busy {
					break
				}
				if !containsExtension(m.extensionsView.settings.Disabled, "fixture.workbench") || !m.extensionsView.reload {
					failure = errors.New("Disable did not persist reload-required state")
					cx.Quit()
					break
				}
				if capture("extension-disabled") {
					click("extension-toggle")
				}
			case 23:
				if m.extensionsView.busy {
					break
				}
				if containsExtension(m.extensionsView.settings.Disabled, "fixture.workbench") {
					failure = errors.New("Enable did not reverse state")
					cx.Quit()
					break
				}
				click("extension-uninstall")
			case 24:
				if m.extensionsView.busy {
					break
				}
				if !containsExtension(m.extensionsView.settings.Uninstall, "fixture.workbench") {
					failure = errors.New("Uninstall was not scheduled for host shutdown")
					cx.Quit()
					break
				}
				phase++
				frame = cx.RenderedFrames() + 5
			case 25:
				key(121, 0)
			case 26:
				if m.menu.name != "File" {
					failure = errors.New("F10 did not open File")
					cx.Quit()
					break
				}
				key(40, 0)
			case 27:
				if m.menu.index != 0 {
					failure = errors.New("keyboard did not select first menu row")
					cx.Quit()
					break
				}
				key(39, 0)
			case 28:
				if m.menu.name != "Edit" {
					failure = errors.New("Right did not switch to Edit")
					cx.Quit()
					break
				}
				if capture("keyboard-edit-menu") {
					key(27, 0)
				}
			case 29:
				key('F', ui.ModifierAlt)
			case 30:
				if m.menu.name != "File" {
					failure = errors.New("real Alt+F system key did not open File")
					cx.Quit()
					break
				}
				b, ok := cx.ElementBounds("menu-Edit")
				if !ok {
					failure = errors.New("Edit menubar target missing")
					cx.Quit()
					break
				}
				busy = true
				go func() {
					w, e := winprobe.Find("gocode — "+filepath.Base(root), uint32(os.Getpid()))
					if e == nil {
						e = w.Pointer(0x200, int(b.X+b.Width/2), int(b.Y+b.Height/2))
					}
					cx.Dispatch(func() { advance(e) })
				}()
			case 31:
				if m.menu.name != "Edit" {
					failure = errors.New("unpressed native hover did not switch menus")
					cx.Quit()
					break
				}
				if capture("hover-edit-menu") {
					key(27, 0)
				}
			case 32:
				if m.menu.name != "" {
					failure = errors.New("Escape did not dismiss menu")
					cx.Quit()
					break
				}
				key(188, ui.ModifierControl)
			case 33:
				if m.activity != "settings" {
					failure = errors.New("VS Code settings shortcut failed")
					cx.Quit()
					break
				}
				click("keymap-jetbrains")
			case 34:
				if m.keymapProfile() != "jetbrains" {
					failure = errors.New("native JetBrains preset selection failed")
					cx.Quit()
					break
				}
				if capture("jetbrains-keymap-settings") {
					key('A', ui.ModifierControl|ui.ModifierShift)
				}
			case 35:
				if !m.palette || m.query != ">" {
					failure = errors.New("JetBrains Find Action failed")
					cx.Quit()
					break
				}
				key(27, 0)
			case 36:
				key('N', ui.ModifierControl|ui.ModifierShift)
			case 37:
				if !m.palette || m.query != "" {
					failure = errors.New("JetBrains Go to File failed")
					cx.Quit()
					break
				}
				key(27, 0)
			case 38:
				tap(16)
			case 39:
				tap(16)
			case 40:
				if !m.palette || !m.quick.everywhere {
					failure = errors.New("native JetBrains double Shift failed")
					cx.Quit()
					break
				}
				if capture("jetbrains-search-everywhere") {
					key(27, 0)
				}
			case 41:
				key('S', ui.ModifierControl|ui.ModifierAlt)
			case 42:
				if m.activity != "settings" || m.menu.name != "" {
					failure = errors.New("Ctrl+Alt+S was hijacked by menu mnemonic")
					cx.Quit()
					break
				}
				click("keymap-vscode")
			case 43:
				if m.keymapProfile() != "vscode" {
					failure = errors.New("native VS Code preset selection failed")
					cx.Quit()
					break
				}
				key('P', ui.ModifierControl|ui.ModifierShift)
			case 44:
				if !m.palette || m.query != ">" {
					failure = errors.New("restored VS Code Command Palette failed")
					cx.Quit()
					break
				}
				phase++
				cx.Quit()
			}
		}
		cx.Invalidate()
		return m.view(cx)
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	if phase != 45 {
		return fmt.Errorf("incomplete workbench acceptance phase=%d", phase)
	}
	// Preset selection and palette input are UI acknowledgements, not disk
	// acknowledgements. Drain the existing bounded writer after ui.Run ends,
	// then read the real file off the UI thread. A concurrent read here used to
	// accept the old preset or obstruct its Windows atomic replacement.
	if stopKeyboard != nil {
		stopKeyboard()
		stopKeyboard = nil
	}
	if err = verifyWindowsWorkbenchKeymapAfterDrain(keyboardWriterFinished, filepath.Join(root, "keyboard.json")); err != nil {
		return err
	}
	body, err := os.ReadFile(saved)
	if err != nil || string(body) != "// 原生 File 世界 😀" {
		return fmt.Errorf("actual Unicode saved file differs: %w", err)
	}
	body, err = os.ReadFile(readme)
	if err != nil || string(body) != "# Real Windows File menu\r\nUnicode 世界 😀\r\n" {
		return errors.New("Open/cancel changed original disk file")
	}
	state, err := readExtensionSettings(extRoot)
	if err != nil || !containsExtension(state.Uninstall, fixture.ID()) {
		return errors.New("native extension actions did not persist to disk")
	}
	if err = removePendingExtensions(extRoot, &state); err != nil {
		return err
	}
	remaining, err := extensions.List(extRoot)
	if err != nil || len(remaining) != 0 {
		return errors.New("deferred uninstall did not remove owned VSIX")
	}
	fmt.Println("Windows native File/shell/Unicode save/Quick Input/VSIX management and persistent VS Code/JetBrains keymaps including real double Shift passed")
	return nil
}

func verifyWindowsWorkbenchKeymapAfterDrain(finished <-chan struct{}, path string) error {
	select {
	case <-finished:
		return verifyWindowsWorkbenchKeymap(path)
	default:
		return errors.New("native keymap writer did not finish within the existing 3s shutdown limit")
	}
}

func verifyWindowsWorkbenchKeymap(path string) error {
	profile, readErr := readKeymap(path)
	file, openErr := os.Open(path)
	if openErr != nil {
		return fmt.Errorf("native keymap changes did not persist exact final preset: actual=%q read=%v file=%w", profile, readErr, openErr)
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		_ = file.Close()
		return fmt.Errorf("native keymap changes did not persist exact final preset: actual=%q read=%v invalid regular/bounded file stat=%v", profile, readErr, statErr)
	}
	body, rawErr := io.ReadAll(io.LimitReader(file, 4097))
	rawErr = errors.Join(rawErr, file.Close())
	if readErr != nil || profile != "vscode" || rawErr != nil || !bytes.Equal(body, []byte("{\"keymap\":\"vscode\"}\n")) {
		return fmt.Errorf("native keymap changes did not persist exact final preset: actual=%q read=%v bytes=%d raw=%v", profile, readErr, len(body), rawErr)
	}
	return nil
}
func chooseOwnedWindowsDialog(workspace, path string, cancel bool) error {
	return chooseOwnedWindowsDialogFor(workspace, path, cancel, uint32(os.Getpid()))
}
func chooseOwnedWindowsDialogFor(workspace, path string, cancel bool, expectedPID uint32) error {
	w, err := ownedWorkbenchWindow("gocode — "+filepath.Base(workspace), expectedPID)
	if err != nil {
		return err
	}
	user := windows.NewLazySystemDLL("user32.dll")
	var dialog uintptr
	var ownedDialogs []string
	callback := syscall.NewCallback(func(hwnd, param uintptr) uintptr {
		var pid uint32
		user.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		owner, _, _ := user.NewProc("GetWindow").Call(hwnd, 4)
		var class [32]uint16
		user.NewProc("GetClassNameW").Call(hwnd, uintptr(unsafe.Pointer(&class[0])), 32)
		visible, _, _ := user.NewProc("IsWindowVisible").Call(hwnd)
		enabled, _, _ := user.NewProc("IsWindowEnabled").Call(hwnd)
		if pid == expectedPID && windows.UTF16ToString(class[:]) == "#32770" {
			ownedDialogs = append(ownedDialogs, fmt.Sprintf("hwnd=%x owner=%x expected=%x visible=%d enabled=%d", hwnd, owner, uintptr(w), visible, enabled))
		}
		if pid == expectedPID && owner == uintptr(w) && windows.UTF16ToString(class[:]) == "#32770" && visible != 0 && enabled != 0 {
			dialog = hwnd
			return 0
		}
		return 1
	})
	deadline := time.Now().Add(10 * time.Second)
	for dialog == 0 && time.Now().Before(deadline) {
		ownedDialogs = nil
		user.NewProc("EnumWindows").Call(callback, 0)
		if dialog == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if dialog == 0 {
		return fmt.Errorf("owned shell file dialog did not appear: %v", ownedDialogs)
	}
	if cancel {
		user.NewProc("PostMessageW").Call(dialog, 0x10, 0, 0)
		return nil
	}
	var edit uintptr
	var folderPicker bool
	var controls []string
	child := syscall.NewCallback(func(hwnd, param uintptr) uintptr {
		var class [64]uint16
		user.NewProc("GetClassNameW").Call(hwnd, uintptr(unsafe.Pointer(&class[0])), 64)
		name := windows.UTF16ToString(class[:])
		id, _, _ := user.NewProc("GetDlgCtrlID").Call(hwnd)
		parent, _, _ := user.NewProc("GetParent").Call(hwnd)
		parentID, _, _ := user.NewProc("GetDlgCtrlID").Call(parent)
		var parentClass [64]uint16
		user.NewProc("GetClassNameW").Call(parent, uintptr(unsafe.Pointer(&parentClass[0])), 64)
		if name == "Edit" {
			visible, _, _ := user.NewProc("IsWindowVisible").Call(hwnd)
			enabled, _, _ := user.NewProc("IsWindowEnabled").Call(hwnd)
			controls = append(controls, fmt.Sprintf("Edit id=%d parent=%d/%s visible=%d enabled=%d dialog=%x", id, parentID, windows.UTF16ToString(parentClass[:]), visible, enabled, dialog))
			filename := parentID == 1148 || id == 1148 || id == 1001 && windows.UTF16ToString(parentClass[:]) == "ComboBox"
			folder := id == 1152 && parent == dialog && windows.UTF16ToString(parentClass[:]) == "#32770"
			if (filename || folder) && visible != 0 && enabled != 0 {
				edit = hwnd
				folderPicker = folder
			}
		}
		return 1
	})
	for edit == 0 && time.Now().Before(deadline) {
		controls = nil
		user.NewProc("EnumChildWindows").Call(dialog, child, 0)
		if edit == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if edit == 0 {
		return fmt.Errorf("owned shell filename edit missing: %v", controls)
	}
	time.Sleep(100 * time.Millisecond) // let the shell complete its initial filename update
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	ok, _, _ := user.NewProc("SendMessageW").Call(edit, 0x0c, 0, uintptr(unsafe.Pointer(name)))
	var entered [32768]uint16
	// GetWindowText does not read a foreign process's edit contents. WM_GETTEXT
	// is marshalled by USER32 and also verifies the real child-process picker.
	user.NewProc("SendMessageW").Call(edit, 0x0d, uintptr(len(entered)), uintptr(unsafe.Pointer(&entered[0])))
	if ok == 0 || windows.UTF16ToString(entered[:]) != path {
		return fmt.Errorf("shell filename entry rejected text: result=%d actual=%q controls=%v", ok, windows.UTF16ToString(entered[:]), controls)
	}
	accept, _, _ := user.NewProc("GetDlgItem").Call(dialog, 1)
	if accept == 0 {
		return errors.New("owned shell accept button missing")
	}
	user.NewProc("PostMessageW").Call(dialog, 0x111, 1, accept)
	if folderPicker {
		// The shell first navigates an absolute typed folder. Accept that
		// selected current folder after the edit shows its final component.
		for time.Now().Before(deadline) {
			alive, _, _ := user.NewProc("IsWindow").Call(dialog)
			if alive == 0 {
				return nil
			}
			entered = [32768]uint16{}
			user.NewProc("SendMessageW").Call(edit, 0x0d, uintptr(len(entered)), uintptr(unsafe.Pointer(&entered[0])))
			if windows.UTF16ToString(entered[:]) == filepath.Base(path) {
				user.NewProc("PostMessageW").Call(dialog, 0x111, 1, accept)
				return nil
			}
			time.Sleep(20 * time.Millisecond)
		}
		if pixels, err := winprobe.Window(dialog).Capture(); err == nil {
			os.MkdirAll(".cache/windows-workbench", 0700)
			if f, err := os.Create(".cache/windows-workbench/folder-dialog-failure.png"); err == nil {
				_ = png.Encode(f, pixels)
				_ = f.Close()
			}
		}
		controls = nil
		user.NewProc("EnumChildWindows").Call(dialog, child, 0)
		return fmt.Errorf("owned folder picker did not navigate/accept: text=%q controls=%v", windows.UTF16ToString(entered[:]), controls)
	}
	return nil
}
