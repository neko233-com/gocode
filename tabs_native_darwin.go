//go:build darwin && cgo

package main

import (
	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/metalprobe"
)

func tabsNativeKey(cx *ui.Context, _ string, key, mods int, pressed bool, done func(error)) {
	cx.Dispatch(func() { done(metalprobe.Key(key, mods, pressed)) })
}
func tabsNativeWheel(cx *ui.Context, _ string, x, y float32, done func(error)) {
	cx.Dispatch(func() { done(metalprobe.Wheel(36, 0, x, y, 0, true)) })
}
func tabsNativePointer(cx *ui.Context, _ string, x, y float32, pressed bool, done func(error)) {
	cx.Dispatch(func() { done(metalprobe.Pointer(pressed, x, y, 0)) })
}
func tabsNativeDrag(cx *ui.Context, _ string, from, to ui.Bounds, done func(error)) {
	cx.Dispatch(func() {
		x, y, endX := from.X+from.Width/2, from.Y+from.Height/2, to.X
		if err := metalprobe.Pointer(true, x, y, 0); err != nil {
			done(err)
			return
		}
		err := metalprobe.Drag(endX, y, 0)
		releaseErr := metalprobe.Pointer(false, endX, y, 0)
		if err == nil {
			err = releaseErr
		}
		done(err)
	})
}
