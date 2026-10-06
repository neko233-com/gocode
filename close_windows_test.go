//go:build windows && cgo

package main

import (
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/testing/winprobe"
)

func TestNativeUnsavedClose(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	restore := winprobe.Awareness()
	defer restore()
	exe := filepath.Join(t.TempDir(), "gocode.exe")
	if data, err := exec.Command("go", "build", "-race", "-o", exe, ".").CombinedOutput(); err != nil {
		t.Fatalf("build %v %s", err, data)
	}
	for _, mode := range []string{"save", "discard", "cancel", "external"} {
		t.Run(mode, func(t *testing.T) {
			m := testModel(t)
			source := m.current().path
			original, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			var output safeOutput
			cmd := exec.Command(exe, "-workspace", m.workspace, "-extensions-dir", t.TempDir(), "-copilot=false", "-lsp=false")
			cmd.Env = append(os.Environ(), "GODESKTOP_READBACK=1", "APPDATA="+t.TempDir())
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			var window winprobe.Window
			until(t, func() bool {
				var err error
				window, err = winprobe.Find("gocode — "+filepath.Base(m.workspace), uint32(cmd.Process.Pid))
				return err == nil
			})
			window.Raise()
			unpin := window.Pin()
			defer unpin()
			width, height, err := window.ClientSize()
			if err != nil {
				t.Fatal(err)
			}
			// ClientSize returns physical pixels; Pointer accepts DIP and applies
			// DPI itself. Keep the modal position in logical coordinates.
			width, height = width*96/int(window.DPI()), height*96/int(window.DPI())
			discardX, discardY := width/2+50, height/2+31
			until(t, func() bool { color, err := window.Pixel(500, 400); return err == nil && color == editor })
			// An asynchronous startup can paint Welcome before the document is
			// loaded. Wait for actual main.go tab glyphs before sending input.
			until(t, func() bool {
				pixels, err := window.Capture()
				if err != nil {
					return false
				}
				scale := float64(window.DPI()) / 96
				ink := 0
				for y := int(42 * scale); y < int(62*scale); y++ {
					for x := int(323 * scale); x < int(375*scale); x++ {
						r, g, b, _ := pixels.At(x, y).RGBA()
						if r>>8 > 150 && g>>8 > 150 && b>>8 > 150 {
							ink++
						}
					}
				}
				return ink > 30
			})
			until(t, func() bool { color, err := window.Pixel(600, 105); return err == nil && color == 0x282828 })
			click := func(x, y int) {
				t.Helper()
				if err := window.Pointer(0x201, x, y); err != nil {
					t.Fatal(err)
				}
				if err := window.Pointer(0x202, x, y); err != nil {
					t.Fatal(err)
				}
				time.Sleep(40 * time.Millisecond)
			}
			click(360, 105)
			if err := window.Send(0x102, 'X', 0); err != nil {
				t.Fatal(err)
			}
			if mode == "external" {
				if err := os.WriteFile(source, append(append([]byte{}, original...), []byte("// external edit\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := window.Close(); err != nil {
				t.Fatal(err)
			}
			until(t, func() bool { color, err := window.Pixel(418, 350); return err == nil && color == 0x252526 })
			if _, _, err := window.ClientSize(); err != nil {
				t.Fatal("dirty OS close destroyed the window", err)
			}
			if directory := os.Getenv("GOCODE_CLOSE_SCREENSHOTS"); directory != "" {
				pixels, err := window.Capture()
				if err != nil {
					t.Fatal(err)
				}
				if err := os.MkdirAll(directory, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(directory, mode+".png"))
				if err != nil {
					t.Fatal(err)
				}
				encodeErr := png.Encode(f, pixels)
				closeErr := f.Close()
				if encodeErr != nil || closeErr != nil {
					t.Fatal(encodeErr, closeErr)
				}
			}
			if mode == "cancel" {
				if err := window.Send(0x100, 27, 0); err != nil {
					t.Fatal(err)
				}
				until(t, func() bool { color, err := window.Pixel(500, 400); return err == nil && color == editor })
				if data, _ := os.ReadFile(source); string(data) != string(original) {
					t.Fatal("cancel saved/discarded source")
				}
				// Cancel restores editing focus; this character needs no extra click.
				if err := window.Send(0x102, 'Y', 0); err != nil {
					t.Fatal(err)
				}
				if err := window.Close(); err != nil {
					t.Fatal(err)
				}
				until(t, func() bool { color, err := window.Pixel(418, 350); return err == nil && color == 0x252526 })
			}
			if mode == "discard" {
				click(discardX, discardY)
			} else {
				if err := window.Send(0x100, 13, 0); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "external" {
				time.Sleep(150 * time.Millisecond)
				if _, _, err := window.ClientSize(); err != nil {
					t.Fatal("failed save closed the window", err)
				}
				if data, _ := os.ReadFile(source); string(data) != string(original)+"// external edit\n" {
					t.Fatal("external file overwritten")
				}
				// Failure keeps the buffer/modal. Explicitly discard only this fixture.
				if err := window.Send(0x100, 27, 0); err != nil {
					t.Fatal(err)
				}
				until(t, func() bool { color, err := window.Pixel(500, 400); return err == nil && color == editor })
				if err := window.Close(); err != nil {
					t.Fatal(err)
				}
				until(t, func() bool { color, err := window.Pixel(418, 350); return err == nil && color == 0x252526 })
				click(discardX, discardY)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("exit %v %s", err, output.String())
				}
			case <-time.After(6 * time.Second):
				t.Fatal("confirmed native close timed out", output.String())
			}
			data, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "save":
				if string(data) != "X"+string(original) {
					t.Fatalf("saved source %q", data)
				}
			case "cancel":
				if string(data) != "XY"+string(original) {
					t.Fatalf("cancel focus/save source %q", data)
				}
			case "discard":
				if string(data) != string(original) {
					t.Fatal("discard wrote source")
				}
			}
			if strings.Contains(output.String(), "DATA RACE") {
				t.Fatal(output.String())
			}
		})
	}
}
