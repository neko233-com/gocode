package languageextension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/neko233-com/gocode/internal/installlayout"
)

const NativeAdapterMinimumVersion = "0.24.0"
const MaxStateBytes = 128 << 10

// State is the existing user preference schema. A deferred uninstall, like a
// disabled extension, is excluded from activation before its files are removed.
type State struct {
	Disabled  []string `json:"disabled"`
	Uninstall []string `json:"uninstall"`
}

func ReadState(root string) (State, error) {
	var state State
	f, err := regular(root, ".gocode-state.json", MaxStateBytes)
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxStateBytes+1))
	if err != nil {
		return state, err
	}
	if len(data) > MaxStateBytes {
		return state, errors.New("extension state exceeds limit")
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, err
	}
	if len(state.Disabled) > 512 || len(state.Uninstall) > 512 {
		return state, errors.New("too many extension state entries")
	}
	return state, nil
}

func stateContains(ids []string, id string) bool {
	for _, value := range ids {
		if strings.EqualFold(value, id) {
			return true
		}
	}
	return false
}

// CheckRollback is read-only and must run off the UI thread, before selecting
// an older executable. Actual installed bytes and the native installation
// receipt identify an adapter; a stale receipt without its package is harmless.
func CheckRollback(ctx context.Context, root, targetVersion string) error {
	if !installlayout.ValidVersion(targetVersion) || root == "" {
		return errors.New("rollback requires an exact version and extension directory")
	}
	if installlayout.CompareVersion(targetVersion, NativeAdapterMinimumVersion) >= 0 {
		return ctx.Err()
	}
	state, err := ReadState(root)
	if err != nil {
		return fmt.Errorf("cannot check rollback extension preferences: %w", err)
	}
	var enabled []string
	for _, kind := range []string{"go", "typescript"} {
		if err := ctx.Err(); err != nil {
			return err
		}
		pin, _ := Package(kind)
		if stateContains(state.Disabled, pin.ID) || stateContains(state.Uninstall, pin.ID) {
			continue
		}
		if _, err := os.Lstat(receiptPath(root, kind)); errors.Is(err, os.ErrNotExist) {
			continue // ordinary VSIX installation is not a native adapter migration
		} else if err != nil {
			return err
		}
		info, err := os.Lstat(installedPath(root, pin))
		if errors.Is(err, os.ErrNotExist) {
			continue // uninstall leaves an intentionally persistent marker
		}
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("cannot verify native language package for rollback")
		}
		if _, err := VerifyInstalled(root, kind); err != nil {
			return fmt.Errorf("cannot verify %s before rollback; disable its language support first: %w", pin.ID, err)
		}
		enabled = append(enabled, pin.ID)
	}
	if len(enabled) != 0 {
		return fmt.Errorf("cannot roll back to %s while native language support is enabled (%s); disable these extensions first, then retry rollback", targetVersion, strings.Join(enabled, ", "))
	}
	return ctx.Err()
}
