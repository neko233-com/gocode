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
	"syscall"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/update"
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
	configRoot := t.TempDir()
	cmd := exec.Command(exe, "-workspace", workspace, "-extensions-dir", t.TempDir(), "-copilot=false", "-lsp=false")
	cmd.Env = append(os.Environ(), "GODESKTOP_READBACK=1", "APPDATA="+configRoot)
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	var w winprobe.Window
	t.Cleanup(func() {
		if t.Failed() {
			data, err := os.ReadFile(filepath.Join(workspace, "main.go"))
			t.Logf("owned source after failure: %q (%v); native output: %s", data, err, output.String())
			if name := os.Getenv("GOCODE_SCREENSHOT"); name != "" && w != 0 {
				if pixels, err := w.Capture(); err == nil {
					if f, err := os.Create(name + ".failed.png"); err == nil {
						_ = png.Encode(f, pixels)
						_ = f.Close()
					}
				}
			}
		}
	})
	until(t, func() bool {
		var err error
		w, err = winprobe.Find("gocode — "+filepath.Base(workspace), uint32(cmd.Process.Pid))
		return err == nil
	})
	for _, kind := range []uintptr{0, 1} {
		until(t, func() bool {
			icon, _, _ := syscall.NewLazyDLL("user32.dll").NewProc("SendMessageW").Call(uintptr(w), 0x7f, kind, 0)
			return icon != 0
		})
	}
	w.Raise()
	unpin := w.Pin()
	defer unpin()
	until(t, func() bool { color, err := w.Pixel(500, 400); return err == nil && color == editor })
	// Welcome and the pending-open banner can paint before startup finishes.
	// Wait for the actual first source row at its final input coordinates.
	until(t, func() bool { color, err := w.Pixel(600, 105); return err == nil && color == 0x282828 })
	pixelWidth, pixelHeight, err := w.ClientSize()
	if err != nil {
		t.Fatal(err)
	}
	dpi := float64(w.DPI()) / 96
	viewWidth, viewHeight := int(float64(pixelWidth)/dpi), int(float64(pixelHeight)/dpi)
	for _, point := range []struct {
		x, y  int
		color uint32
	}{{12, 250, outer}, {100, min(600, viewHeight-220), outer}, {500, 400, editor}, {1, viewHeight - 10, accent}} {
		color, err := w.Pixel(point.x, point.y)
		if err != nil || color != point.color {
			t.Fatalf("pixel %d,%d=%06x expected %06x (%v)", point.x, point.y, color, point.color, err)
		}
	}
	if hit, err := w.HitTest(1, 1); err != nil || hit != 13 {
		t.Fatalf("resize corner %d %v", hit, err)
	}
	dragFound := false
	for x := 320; x < viewWidth-140; x += 10 {
		if hit, err := w.HitTest(x, 16); err != nil {
			t.Fatal(err)
		} else if hit == 2 {
			dragFound = true
			break
		}
	}
	if !dragFound {
		t.Fatal("titlebar has no native drag region")
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
	// Select the first shaped CJK character through actual captured mouse input,
	// then replace the selection without splitting the following surrogate pair.
	if err := w.Pointer(0x201, 358, 105); err != nil {
		t.Fatal(err)
	}
	moveX, moveY := int16(370*dpi), int16(105*dpi)
	if err := w.Send(0x200, 1, uintptr(uint32(uint16(moveX))|uint32(uint16(moveY))<<16)); err != nil {
		t.Fatal(err)
	}
	if err := w.Pointer(0x202, 370, 105); err != nil {
		t.Fatal(err)
	}
	if err := w.Send(0x102, 0x754c, 0); err != nil {
		t.Fatal(err)
	}
	click(54, 16)
	// The native group tree is now initialized before service startup, so the
	// workspace-wide command palette precedes the group tabs and breadcrumb.
	// Click its actual first action rather than the old single-editor location.
	until(t, func() bool { color, err := w.Pixel(700, 90); return err == nil && color == 0x252526 })
	click(430, 90)
	until(t, func() bool {
		data, _ := os.ReadFile(filepath.Join(workspace, "main.go"))
		return strings.HasPrefix(string(data), "界😀package main")
	})
	// Close the save notification, then execute an installed VSIX from its UI.
	click(viewWidth-15, 50)
	click(24, 250)
	click(130, 215)
	until(t, func() bool { color, err := w.Pixel(700, 50); return err == nil && color == 0x252526 })
	if name := os.Getenv("GOCODE_SCREENSHOT"); name != "" {
		image, err := w.Capture()
		if err != nil {
			t.Fatal(err)
		}
		scale := float64(w.DPI()) / 96
		p := image.RGBAAt(int(scale), int(float64(viewHeight-10)*scale))
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
	// Real native settings clicks and text events persist only this fixture's
	// per-user config. They must not change the active editor or contact AI.
	click(24, viewHeight-46)
	click(120, 262)
	for _, r := range "https://ghfast.top/" {
		if err := w.Send(0x102, uintptr(r), 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Send(0x100, 13, 0); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(configRoot, "gocode", "updates.json")
	until(t, func() bool {
		config, err := update.LoadConfig(configPath)
		return err == nil && config.Mirror == "https://ghfast.top/"
	})
	click(120, 210)
	until(t, func() bool {
		config, err := update.LoadConfig(configPath)
		return err == nil && config.Mode == "mirror"
	})
	click(120, 126)
	until(t, func() bool {
		config, err := update.LoadConfig(configPath)
		return err == nil && !config.Auto && config.Mode == "mirror"
	})
	if name := os.Getenv("GOCODE_UPDATES_SCREENSHOT"); name != "" {
		pixels, err := w.Capture()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, pixels)
		closeErr := f.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("settings capture %v %v", err, closeErr)
		}
	}
	click(viewWidth-69, 16)
	user32 := syscall.NewLazyDLL("user32.dll")
	isZoomed := user32.NewProc("IsZoomed")
	isIconic := user32.NewProc("IsIconic")
	until(t, func() bool { value, _, _ := isZoomed.Call(uintptr(w)); return value != 0 })
	maxWidth, maxHeight, err := w.ClientSize()
	if err != nil {
		t.Fatal(err)
	}
	maxScale := float64(w.DPI()) / 96
	until(t, func() bool {
		color, err := w.Pixel(1, int(float64(maxHeight)/maxScale)-10)
		return err == nil && color == accent
	})
	click(int(float64(maxWidth)/maxScale)-69, 16)
	until(t, func() bool {
		value, _, _ := isZoomed.Call(uintptr(w))
		width, height, err := w.ClientSize()
		return value == 0 && err == nil && width == pixelWidth && height == pixelHeight
	})
	click(viewWidth-115, 16)
	until(t, func() bool { value, _, _ := isIconic.Call(uintptr(w)); return value != 0 })
	w.Show(9) // SW_RESTORE, as when restoring from the taskbar.
	until(t, func() bool {
		value, _, _ := isIconic.Call(uintptr(w))
		width, height, err := w.ClientSize()
		return value == 0 && err == nil && width == pixelWidth && height == pixelHeight
	})
	w.Raise()
	// Click immediately after restore: the framework must refresh hit targets
	// before pointer input even if the next DXGI-paced draw has not happened.
	click(viewWidth-23, 16)
	isWindow := user32.NewProc("IsWindow")
	until(t, func() bool { value, _, _ := isWindow.Call(uintptr(w)); return value == 0 })
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
