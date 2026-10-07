//go:build windows && cgo

package main

import (
	_ "embed"
	"encoding/binary"
	"os"
	"syscall"
	"unsafe"
)

//go:embed assets/code-oss/code.ico
var appIconData []byte

// applyAppIcon sets both taskbar and window icons on this process's HWND. The
// source-run fallback uses the same upstream ICO as the executable/MSI resource.
func applyAppIcon(title string) func() {
	w, err := ownedWorkbenchWindow(title, uint32(os.Getpid()))
	if err != nil {
		return func() {}
	}
	user := syscall.NewLazyDLL("user32.dll")
	module, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	var owned []uintptr
	for _, size := range []uintptr{16, 32} {
		icon, _, _ := user.NewProc("LoadImageW").Call(module, 1, 1, size, size, 0x8000)
		if icon == 0 && len(appIconData) >= 6 {
			count := int(binary.LittleEndian.Uint16(appIconData[4:6]))
			best, distance := -1, 1000
			for i := 0; i < count && 6+(i+1)*16 <= len(appIconData); i++ {
				entry := appIconData[6+i*16 : 6+(i+1)*16]
				width := int(entry[0])
				if width == 0 {
					width = 256
				}
				delta := width - int(size)
				if delta < 0 {
					delta = -delta
				}
				if delta < distance {
					best, distance = i, delta
				}
			}
			if best >= 0 {
				entry := appIconData[6+best*16 : 6+(best+1)*16]
				length, offset := int(binary.LittleEndian.Uint32(entry[8:12])), int(binary.LittleEndian.Uint32(entry[12:16]))
				if length > 0 && offset >= 0 && offset <= len(appIconData)-length {
					image := appIconData[offset : offset+length]
					icon, _, _ = user.NewProc("CreateIconFromResourceEx").Call(uintptr(unsafe.Pointer(&image[0])), uintptr(len(image)), 1, 0x30000, size, size, 0)
					if icon != 0 {
						owned = append(owned, icon)
					}
				}
			}
		}
		if icon != 0 {
			kind := uintptr(0)
			if size == 32 {
				kind = 1
			}
			user.NewProc("SendMessageW").Call(uintptr(w), 0x80, kind, icon)
		}
	}
	return func() {
		for _, icon := range owned {
			user.NewProc("DestroyIcon").Call(icon)
		}
	}
}
