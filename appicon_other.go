//go:build !windows || !cgo

package main

func applyAppIcon(string) func() { return func() {} }
