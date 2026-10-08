//go:build windows

package languageextension

import (
	"errors"
	"fmt"
	"time"

	"github.com/neko233-com/gocode/internal/languageserver"
	"golang.org/x/sys/windows"
)

// Observe real kernel job membership, retain exact process handles, then check
// those same identities after shutdown. No global PID search or process kill.
func ObserveProcesses(client *languageserver.Client, kind string) ([]int, func() error, error) {
	deadline := time.Now().Add(3 * time.Second)
	var ids []int
	for {
		var err error
		ids, err = client.ProcessIDs()
		if err != nil {
			return nil, nil, err
		}
		if kind != "typescript" || len(ids) >= 2 {
			break
		}
		if time.Now().After(deadline) {
			return nil, nil, errors.New("actual TypeScript job had no tsserver descendant")
		}
		<-time.After(5 * time.Millisecond)
	}
	if kind == "go" {
		ids = []int{client.ProcessID()}
	} // go-list children may finish before a handle can be retained
	var handles []windows.Handle
	for _, id := range ids {
		h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(id))
		if err != nil {
			for _, old := range handles {
				windows.CloseHandle(old)
			}
			return nil, nil, fmt.Errorf("open observed server PID%d: %w", id, err)
		}
		handles = append(handles, h)
	}
	return ids, func() error {
		var failure error
		for i, h := range handles {
			result, err := windows.WaitForSingleObject(h, 3000)
			if err != nil {
				failure = errors.Join(failure, fmt.Errorf("observed language PID%d wait: %w", ids[i], err))
			} else if result != windows.WAIT_OBJECT_0 {
				failure = errors.Join(failure, fmt.Errorf("observed language PID%d survived owned shutdown (wait=%d)", ids[i], result))
			}
			failure = errors.Join(failure, windows.CloseHandle(h))
		}
		return failure
	}, nil
}
