package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/neko233-com/gocode/internal/terminal"
	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/extensions"
)

const terminalVSIXFixture = `const v=require('vscode');let held,natural,failed,user,userClosed,base,counts={open:0,close:0,active:0,state:0};
const check=(ok,text)=>{if(!ok)throw Error(text);};
const program='('+(()=>{
 const decoder=new (require('node:string_decoder').StringDecoder)('utf8');let input='';
 if(process.stdin.isTTY)process.stdin.setRawMode(true);
 if(process.env.GOCODE_TERMINAL_SENTINEL!==undefined||process.env.API_REMOVE!==undefined)process.exit(41);
 if(require('node:fs').realpathSync(process.cwd())!==require('node:fs').realpathSync(process.env.API_EXPECTED_CWD))process.exit(44);
 process.stdout.write('\x1b[38;2;220;220;170mAPI_COMMAND\x1b[0m\r\n');
 process.stdin.on('data',chunk=>{input+=decoder.write(chunk);if(input.length>4096)process.exit(42);for(;;){const i=input.search(/[\r\n]/);if(i<0)break;const line=input.slice(0,i);input=input.slice(i+1).replace(/^[\r\n]+/,'');if(line==='EXIT_7')process.exit(7);process.stdout.write('\x1b[38;2;229;192;123mAPI_ENV:'+process.env.API_VALUE+' 😀\x1b[0m\r\n\x1b[38;2;206;145;120mAPI_INPUT:'+line+'\x1b[0m\r\n');}});
}).toString()+')();';
exports.activate=c=>{
 for(const [key,event]of Object.entries({open:v.window.onDidOpenTerminal,close:v.window.onDidCloseTerminal,active:v.window.onDidChangeActiveTerminal,state:v.window.onDidChangeTerminalState}))c.subscriptions.push(event(()=>counts[key]++));
 const command=(name,fn)=>c.subscriptions.push(v.commands.registerCommand('fixture.terminals.'+name,fn));
 const options=name=>{const cwd=v.Uri.joinPath(v.workspace.workspaceFolders[0].uri,'run');const env={API_VALUE:'env 世界',API_REMOVE:null,API_EXPECTED_CWD:cwd.fsPath};if(process.platform==='win32')for(const key of ['SystemRoot','USERPROFILE','TEMP','TMP'])env[key]=process.env[key];return {name,shellPath:process.execPath,shellArgs:['-e',program],cwd,strictEnv:true,env,hideFromUser:true,message:'\x1b[38;2;220;220;170mEXTENSION_START\x1b[0m',isTransient:true};};
 command('setup',async()=>{
  base=v.window.terminals.length;const given=options('API terminal 😀');held=v.window.createTerminal(given);given.env.API_VALUE='wrong';
  const pid=await held.processId;check(Number.isInteger(pid)&&pid>0,'actual native PID missing');check(held.processId===held.processId,'unstable PID promise');
  check(v.window.terminals.includes(held)&&held.creationOptions.env.API_VALUE==='env 世界','identity or frozen creation options');
  held.sendText('first 世界',false);held.sendText(' 😀');return pid;
 });
 command('show',()=>{held.show(true);return true;});
 command('hide',()=>{held.hide();return true;});
 command('focus',()=>{held.show();return true;});
 command('check',()=>{check(v.window.activeTerminal===held&&held.state.isInteractedWith&&counts.open===1&&counts.state===1,'native active/state/open events');check(v.window.terminals.length===base+1,'terminal collection');return true;});
 command('dispose',async()=>{
  const closed=new Promise(resolve=>{const d=v.window.onDidCloseTerminal(t=>{if(t===held){d.dispose();resolve();}});});held.dispose();held.dispose();await closed;
  check(held.exitStatus.reason===v.TerminalExitReason.Extension&&!v.window.terminals.includes(held)&&counts.close===1,'dispose identity/reason/deduplication');let rejected=false;try{held.sendText('late');}catch{rejected=true;}check(rejected,'disposed terminal accepted input');return true;
 });
 command('natural',async()=>{natural=v.window.createTerminal(options('Natural exit'));const pid=await natural.processId;check(natural!==held&&pid>0,'new terminal reused disposed identity');return pid;});
 command('exit',async()=>{const closed=new Promise(resolve=>{const d=v.window.onDidCloseTerminal(t=>{if(t===natural){d.dispose();resolve();}});});natural.sendText('EXIT_7');await closed;check(natural.exitStatus.code===7&&natural.exitStatus.reason===v.TerminalExitReason.Process&&counts.close===2,'actual exit status');return true;});
 command('failure',async()=>{
  const closed=new Promise(resolve=>{const d=v.window.onDidCloseTerminal(t=>{if(t===failed){d.dispose();resolve();}});});failed=v.window.createTerminal({name:'Expected failure',shellPath:process.execPath+'.missing'});
  check(await failed.processId===undefined,'failed startup fabricated PID');await closed;check(failed.exitStatus.reason===v.TerminalExitReason.Unknown&&counts.close===3,'failed startup lifecycle');return true;
 });
 command('user',async()=>{user=v.window.createTerminal(options('Native close'));userClosed=new Promise(resolve=>{const d=v.window.onDidCloseTerminal(t=>{if(t===user){d.dispose();resolve();}});});const pid=await user.processId;check(pid>0,'user-close process missing');user.show();return pid;});
 command('userclosed',async()=>{await userClosed;check(user.exitStatus.reason===v.TerminalExitReason.User&&!v.window.terminals.includes(user)&&counts.close===4,'native user close reason/event');return true;});
 command('final',()=>{check(v.window.terminals.length===base&&counts.open>=4&&counts.close===4,'native collection or lifecycle leaked');return true;});
};`

