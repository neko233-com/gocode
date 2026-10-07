//go:build windows && cgo

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

// The normal application child has its own model and Context. A test-only
// build overlay adds a read-only observer, leaving production main.go intact.
// Every control still receives actual owned USER32 pointer input.
func nativeWorkbenchBoundsOverlay(t *testing.T, root string) string {
	t.Helper()
	workspace, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(workspace, "main.go")
	probePath := filepath.Join(workspace, "native_workbench_bounds_probe_injected.go")
	if _, err := os.Lstat(probePath); !os.IsNotExist(err) {
		t.Fatal("native probe overlay would mask an existing source file", err)
	}
	data, err := os.ReadFile(mainPath)
	if err != nil || len(data) > 256<<10 {
		t.Fatal("read bounded production main", err)
	}
	source := strings.ReplaceAll(string(data), "\r\n", "\n")
	const viewNeedle = "\t\treturn m.view(viewContext)\n"
	const runNeedle = "\terr = ui.Run(ui.WindowOptions{"
	if strings.Count(source, viewNeedle) != 1 || strings.Count(source, runNeedle) != 1 {
		t.Fatal("unknown production view template; cannot attach read-only native bounds observer")
	}
	source = strings.Replace(source, viewNeedle, "\t\treturn nativeWorkbenchObserveBounds(hostCtx, viewContext, m, m.view(viewContext))\n", 1)
	source = strings.Replace(source, runNeedle, "\tdefer nativeWorkbenchStopBounds()\n"+runNeedle, 1)
	backingMain := filepath.Join(root, "main-overlay.go")
	backingProbe := filepath.Join(root, "bounds-overlay.go")
	if err = os.WriteFile(backingMain, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(backingProbe, []byte(nativeWorkbenchBoundsObserverSource), 0600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(struct {
		Replace map[string]string
	}{map[string]string{mainPath: backingMain, probePath: backingProbe}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "overlay.json")
	if err = os.WriteFile(path, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNativeWorkbenchBoundsOverlayCompilesWithoutMutatingSource(t *testing.T) {
	before, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	overlay := nativeWorkbenchBoundsOverlay(t, root)
	exe := filepath.Join(root, "observed-workbench.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-overlay="+overlay, "-race", "-o", exe, ".")
	cmd.WaitDelay = 3 * time.Second
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile read-only geometry overlay: %v\n%s", err, output)
	}
	if err = winprobe.ValidateAMD64PE(exe); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile("main.go")
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("test geometry overlay changed production source", err)
	}
	if _, err = os.Lstat("native_workbench_bounds_probe_injected.go"); !os.IsNotExist(err) {
		t.Fatal("test geometry observer leaked a repository source file", err)
	}
}

type nativeWorkbenchBoundsResponse struct {
	Sequence       uint64               `json:"sequence"`
	PID            int                  `json:"pid"`
	Completed      uint64               `json:"completed"`
	ViewGeneration uint64               `json:"view_generation"`
	TreeTab        string               `json:"tree_tab"`
	Bounds         map[string]ui.Bounds `json:"bounds"`
}

type nativeWorkbenchBoundsClient struct {
	root     string
	pid      int
	sequence uint64
}

func (c *nativeWorkbenchBoundsClient) lookup(t *testing.T, tab, key string) ui.Bounds {
	t.Helper()
	c.sequence++
	request, err := json.Marshal(struct {
		Sequence uint64   `json:"sequence"`
		Tab      string   `json:"tab"`
		Keys     []string `json:"keys"`
	}{c.sequence, tab, []string{key}})
	if err != nil || c.sequence > 32 {
		t.Fatal("bounded native geometry request", err)
	}
	path := filepath.Join(c.root, "request.json")
	if err = os.WriteFile(path+".pending", request, 0600); err == nil {
		err = os.Rename(path+".pending", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	var result nativeWorkbenchBoundsResponse
	until(t, func() bool {
		data, e := os.ReadFile(filepath.Join(c.root, "response.json"))
		if e != nil || len(data) > 4096 || json.Unmarshal(data, &result) != nil {
			return false
		}
		b := result.Bounds[key]
		return result.Sequence == c.sequence && result.PID == c.pid && result.TreeTab == tab && result.Completed > 0 && result.ViewGeneration > 0 && b.Width > 0 && b.Height > 0
	})
	t.Logf("actual native geometry %s: generation=%d completed=%d bounds=%+v", key, result.ViewGeneration, result.Completed, result.Bounds[key])
	return result.Bounds[key]
}

// This source exists only in the test's private overlay directory. One worker
// reads/writes bounded local files; its UI mailbox callback only copies geometry.
const nativeWorkbenchBoundsObserverSource = `package main

import (
 "context"
 "encoding/json"
 "io"
 "os"
 "path/filepath"
 "sync"
 "time"
 "github.com/neko233-com/gocode/internal/uidispatch"
 ui "github.com/neko233-com/godesktop"
)

var nativeWorkbenchProbe struct {
 once sync.Once
 mu sync.Mutex
 cancel context.CancelFunc
 done chan struct{}
 generation uint64 // UI-owned; paired with the last actual View's tab.
 tab string
}

func nativeWorkbenchObserveBounds(parent context.Context, cx *ui.Context, m *model, tree *ui.Element) *ui.Element {
 nativeWorkbenchProbe.generation++
 nativeWorkbenchProbe.tab = m.extensionsView.tab
 nativeWorkbenchProbe.once.Do(func() {
  root := os.Getenv("GOCODE_TEST_WORKBENCH_BOUNDS")
  if !filepath.IsAbs(root) { return }
  ctx, cancel := context.WithCancel(parent)
  done := make(chan struct{})
  nativeWorkbenchProbe.mu.Lock()
  nativeWorkbenchProbe.cancel, nativeWorkbenchProbe.done = cancel, done
  nativeWorkbenchProbe.mu.Unlock()
  go nativeWorkbenchBoundsWorker(ctx, cx, root, done)
 })
 return tree
}

func nativeWorkbenchStopBounds() {
 nativeWorkbenchProbe.mu.Lock()
 cancel, done := nativeWorkbenchProbe.cancel, nativeWorkbenchProbe.done
 nativeWorkbenchProbe.mu.Unlock()
 if cancel == nil { return }
 cancel()
 select { case <-done: case <-time.After(3*time.Second): panic("native geometry observer shutdown exceeded 3s") }
}

func nativeWorkbenchBoundsWorker(ctx context.Context, cx *ui.Context, root string, done chan struct{}) {
 defer close(done)
 info, err := os.Lstat(root)
 if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 { return }
 ticker := time.NewTicker(20*time.Millisecond)
 defer ticker.Stop()
 var active, answered, floorFrame, floorGeneration uint64
 type request struct { Sequence uint64 ` + "`json:\"sequence\"`" + `; Tab string ` + "`json:\"tab\"`" + `; Keys []string ` + "`json:\"keys\"`" + ` }
 type response struct { Sequence uint64 ` + "`json:\"sequence\"`" + `; PID int ` + "`json:\"pid\"`" + `; Completed uint64 ` + "`json:\"completed\"`" + `; ViewGeneration uint64 ` + "`json:\"view_generation\"`" + `; TreeTab string ` + "`json:\"tree_tab\"`" + `; Bounds map[string]ui.Bounds ` + "`json:\"bounds\"`" + ` }
 for {
  select { case <-ctx.Done(): return; case <-ticker.C: }
  file, err := os.Open(filepath.Join(root,"request.json"))
  if err != nil { continue }
  data, err := io.ReadAll(io.LimitReader(file,4097)); _ = file.Close()
  var r request
  if err != nil || len(data)>4096 || json.Unmarshal(data,&r)!=nil || r.Sequence==0 || r.Sequence>32 || r.Sequence<=answered || len(r.Keys)<1 || len(r.Keys)>2 || (r.Tab!="Details" && r.Tab!="Feature Contributions") { continue }
  allowed := true
  for _, key := range r.Keys { if key!="extension-tab-Feature Contributions" && key!="extension-detail-command-gocode.hello" { allowed=false } }
  if !allowed { continue }
  reply := make(chan response,1)
  frozen := r
  if !uidispatch.Retry(ctx,cx.Dispatch,func() {
   snapshot := response{Sequence:frozen.Sequence,PID:os.Getpid(),Completed:cx.RenderedFrames(),ViewGeneration:nativeWorkbenchProbe.generation,TreeTab:nativeWorkbenchProbe.tab,Bounds:make(map[string]ui.Bounds,len(frozen.Keys))}
   for _, key := range frozen.Keys { if b, ok := cx.ElementBounds(key); ok { snapshot.Bounds[key]=b } }
   reply<-snapshot
   cx.Invalidate() // Request real frames, never fabricate counts or call View.
  }) { return }
  var snapshot response
  select { case <-ctx.Done(): return; case snapshot=<-reply: }
  if active!=r.Sequence { active=r.Sequence; floorFrame=snapshot.Completed+3; floorGeneration=snapshot.ViewGeneration }
  ready := snapshot.Completed>=floorFrame && snapshot.ViewGeneration>floorGeneration && snapshot.TreeTab==r.Tab
  for _, key := range r.Keys { b:=snapshot.Bounds[key]; if b.Width<=0 || b.Height<=0 { ready=false } }
  if !ready { continue }
  encoded, err := json.Marshal(snapshot)
  if err != nil || len(encoded)>4096 { return }
  path := filepath.Join(root,"response.json")
  if err = os.WriteFile(path+".pending",encoded,0600); err == nil { err=os.Rename(path+".pending",path) }
  if err != nil { return }
  answered=r.Sequence
 }
}
`
