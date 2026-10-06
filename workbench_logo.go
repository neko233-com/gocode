package main

import (
	"bytes"
	_ "embed"
	"image/png"

	ui "github.com/neko233-com/godesktop"
)

//go:embed assets/code-oss/code.png
var workbenchLogoPNG []byte

// Decode once before native startup, never inside a frame callback. The same
// immutable upstream asset is reused through resize, DPI and device recovery.
func loadWorkbenchLogo() (*ui.Bitmap, error) {
	source, err := png.Decode(bytes.NewReader(workbenchLogoPNG))
	if err != nil {
		return nil, err
	}
	return ui.NewBitmap(source)
}
