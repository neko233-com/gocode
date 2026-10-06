package main

import "runtime/debug"

var version = "0.4.0"
var sourceCommit = "development"

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
