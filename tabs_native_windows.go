//go:build windows && cgo

package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

// SendMessage does not update keyboard-state tables. Set only this owned UI
// thread's state during the test message, restoring it immediately. This does
// not post global input or change other applications' keyboard queues.
func tabsNativeKey(cx *ui.Context, workspace string, key, mods int, pressed bool, done func(error)) {
	cx.Dispatch(func() {
		w, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
		if err != nil {
			done(err)
			return
		}
		user := syscall.NewLazyDLL("user32.dll")
		thread, _, _ := user.NewProc("GetWindowThreadProcessId").Call(uintptr(w), 0)
		current, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId").Call()
		if thread != current {
			done(fmt.Errorf("keyboard probe must run on the owned UI thread"))
			return
		}
		var saved [256]byte
		ok, _, err := user.NewProc("GetKeyboardState").Call(uintptr(unsafe.Pointer(&saved[0])))
		if ok == 0 {
			done(err)
			return
		}
		state := saved
		for _, vk := range []int{16, 17, 18, 91, 92} {
			state[vk] &= 0x7f
		}
		for _, pair := range [][2]int{{16, ui.ModifierShift}, {17, ui.ModifierControl}, {18, ui.ModifierAlt}, {91, ui.ModifierCommand}} {
			if mods&pair[1] != 0 {
				state[pair[0]] |= 0x80
			}
		}
		ok, _, err = user.NewProc("SetKeyboardState").Call(uintptr(unsafe.Pointer(&state[0])))
		if ok == 0 {
			done(err)
			return
		}
		message := uint32(0x101)
		if pressed {
			message = 0x100
		}
		err = w.Send(message, uintptr(key), 0)
		ok, _, restoreErr := user.NewProc("SetKeyboardState").Call(uintptr(unsafe.Pointer(&saved[0])))
		if ok == 0 && err == nil {
			err = restoreErr
		}
		done(err)
	})
}

func tabsNativeWheel(cx *ui.Context, workspace string, x, y float32, done func(error)) {
	go func() {
		w, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
		if err == nil {
			err = w.Wheel(true, int(x), int(y), -120, 0)
		}
		cx.Dispatch(func() { done(err) })
	}()
}

func tabsNativePointer(cx *ui.Context, workspace string, x, y float32, pressed bool, done func(error)) {
	go func() {
		w, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
		message := uint32(0x202)
		if pressed {
			message = 0x201
		}
		if err == nil {
			err = w.Pointer(message, int(x), int(y))
		}
		cx.Dispatch(func() { done(err) })
	}()
}
func tabsNativeDrag(cx *ui.Context, workspace string, from, to ui.Bounds, done func(error)) {
	go func() {
		w, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
		x, y := int(from.X+from.Width/2), int(from.Y+from.Height/2)
		if err == nil {
			err = w.Pointer(0x201, x, y)
		}
		if err == nil {
			scale := float64(w.DPI()) / 96
			param := uintptr(uint32(uint16(int16(math.Round(float64(to.X)*scale)))) | uint32(uint16(int16(math.Round(float64(y)*scale))))<<16)
			err = w.Send(0x200, 1, param)
		}
		if err == nil {
			err = w.Pointer(0x202, int(to.X), y)
		}
		cx.Dispatch(func() { done(err) })
	}()
}
