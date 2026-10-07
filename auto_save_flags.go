package main

import (
	"flag"
	"strconv"
)

// Visited flags distinguish an absent override from explicitly invalid zero or
// empty values. Session overrides preserve the unspecified stored property and
// never write user configuration merely because the application was launched.
func autoSaveSessionConfig(config autoSaveConfig, flags *flag.FlagSet) (autoSaveConfig, error) {
	var parseErr error
	flags.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "auto-save":
			config.Mode = f.Value.String()
		case "auto-save-delay":
			config.DelayMS, parseErr = strconv.Atoi(f.Value.String())
		}
	})
	if parseErr != nil {
		return config, parseErr
	}
	return config, validateAutoSaveConfig(config)
}
