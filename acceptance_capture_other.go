//go:build !windows || !cgo

package main

func captureCopilotAcceptance(string) error   { return nil }
func captureLargefileAcceptance(string) error { return nil }
func captureLSPAcceptance(string) error       { return nil }
