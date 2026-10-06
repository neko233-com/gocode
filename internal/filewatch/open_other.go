//go:build !windows

package filewatch

import "os"

func OpenRead(path string) (*os.File, error) { return os.Open(path) }
func renameFile(source, target string) error { return os.Rename(source, target) }
func transientReplace(error) bool            { return false }
