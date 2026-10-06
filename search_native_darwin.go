//go:build darwin && cgo

package main

import (
	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/metalprobe"
)

func searchNativeText(cx *ui.Context, _ string, text string, done func(error)) {
	cx.Dispatch(func() {
		for _, r := range text {
			if err := metalprobe.Key(int(r), 0, true); err != nil {
				done(err)
				return
			}
		}
		done(nil)
	})
}
