//go:build windows

package terminal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"golang.org/x/sys/windows"
)

func embeddedConPTY() (map[string][]byte, error) {
	values := map[string][]byte{}
	for index, file := range conPTYFiles {
		info, err := windows.FindResource(0, windows.ResourceID(7701+index), windows.RT_RCDATA)
		if err != nil {
			if index == 0 {
				return nil, nil
			}
			return nil, err
		}
		size, err := windows.SizeofResource(0, info)
		if err != nil {
			return nil, err
		}
		if int64(size) != file.size {
			return nil, errors.New("invalid embedded ConPTY size")
		}
		data, err := windows.LoadResourceData(0, info)
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != file.sha {
			return nil, errors.New("embedded ConPTY SHA256 mismatch")
		}
		values[file.name] = data
	}
	return values, nil
}
