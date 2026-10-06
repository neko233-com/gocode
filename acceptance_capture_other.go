//go:build !windows || !cgo

package main

func captureCopilotAcceptance(string) error            { return nil }
func captureLargefileAcceptance(string) error          { return nil }
func captureLSPAcceptance(string) error                { return nil }
func captureCloseAcceptance(string, string) error      { return nil }
func captureTerminalAcceptance(string, string) error   { return nil }
func resizeTerminalAcceptance(string) (float32, error) { return 0, nil }
