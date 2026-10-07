package main

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/extensions"
)

const editorGroupsVSIX = `const v=require('vscode');let left,right,hidden;const events={visible:0,column:0,selection:0,ranges:0,close:0,save:0};
exports.activate=c=>{
 for(const [name,event] of Object.entries({visible:v.window.onDidChangeVisibleTextEditors,column:v.window.onDidChangeTextEditorViewColumn,selection:v.window.onDidChangeTextEditorSelection,ranges:v.window.onDidChangeTextEditorVisibleRanges,close:v.workspace.onDidCloseTextDocument,save:v.workspace.onDidSaveTextDocument}))c.subscriptions.push(event(()=>events[name]++));
 const command=(name,fn)=>c.subscriptions.push(v.commands.registerCommand('fixture.editors.'+name,fn));
 const check=(value,message)=>{if(!value)throw Error(message);};
 command('setup',async()=>{
  left=v.window.activeTextEditor;check(left&&v.window.visibleTextEditors.length===1,'initial native editor');
  hidden=await v.workspace.openTextDocument(v.Uri.joinPath(v.workspace.workspaceFolders[0].uri,'other.go'));
  check(v.window.visibleTextEditors.length===1&&v.window.activeTextEditor===left,'hidden document took focus');
  const character=left.document.lineAt(40).text.indexOf('😀');
  right=await v.window.showTextDocument(left.document,{viewColumn:v.ViewColumn.Beside,preserveFocus:true,selection:new v.Range(40,character,40,character+2)});
  check(right!==left&&right.document===left.document&&right.viewColumn===2&&left.viewColumn===1,'native shared views/columns');
  check(v.window.visibleTextEditors.length===2&&v.window.activeTextEditor===left&&right.selection.active.character===character+2,'preserved focus/UTF-16 selection');return 'setup';
 });
 command('mutate',async()=>{
  right.selection=new v.Selection(0,0,0,0);
  check(await right.edit(e=>e.insert(new v.Position(0,0),'// VSIX group 😀界\r\n')),'inactive native edit failed');
  check(left.document===right.document&&left.document.getText().startsWith('// VSIX group 😀界\r\npackage main\r\n'),'shared CRLF snapshot');
  right.revealRange(new v.Range(100,0,100,0),v.TextEditorRevealType.AtTop);
  check(await right.edit(()=>{}),'native operation barrier');
  check(right.visibleRanges[0].start.line===100&&v.window.activeTextEditor===left,'independent reveal stole focus');return 'mutate';
 });
 command('survivor',async()=>{
  check(v.window.visibleTextEditors.length===1&&v.window.visibleTextEditors[0]===right&&right.viewColumn===1&&v.window.activeTextEditor===right,'surviving native view identity');
  check(!(await left.edit(e=>e.insert(new v.Position(0,0),'STALE'))),'disposed view edited source');
  check(!right.document.isClosed&&!hidden.isClosed&&events.column>=1&&events.visible>=2&&events.selection>=1&&events.ranges>=1,'actual native event lifetimes');return 'survivor';
 });
 command('save',async()=>{check(await right.document.save(),'native save acknowledgement');check(!right.document.isDirty&&events.save===1,'save notification deduplication');return 'save';});
 command('final',async()=>{check(v.window.activeTextEditor===undefined&&v.window.visibleTextEditors.length===0&&right.document.isClosed&&!hidden.isClosed&&v.workspace.textDocuments.length===1,'last-view close/hidden resource');check(!(await right.edit(e=>e.insert(new v.Position(0,0),'STALE'))),'closed editor revived');return 'final';});
};`

func installEditorGroupsFixture(root string) (extensions.Extension, error) {
	archive := filepath.Join(root, "editor-groups.vsix")
	file, err := os.Create(archive)
	if err != nil {
		return extensions.Extension{}, err
	}
	writer := zip.NewWriter(file)
	manifest := `{"publisher":"gocode","name":"editor-groups-fixture","version":"0.1.0","engines":{"vscode":"^1.140.0"},"main":"./extension.cjs","activationEvents":["onCommand:fixture.editors.setup"]}`
	for _, entry := range []struct{ name, text string }{{"extension/package.json", manifest}, {"extension/extension.cjs", editorGroupsVSIX}} {
		part, err := writer.Create(entry.name)
		if err != nil {
			writer.Close()
			file.Close()
			return extensions.Extension{}, err
		}
		if _, err = part.Write([]byte(entry.text)); err != nil {
			writer.Close()
			file.Close()
			return extensions.Extension{}, err
		}
	}
	if err := errors.Join(writer.Close(), file.Close()); err != nil {
		return extensions.Extension{}, err
	}
	return extensions.Install(filepath.Join(root, "extensions"), archive)
}

