//go:build !windows

package terminal

func embeddedConPTY() (map[string][]byte, error) { return nil, nil }
