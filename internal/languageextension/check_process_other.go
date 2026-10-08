//go:build !windows

package languageextension

import "github.com/neko233-com/gocode/internal/languageserver"

func ObserveProcesses(client *languageserver.Client, kind string) ([]int, func() error, error) {
	// Unix process-group ownership is tested separately. Do not invent a complete
	// child inventory on a platform without the exact Job membership API.
	return nil, func() error { return nil }, nil
}
