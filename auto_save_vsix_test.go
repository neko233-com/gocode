package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	textbuffer "github.com/neko233-com/godesktop/editor"
	"github.com/neko233-com/godesktop/extensions"
)

type autoSaveVSIXSave struct {
	Version int
	Text    string
	Disk    string
	Dirty   bool
}

type autoSaveVSIXReport struct {
	Saved, Dirty bool
	Version      int
	Text         string
	Saves        []autoSaveVSIXSave
}

func TestAutoSaveRealVSIXReceivesSharedWriterEventsExactlyOnce(t *testing.T) {
	m, diskMailbox, now := autoSaveModel(t, "afterDelay")
	d := autoSaveOpen(t, m, "自动保存😀.go", "package main\r\n// original\r\n")
	archiveRoot, extensionRoot, runtimeRoot := t.TempDir(), t.TempDir(), t.TempDir()
	// The real Node bridge creates and removes its runtime in this owned root;
	// archive/store/workspace directories were created before changing temp vars.
	t.Setenv("TMP", runtimeRoot)
	t.Setenv("TEMP", runtimeRoot)
	t.Setenv("TMPDIR", runtimeRoot)
	manifest := `{"name":"autosave-proof","publisher":"fixture","version":"0.1.0","main":"extension.cjs","engines":{"vscode":"^1.140.0"},"activationEvents":["onStartupFinished"],"contributes":{"commands":[{"command":"autosave.inspect","title":"Inspect real saves"},{"command":"autosave.manual","title":"Save through native writer"}]}}`
	script := `const v=require('vscode'),fs=require('node:fs');exports.activate=c=>{
 const saves=[];
 c.subscriptions.push(v.workspace.onDidSaveTextDocument(d=>saves.push({version:d.version,text:d.getText(),dirty:d.isDirty,disk:fs.readFileSync(d.fileName,'utf8')})));
 const inspect=()=>{const d=v.window.activeTextEditor.document;return {saves:[...saves],version:d.version,text:d.getText(),dirty:d.isDirty};};
 c.subscriptions.push(v.commands.registerCommand('autosave.inspect',inspect));
 c.subscriptions.push(v.commands.registerCommand('autosave.manual',async()=>{const d=v.window.activeTextEditor.document,saved=await d.save();return {...inspect(),saved};}));
};`
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for name, source := range map[string]string{"extension/package.json": manifest, "extension/extension.cjs": script} {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write([]byte(source)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(archiveRoot, "autosave-proof.vsix")
	if err := os.WriteFile(archivePath, archive.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	installed, err := extensions.Install(extensionRoot, archivePath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	host, err := extensions.Start(ctx, m.workspace, []extensions.Extension{installed})
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	hostErrors, eventsDone := make(chan string, 8), make(chan struct{})
	go func() {
		defer close(eventsDone)
		for event := range host.Events {
			if event.Type == "error" {
				select {
				case hostErrors <- event.Text:
				default:
				}
			}
		}
	}()
	// This mailbox is the headless UI actor. RPC handlers and synchronization
	// barriers post here; only the test goroutine reads or changes native state.
	uiMailbox := make(chan func(), 8)
	dispatch := func(fn func()) bool {
		select {
		case uiMailbox <- fn:
			return true
		case <-ctx.Done():
			return false
		}
	}
	host.Register("workspace/saveDocument", func(requestContext context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			Path     string `json:"path"`
			Version  int    `json:"version"`
			Instance string `json:"instance"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		physical, err := canonicalRPCPath(requestContext, m.workspace, request.Path)
		if err != nil {
			return nil, err
		}
		type answer struct {
			value any
			err   error
		}
		reply := make(chan answer, 1)
		if !dispatch(func() {
			if err := requestContext.Err(); err != nil {
				reply <- answer{err: err}
				return
			}
			if m.closeBusy {
				reply <- answer{err: errors.New("workbench is closing; save was not queued")}
				return
			}
			target := m.findDocument(physical)
			if target == nil || target.buffer == nil || target.buffer.Version() != request.Version || request.Instance != "" && target.instance != request.Instance {
				reply <- answer{value: map[string]bool{"saved": false}}
				return
			}
			m.requestSave(requestContext, []*document{target}, func(err error) {
				if errors.Is(err, errSaveChanged) {
					reply <- answer{value: map[string]any{"saved": false, "document": stateOf(target)}}
				} else if err != nil {
					reply <- answer{err: err}
				} else {
					reply <- answer{value: map[string]any{"saved": true, "document": stateOf(target)}}
				}
			})
		}) {
			return nil, errors.New("workbench closed before save request")
		}
		select {
		case result := <-reply:
			return result.value, result.err
		case <-requestContext.Done():
			return nil, requestContext.Err()
		}
	})
	initial := stateOf(d)
	syncErrors := make(chan error, 8)
	q := startExtensionDocumentSync(ctx,
		func(request context.Context) error {
			return host.Call(request, "initialize", map[string]any{"documents": []*documentState{initial}, "active": initial}, nil)
		},
		func(request context.Context, params map[string]any) error {
			return host.Call(request, "syncDocument", params, nil)
		},
		func(err error) { syncErrors <- err })
	defer func() { m.onDocument = nil; cancel(); <-q.done }()
	var nativeSaveIDs []uint64
	var nativeSaved []autoSaveVSIXSave
	m.onDocument = func(kind string, target *document, change textbuffer.ChangeEvent) {
		if target != d {
			return
		}
		state := stateOf(target)
		if kind == "save" {
			nativeSaveIDs = append(nativeSaveIDs, state.SaveID)
			nativeSaved = append(nativeSaved, autoSaveVSIXSave{Version: state.Version, Text: *state.Text, Dirty: state.Dirty, Disk: autoSaveDisk(t, target.path)})
		}
		// All fields are immutable values captured from the actual UI event,
		// including the writer's saveId; no synthetic successful save is sent.
		if err := q.push(extensionSyncJob{params: map[string]any{"kind": kind, "document": state, "changes": change.Changes}}); err != nil {
			t.Fatal(err)
		}
	}
	awaitSync := func() {
		t.Helper()
		reply := make(chan error, 1)
		go func() { reply <- q.await(ctx, dispatch) }()
		saveAck(t, uiMailbox)()
		select {
		case err := <-reply:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		select {
		case err := <-syncErrors:
			t.Fatal("actual Node document synchronization failed", err)
		default:
		}
	}
	inspect := func() autoSaveVSIXReport {
		t.Helper()
		awaitSync()
		var result autoSaveVSIXReport
		if err := host.Call(ctx, "execute", map[string]string{"command": "autosave.inspect"}, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	awaitSync()
	m.replaceSelection(d, "first 😀\r\n")
	m.autoSaveChanged(d, now)
	first := d.buffer.Text()
	m.autoSaveTick(now.Add(time.Second))
	held := saveAck(t, diskMailbox)
	m.replaceSelection(d, "newer 世界\r\n")
	m.autoSaveChanged(d, now.Add(2*time.Second))
	second, secondVersion := d.buffer.Text(), d.buffer.Version()
	if report := inspect(); len(report.Saves) != 0 || !report.Dirty || report.Text != second || autoSaveDisk(t, d.path) != first {
		t.Fatal("held old write was reported as a successful current save", report)
	}
	held()
	if report := inspect(); len(report.Saves) != 0 || !report.Dirty || d.saveID != 0 {
		t.Fatal("old receipt emitted a save for newer dirty edits", report)
	}
	m.autoSaveTick(now.Add(3 * time.Second))
	saveAck(t, diskMailbox)()
	if report := inspect(); len(report.Saves) != 1 || report.Dirty || report.Text != second || report.Saves[0].Version != secondVersion || report.Saves[0].Text != second || report.Saves[0].Disk != second || report.Saves[0].Dirty || d.saveID != 1 {
		t.Fatal("actual VSIX missed the latest automatic writer event", report)
	}
	for range 4 {
		m.autoSaveTick(now.Add(time.Hour))
	}
	if report := inspect(); len(report.Saves) != 1 || d.saveID != 1 || m.saveBusy {
		t.Fatal("repeated timer duplicated the VSIX save event", report)
	}
	m.configureAutoSave(defaultAutoSaveConfig())
	m.replaceSelection(d, "manual while off 😀\r\n")
	manualText, manualVersion := d.buffer.Text(), d.buffer.Version()
	m.autoSaveTick(now.Add(2 * time.Hour))
	if report := inspect(); len(report.Saves) != 1 || !report.Dirty || autoSaveDisk(t, d.path) != second || m.saveBusy {
		t.Fatal("off policy wrote a new dirty version", report)
	}
	type manualAnswer struct {
		result autoSaveVSIXReport
		err    error
	}
	manualCall := func() chan manualAnswer {
		reply := make(chan manualAnswer, 1)
		go func() {
			var result autoSaveVSIXReport
			err := host.Call(ctx, "execute", map[string]string{"command": "autosave.manual"}, &result)
			reply <- manualAnswer{result, err}
		}()
		return reply
	}
	finishManual := func(reply chan manualAnswer) autoSaveVSIXReport {
		t.Helper()
		select {
		case answer := <-reply:
			if answer.err != nil {
				t.Fatal(answer.err)
			}
			return answer.result
		case <-ctx.Done():
			t.Fatal(ctx.Err())
			return autoSaveVSIXReport{}
		}
	}
	manualReply := manualCall()
	saveAck(t, uiMailbox)()
	saveAck(t, diskMailbox)()
	if result := finishManual(manualReply); !result.Saved || result.Dirty || result.Text != manualText {
		t.Fatal("real VSIX Document.save did not use the shared writer", result)
	}
	if report := inspect(); len(report.Saves) != 2 || report.Saves[1].Version != manualVersion || report.Saves[1].Text != manualText || report.Saves[1].Disk != manualText || report.Saves[1].Dirty || d.saveID != 2 {
		t.Fatal("manual RPC acknowledgement and native event were not deduplicated", report)
	}
	// A real manual request is held before UI validation, then a newer editor
	// change invalidates its exact requested version. It must never hit disk.
	m.replaceSelection(d, "obsolete manual request\r\n")
	awaitSync()
	staleReply := manualCall()
	staleRequest := saveAck(t, uiMailbox)
	m.replaceSelection(d, "final version 世界😀\r\n")
	finalText, finalVersion := d.buffer.Text(), d.buffer.Version()
	staleRequest()
	if result := finishManual(staleReply); result.Saved {
		t.Fatal("stale VSIX save was accepted", result)
	}
	if report := inspect(); len(report.Saves) != 2 || !report.Dirty || report.Text != finalText || m.saveBusy || autoSaveDisk(t, d.path) != manualText {
		t.Fatal("stale manual save emitted an event or overwrote disk", report)
	}
	m.configureAutoSave(autoSaveConfig{Mode: "afterDelay", DelayMS: 1000})
	m.autoSaveChanged(d, now.Add(6*time.Second))
	m.autoSaveTick(now.Add(7 * time.Second))
	saveAck(t, diskMailbox)()
	report := inspect()
	if len(report.Saves) != 3 || report.Dirty || report.Saves[2].Version != finalVersion || report.Saves[2].Text != finalText || report.Saves[2].Disk != finalText || report.Saves[2].Dirty || d.saveID != 3 {
		t.Fatal("re-enabled policy did not save the latest native revision", report)
	}
	if len(nativeSaveIDs) != 3 || len(nativeSaved) != 3 {
		t.Fatal("native receipt count differs from real VSIX events", nativeSaveIDs)
	}
	for i, id := range nativeSaveIDs {
		if id != uint64(i+1) || nativeSaved[i] != report.Saves[i] || i > 0 && nativeSaved[i].Version <= nativeSaved[i-1].Version {
			t.Fatal("native saveId/VSIX text and version order differs", nativeSaveIDs, nativeSaved, report.Saves)
		}
	}
	select {
	case err := <-hostErrors:
		t.Fatal("real VSIX reported a runtime error", err)
	default:
	}
	if host.DroppedEvents() != 0 {
		t.Fatal("actual extension notifications were dropped", host.DroppedEvents())
	}
	m.onDocument = nil
	cancel()
	<-q.done
	if err := host.Close(); err != nil {
		t.Fatal(err)
	}
	<-eventsDone
	entries, err := os.ReadDir(runtimeRoot)
	if err != nil || len(entries) != 0 {
		t.Fatal("real Node runtime was not cleaned up", entries, err)
	}
	leftovers, err := filepath.Glob(filepath.Join(m.workspace, ".gocode-save-*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatal("shared writer left temporary files", leftovers, err)
	}
}