func installTerminalFixture(root string) (extensions.Extension, error) {
	file, err := os.Create(filepath.Join(root, "terminal.vsix"))
	if err != nil {
		return extensions.Extension{}, err
	}
	writer := zip.NewWriter(file)
	manifest := `{"publisher":"gocode","name":"terminal-fixture","version":"0.1.0","engines":{"vscode":"^1.140.0"},"main":"./extension.cjs","activationEvents":["onCommand:fixture.terminals.setup"]}`
	for _, entry := range []struct{ name, text string }{{"extension/package.json", manifest}, {"extension/extension.cjs", terminalVSIXFixture}} {
		part, err := writer.Create(entry.name)
		if err == nil {
			_, err = part.Write([]byte(entry.text))
		}
		if err != nil {
			writer.Close()
			file.Close()
			return extensions.Extension{}, err
		}
	}
	if err := errors.Join(writer.Close(), file.Close()); err != nil {
		return extensions.Extension{}, err
	}
	return extensions.Install(filepath.Join(root, "extensions"), file.Name())
}

func runTerminalVSIXAcceptance() error {
	for _, key := range []string{"GOCODE_TERMINAL_SENTINEL", "API_REMOVE"} {
		old, exists := os.LookupEnv(key)
		if err := os.Setenv(key, "owned fixture inheritance marker"); err != nil {
			return err
		}
		defer func() {
			if exists {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		}()
	}
	if err := os.Setenv("GODESKTOP_READBACK", "1"); err != nil {
		return err
	}
	root, err := os.MkdirTemp("", "gocode-terminal-vsix-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	if err = os.Mkdir(filepath.Join(root, "run"), 0700); err != nil {
		return err
	}
	source := []byte("package main\r\n// VSIX terminal source stays unchanged 😀\r\nfunc main() {}\r\n")
	file := filepath.Join(root, "main.go")
	if err = os.WriteFile(file, source, 0600); err != nil {
		return err
	}
	extension, err := installTerminalFixture(root)
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
	d, err := loadDocument(ctx, file)
	if err != nil {
		return err
	}
	m.docs = []*document{d}
	m.active = 0
	m.files = []string{"main.go"}
	m.editing = true
	m.logo, err = loadWorkbenchLogo()
	if err != nil {
		return err
	}
	var stopTerminals, stopIcon func()
	defer func() {
		cancel()
		if stopTerminals != nil {
			stopTerminals()
		}
		if stopIcon != nil {
			stopIcon()
		}
	}()
	var initialized <-chan struct{}
	var failure error
	phase, waiting := 0, false
	var pid int
	var owned *terminalTab
	var session *terminal.Session
	var diagnostic atomic.Value
	diagnostic.Store("starting")
	watchdog := time.AfterFunc(60*time.Second, func() { fmt.Fprintln(os.Stderr, "native terminal VSIX timeout:", diagnostic.Load()); os.Exit(2) })
	defer watchdog.Stop()
	var ticking bool
	logged := false
	err = ui.Run(ui.WindowOptions{Title: "gocode — " + filepath.Base(root), Width: 1280, Height: 820, CustomTitlebar: true, Background: ui.RGB(editor), Input: m.input}, func(cx *ui.Context) *ui.Element {
		m.native = cx
		if initialized == nil {
			stopTerminals = m.startTerminals(ctx, cx)
			stopIcon = applyAppIcon("gocode — " + filepath.Base(root))
			m.newTerminal()
			m.hidePanel()
			initialized = m.startExtensions(ctx, cx, host, filepath.Join(root, "state"))
		}
		fail := func(err error) { failure = err; cx.Quit() }
		execute := func(name string) {
			waiting = true
			go func() {
				request, done := context.WithTimeout(ctx, 10*time.Second)
				defer done()
				var value json.RawMessage
				err := m.awaitExtensions(request)
				if err == nil {
					err = host.Call(request, "execute", map[string]string{"command": "fixture.terminals." + name}, &value)
				}
				cx.Dispatch(func() {
					if err != nil {
						fail(err)
						return
					}
					if name == "setup" || name == "natural" || name == "user" {
						if err := json.Unmarshal(value, &pid); err != nil {
							fail(err)
							return
						}
					}
					waiting = false
					phase++
				})
			}()
		}
		diagnostic.Store(fmt.Sprintf("phase=%d waiting=%t terminals=%d pid=%d focused=%t panel=%t message=%s", phase, waiting, len(m.terminals), pid, m.terminalFocused, m.showPanel, m.message))
		if !waiting && failure == nil && cx.RenderedFrames() >= 8 {
			switch phase {
			case 0:
				select {
				case <-initialized:
					execute("setup")
				default:
				}
			case 1:
				for _, tab := range m.terminals {
					if tab.session != nil && tab.session.PID() == pid {
						owned = tab
						session = tab.session
					}
				}
				if owned == nil || owned.frame == nil {
					break
				}
				if !logged && owned.frame.Generation > 8 {
					logged = true
					fmt.Printf("Real VSIX terminal frame: %+v\n%s\n", owned.frame.Size, owned.frame.Text())
				}
				diagnostic.Store(fmt.Sprintf("phase=1 pid=%d frame=%d interacted=%t\n%s", pid, owned.frame.Generation, owned.interacted, owned.frame.Text()))
				if owned.frame.Exited {
					fail(fmt.Errorf("custom shell exited before real input: code=%d\n%s", owned.frame.ExitCode, owned.frame.Text()))
					break
				}
				if terminalHasColor(owned.frame, "API_INPUT:first 世界 😀", 0xce9178) && terminalHasColor(owned.frame, "API_ENV:env 世界 😀", 0xe5c07b) {
					if !m.editing || m.showPanel || m.terminalFocused || !owned.hidden {
						fail(errors.New("hidden terminal stole native editor focus"))
						break
					}
					execute("show")
				}
			case 2:
				if m.currentTerminal() != owned || !m.showPanel || owned.hidden {
					break
				}
				if !m.editing || m.terminalFocused {
					fail(errors.New("preserveFocus changed actual editor input"))
					break
				}
				bounds, ok := cx.ElementBounds("terminal-grid")
				if !ok {
					break
				}
				directory := os.Getenv("GOCODE_TERMINAL_VSIX_SCREENSHOTS")
				if directory == "" {
					directory = filepath.Join(".cache", "terminal-vsix")
				}
				if err := captureTerminalAcceptance(cx, root, filepath.Join(directory, "output-vsix.png"), bounds); err != nil {
					if !errors.Is(err, errTerminalPixelsPending) {
						fail(err)
					}
					break
				}
				execute("check")
			case 3:
				execute("hide")
			case 4:
				if !m.showPanel && m.editing && !m.terminalFocused {
					execute("focus")
				}
			case 5:
				if m.showPanel && m.terminalFocused && !m.editing && m.currentTerminal() == owned {
					execute("dispose")
				}
			case 6:
				if m.extensionTerminal(owned.wireID) != nil {
					fail(errors.New("disposed terminal remains native"))
					break
				}
				select {
				case <-session.Done():
					owned = nil
					execute("natural")
				default:
				}
			case 7:
				for _, tab := range m.terminals {
					if tab.session != nil && tab.session.PID() == pid {
						owned = tab
						session = tab.session
					}
				}
				if owned != nil && owned.frame != nil && terminalHasColor(owned.frame, "API_COMMAND", 0xdcdcaa) {
					execute("exit")
				}
			case 8:
				select {
				case <-session.Done():
					execute("failure")
				default:
				}
			case 9:
				owned = nil
				execute("user")
			case 10:
				for _, tab := range m.terminals {
					if tab.session != nil && tab.session.PID() == pid {
						owned = tab
						session = tab.session
					}
				}
				if owned == nil || m.currentTerminal() != owned || !m.showPanel || !m.terminalFocused {
					break
				}
				bounds, ok := cx.ElementBounds("terminal-kill")
				if !ok {
					break
				}
				waiting = true
				x, y := bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2
				tabsNativePointer(cx, root, x, y, true, func(err error) {
					if err != nil {
						fail(err)
						return
					}
					tabsNativePointer(cx, root, x, y, false, func(err error) {
						if err != nil {
							fail(err)
							return
						}
						waiting = false
						phase++
						cx.Invalidate()
					})
				})
			case 11:
				execute("userclosed")
			case 12:
				select {
				case <-session.Done():
					execute("final")
				default:
				}
			case 13:
				if len(m.terminals) != 1 || d.buffer.Dirty() || d.buffer.Version() != 1 {
					fail(errors.New("VSIX terminal changed editor or leaked native tabs"))
					break
				}
				cx.Quit()
			}
		}
		if !ticking {
			ticking = true
			time.AfterFunc(30*time.Millisecond, func() { cx.Dispatch(func() { ticking = false }) })
		}
		return m.view(cx)
	})
	if err != nil {
		return err
	}
	if failure != nil {
		return failure
	}
	actual, err := os.ReadFile(file)
	if err != nil || string(actual) != string(source) {
		return errors.New("VSIX terminal changed source bytes")
	}
	fmt.Println("Native terminal VSIX passed: real PID/cwd/env/ordered Unicode input, hidden/preserveFocus/show/hide pixels, stable objects/events, actual exit 7, extension disposal/native user close with process cleanup and failed-start PID rejection; source unchanged")
	return nil
}
