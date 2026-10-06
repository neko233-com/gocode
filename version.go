package main

import (
	_ "embed"
	"runtime/debug"
	"strings"
)

var version = ""
var sourceCommit = "development"

//go:embed VERSION
var defaultVersion string

func appVersion() string {
	if version != "" {
		return version
	}
	return strings.TrimSpace(defaultVersion)
}

func buildCommit() string {
	if sourceCommit != "development" {
		return sourceCommit
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return sourceCommit
}
