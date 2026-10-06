//go:build darwin && cgo

package main

/*
#include <stdlib.h>
const char *gocode_search_test_text(const char *title, const char *text);
*/
import "C"

import (
	"errors"
	ui "github.com/neko233-com/godesktop"
	"path/filepath"
	"unsafe"
)

func searchNativeText(cx *ui.Context, workspace string, text string, done func(error)) {
	cx.Dispatch(func() {
		title, chars := C.CString("gocode — "+filepath.Base(workspace)), C.CString(text)
		defer C.free(unsafe.Pointer(title))
		defer C.free(unsafe.Pointer(chars))
		var err error
		if message := C.gocode_search_test_text(title, chars); message != nil {
			err = errors.New(C.GoString(message))
		}
		done(err)
	})
}