func runEditorGroupsVSIXAcceptance() error {
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "gocode-groups-vsix-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	var source strings.Builder
	source.WriteString("package main\r\n")
	for index := range 140 {
		fmt.Fprintf(&source, "var item%d = \"shared 😀界\"\r\n", index)
	}
	for _, name := range []string{"main.go", "other.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(source.String()), 0600); err != nil {
			return err
		}
	}
	extension, err := installEditorGroupsFixture(root)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	host, err := extensions.Start(ctx, root, []extensions.Extension{extension})
	if err != nil {
		return err
	}
	defer host.Close()
	go func() {
		for range host.Events {
		}
	}()
	m, err := newWorkspaceModel(root)
	if err != nil {
		return err
	}
	defer m.closeDocuments()
	d, err := loadDocument(ctx, filepath.Join(root, "main.go"))
	if err != nil {
		return err
	}
	m.docs = []*document{d}
	m.active = 0
	m.editing = true
	m.showPanel = false
	m.files = []string{"main.go", "other.go"}
	m.logo, err = loadWorkbenchLogo()
	if err != nil {
		return err
	}
	var stopOpen, stopSave, stopIcon func()
	defer func() {
		cancel()
		for _, stop := range []func(){stopOpen, stopSave, stopIcon} {
			if stop != nil {
				stop()
			}
		}
	}()
	var initialized <-chan struct{}
	var diagnostic atomic.Value
	diagnostic.Store("starting")
	watchdog := time.AfterFunc(60*time.Second, func() { fmt.Fprintln(os.Stderr, "native editor VSIX timed out:", diagnostic.Load()); os.Exit(2) })
	defer watchdog.Stop()
	phase, waiting, nextFrame := 0, false, uint64(8)
	var failure error
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(root), Width: 1280, Height: 820, CustomTitlebar: true, Background: ui.RGB(editor), Input: m.input}, func(cx *ui.Context) *ui.Element {
		m.native = cx
		if initialized == nil {
			stopOpen = m.startFileOpens(ctx, cx.Dispatch, nil)
			stopSave = m.startDocumentSaves(ctx, cx)
			stopIcon = applyAppIcon("gocode — " + filepath.Base(root))
			initialized = m.startExtensions(ctx, cx, host, filepath.Join(root, "state"))
		}
		fail := func(err error) { failure = err; cx.Quit() }
		advance := func(err error) {
			if err != nil {
				fail(err)
				return
			}
			waiting = false
			phase++
			nextFrame = cx.RenderedFrames() + 8
			cx.Invalidate()
		}
		execute := func(name string) {
			waiting = true
			go func() {
				request, stop := context.WithTimeout(ctx, 10*time.Second)
				defer stop()
				var result string
				err := m.awaitExtensions(request)
				if err == nil {
					err = host.Call(request, "execute", map[string]string{"command": "fixture.editors." + name}, &result)
				}
				if err == nil && result != name {
					err = fmt.Errorf("VSIX acknowledgement differs: %q", result)
				}
				cx.Dispatch(func() { advance(err) })
			}()
		}
		click := func(key string) {
			b, ok := cx.ElementBounds(key)
			if !ok {
				fail(fmt.Errorf("VSIX native control missing: %s", key))
				return
			}
			waiting = true
			x, y := b.X+b.Width/2, b.Y+b.Height/2
			tabsNativePointer(cx, root, x, y, true, func(err error) {
				if err != nil {
					advance(err)
					return
				}
				tabsNativePointer(cx, root, x, y, false, advance)
			})
		}
		diagnostic.Store(fmt.Sprintf("phase=%d groups=%d active=%d docs=%d", phase, len(m.allGroups()), m.groups.active, len(m.docs)))
		if !waiting && failure == nil && cx.RenderedFrames() >= nextFrame {
			select {
			case <-initialized:
				switch phase {
				case 0:
					execute("setup")
				case 1:
					if len(m.allGroups()) != 2 || m.groups.active != 1 || m.allGroups()[1].current != d || len(m.docs) != 2 {
						fail(errors.New("native VSIX setup differs from actual groups"))
						break
					}
					if err := captureGroupsPixels(cx, m, "vsix-setup"); err != nil {
						fail(err)
						break
					}
					execute("mutate")
				case 2:
					if m.allGroups()[1].views[d].scroll != 100 || !d.dirty() || m.groups.active != 1 {
						fail(errors.New("native VSIX edit/reveal model differs"))
						break
					}
					if err := captureGroupsPixels(cx, m, "vsix-mutated"); err != nil {
						fail(err)
						break
					}
					click("group-2-code-line-100")
				case 3:
					if m.groups.active != 2 {
						fail(errors.New("actual native input did not focus VSIX target"))
						break
					}
					click("close-group")
				case 4:
					execute("survivor")
				case 5:
					if err := captureGroupsPixels(cx, m, "vsix-survivor"); err != nil {
						fail(err)
						break
					}
					execute("save")
				case 6:
					if d.dirty() {
						fail(errors.New("native VSIX Save left acknowledged source dirty"))
						break
					}
					click("group-2-close-group")
				case 7:
					execute("final")
				case 8:
					phase = 9
					cx.Quit()
				}
			default:
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
	if phase != 9 {
		return fmt.Errorf("native VSIX editor groups incomplete: %v", diagnostic.Load())
	}
	mainBytes, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(mainBytes) != "// VSIX group 😀界\r\n"+source.String() {
		return errors.New("native VSIX save/source differs")
	}
	otherBytes, err := os.ReadFile(filepath.Join(root, "other.go"))
	if err != nil || string(otherBytes) != source.String() {
		return errors.New("native VSIX wrote hidden unrelated source")
	}
	fmt.Println("Native editor VSIX passed: real hidden opens, shared view objects, preserveFocus/UTF-16 selection, ordered inactive edit/reveal, visible/column/range/selection events, native focus/close, disposed identity, acknowledged CRLF save and untouched hidden file")
	return nil
}
