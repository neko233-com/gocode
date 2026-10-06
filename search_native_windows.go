//go:build windows && cgo

package main

import (
	"os"
	"path/filepath"
	"unicode/utf16"

	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

func searchNativeText(cx *ui.Context, workspace, text string, done func(error)) {
	go func() {
		w, err := winprobe.Find("gocode — "+filepath.Base(workspace), uint32(os.Getpid()))
		if err == nil {
			for _, ch := range utf16.Encode([]rune(text)) {
				if err = w.Send(0x102, uintptr(ch), 0); err != nil {
					break
				}
			}
		}
		cx.Dispatch(func() { done(err) })
	}()
}
