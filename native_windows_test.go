//go:build windows && cgo

package main

import (
	"bytes"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/testing/winprobe"
)

type safeOutput struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *safeOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}
func (b *safeOutput) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }
func until(t *testing.T, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("native condition timed out")
}
func TestNativeWorkbenchAMD64(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("Windows amd64 integration")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	restore := winprobe.Awareness()
	defer restore()
	exe := filepath.Join(t.TempDir(), "gocode.exe")
	build := exec.Command("go", "build", "-race", "-o", exe, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build %v\n%s", err, output)
	}
	if err := winprobe.ValidateAMD64PE(exe); err != nil {
		t.Fatal(err)
	}
	m := testModel(t)
	workspace := m.workspace
	var output safeOutput
	cmd := exec.Command(exe, "-workspace", workspace, "-extensions-dir", t.TempDir())
	cmd.Env = append(os.Environ(), "GODESKTOP_READBACK=1")
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	var w winprobe.Window
	until(t, func() bool {
		var err error
		w, err = winprobe.Find("gocode — "+filepath.Base(workspace), uint32(cmd.Process.Pid))
		return err == nil
	})
	w.Raise()
	unpin := w.Pin()
	defer unpin()
	until(t, func() bool { color, err := w.Pixel(500, 400); return err == nil && color == editor })
	for _, point := range []struct {
		x, y  int
		color uint32
	}{{12, 250, outer}, {100, 600, outer}, {500, 400, editor}, {10, 809, accent}} {
		color, err := w.Pixel(point.x, point.y)
		if err != nil || color != point.color {
			t.Fatalf("pixel %d,%d=%06x expected %06x (%v)", point.x, point.y, color, point.color, err)
		}
	}
	if hit, err := w.HitTest(1, 1); err != nil || hit != 13 {
		t.Fatalf("resize corner %d %v", hit, err)
	}
	if hit, err := w.HitTest(430, 16); err != nil || hit != 2 {
		t.Fatalf("draggable title region %d %v", hit, err)
	}
	click := func(x, y int) {
		t.Helper()
		if err := w.Pointer(0x201, x, y); err != nil {
			t.Fatal(err)
		}
		if err := w.Pointer(0x202, x, y); err != nil {
			t.Fatal(err)
		}
		_ = w.Send(0, 0, 0)
		time.Sleep(60 * time.Millisecond)
	}
	click(360, 105)
	for _, r := range []int{0x4f60, 0xd83d, 0xde00} {
		if err := w.Send(0x102, uintptr(r), 0); err != nil {
			t.Fatal(err)
		}
	}
	click(54, 16)
	click(430, 146)
	until(t, func() bool {
		data, _ := os.ReadFile(filepath.Join(workspace, "main.go"))
		return strings.HasPrefix(string(data), "你😀package main")
	})
	// Close the save notification, then execute an installed VSIX from its UI.
	click(1260, 110)
	click(24, 250)
	click(130, 215)
	until(t, func() bool { color, err := w.Pixel(700, 108); return err == nil && color == 0x252526 })
	if name := os.Getenv("GOCODE_SCREENSHOT"); name != "" {
		image, err := w.Capture()
		if err != nil {
			t.Fatal(err)
		}
		scale := float64(w.DPI()) / 96
		p := image.RGBAAt(int(10*scale), int(809*scale))
		if p.R != 0 || p.G != 0x78 || p.B != 0xd4 {
			t.Fatalf("capture is not the workbench frame: %v", p)
		}
		if err = os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, image); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	click(1210, 16)
	time.Sleep(120 * time.Millisecond)
	click(1210, 16)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("exit %v %s", err, output.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("window failed to close")
	}
	if strings.Contains(output.String(), "DATA RACE") {
		t.Fatal(output.String())
	}
	smoke := exec.Command(exe, "-workspace", workspace, "-extensions-dir", t.TempDir(), "-smoke")
	if data, err := smoke.CombinedOutput(); err != nil || !strings.Contains(string(data), "gocode smoke passed") {
		t.Fatalf("smoke %v %s", err, data)
	}
}
