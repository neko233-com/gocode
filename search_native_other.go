//go:build (!windows && !darwin) || !cgo

package main

import (
	"errors"
	ui "github.com/neko233-com/godesktop"
)

func searchNativeText(_ *ui.Context, _, _ string, done func(error)) {
	done(errors.New("native search requires Windows/macOS and cgo"))
}
