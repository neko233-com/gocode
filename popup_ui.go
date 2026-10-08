package main

import ui "github.com/neko233-com/godesktop"

// Code-OSS 1.141.0, 2a59476c9bfcb90b3ddc372c36762471b7dfad1c:
// style.css defines shadow-lg/xl; menu.ts selects lg, quickInput.css and
// dialog.css select xl. Modern UI keeps floating shadows and an 8-DIP radius.
func menuPopupShadow() ui.ShadowStyle {
	return ui.ShadowStyle{Color: ui.RGBA(0, .14), Blur: 12}
}

func modalPopupShadow() ui.ShadowStyle {
	return ui.ShadowStyle{Color: ui.RGBA(0, .15), Blur: 20}
}

// Match the old zero-basis centering spacers, including undersized windows.
// A direct Stack child keeps its body/hit geometry while its shadow receives
// the real window clip instead of a tight intermediate Row/Column clip.
func centeredPopup(popup *ui.Element, width, height, windowWidth, windowHeight float32) *ui.Element {
	return popup.Height(height).Position(max(0, (windowWidth-width)/2), max(0, (windowHeight-height)/2))
}
