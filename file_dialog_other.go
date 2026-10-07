//go:build !windows || !cgo

package main

import (
	"context"
	"errors"
)

func nativeChooseFile(context.Context, string, string, string) (string, error) {
	return "", errors.New("native file dialogs currently target Windows")
}
