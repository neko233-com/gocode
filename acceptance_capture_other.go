//go:build !windows || !cgo

package main

import ui "github.com/neko233-com/godesktop"

func captureCopilotAcceptance(string) error                                 { return nil }
func captureLargefileAcceptance(string) error                               { return nil }
func captureLSPAcceptance(string) error                                     { return nil }
func captureRecoveredLSPAcceptance(string, ui.Bounds, ui.Bounds) error      { return nil }
func captureCloseAcceptance(string, string) error                           { return nil }
func captureTerminalAcceptance(string, string, ui.Bounds) error             { return nil }
func resizeTerminalAcceptance(string) (float32, error)                      { return 0, nil }
func captureFileWatchAcceptance(string, string, ui.Bounds, ui.Bounds) error { return nil }
func activateFileWatchControl(_ *ui.Context, _ string, _ ui.Bounds, fallback func(), done func(error)) {
	fallback()
	done(nil)
}
