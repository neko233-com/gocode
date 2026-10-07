package main

import (
	"flag"
	"io"
	"testing"
)

func TestAutoSaveSessionFlagsDistinguishAbsentAndInvalidOverrides(t *testing.T) {
	stored := autoSaveConfig{Mode: "onWindowChange", DelayMS: 3000}
	for _, test := range []struct {
		name string
		args []string
		want autoSaveConfig
		bad  bool
	}{
		{name: "absent retains stored policy", want: stored},
		{name: "unrelated flag retains stored policy", args: []string{"-other"}, want: stored},
		{name: "mode only retains stored delay", args: []string{"-auto-save=off"}, want: autoSaveConfig{"off", 3000}},
		{name: "delay only retains stored mode", args: []string{"-auto-save-delay=500"}, want: autoSaveConfig{"onWindowChange", 500}},
		{name: "both overrides", args: []string{"-auto-save=onFocusChange", "-auto-save-delay=250"}, want: autoSaveConfig{"onFocusChange", 250}},
		{name: "explicit zero rejected", args: []string{"-auto-save-delay=0"}, bad: true},
		{name: "negative rejected", args: []string{"-auto-save-delay=-1"}, bad: true},
		{name: "below minimum rejected", args: []string{"-auto-save-delay=99"}, bad: true},
		{name: "minimum accepted", args: []string{"-auto-save-delay=100"}, want: autoSaveConfig{"onWindowChange", 100}},
		{name: "maximum accepted", args: []string{"-auto-save-delay=600000"}, want: autoSaveConfig{"onWindowChange", 600000}},
		{name: "above maximum rejected", args: []string{"-auto-save-delay=600001"}, bad: true},
		{name: "non integer rejected", args: []string{"-auto-save-delay=invalid"}, bad: true},
		{name: "explicit empty mode rejected", args: []string{"-auto-save="}, bad: true},
		{name: "unknown mode rejected", args: []string{"-auto-save=enabled"}, bad: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			flags := flag.NewFlagSet("auto-save", flag.ContinueOnError)
			flags.SetOutput(io.Discard)
			flags.String("auto-save", "", "")
			flags.Int("auto-save-delay", 0, "")
			flags.Bool("other", false, "")
			parseErr := flags.Parse(test.args)
			got, err := autoSaveSessionConfig(stored, flags)
			if test.bad {
				if parseErr == nil && err == nil {
					t.Fatalf("invalid explicit override accepted: args=%v config=%+v", test.args, got)
				}
				return
			}
			if parseErr != nil || err != nil || got != test.want {
				t.Fatalf("args=%v config=%+v want=%+v parse=%v validation=%v", test.args, got, test.want, parseErr, err)
			}
			if stored != (autoSaveConfig{Mode: "onWindowChange", DelayMS: 3000}) {
				t.Fatal("session override mutated the stored configuration")
			}
		})
	}
}
