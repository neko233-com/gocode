package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAutoSaveConfigDefaultsAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	if config, err := readAutoSaveConfig(path); err != nil || config != defaultAutoSaveConfig() {
		t.Fatal("missing settings did not return defaults", config, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("reading defaults created a file", err)
	}
	for _, mode := range []string{"off", "afterDelay", "onFocusChange", "onWindowChange"} {
		for _, delay := range []int{100, 1000, 600000} {
			if err := validateAutoSaveConfig(autoSaveConfig{Mode: mode, DelayMS: delay}); err != nil {
				t.Fatal(mode, delay, err)
			}
		}
	}
	for _, config := range []autoSaveConfig{{"", 1000}, {"always", 1000}, {"off", 0}, {"afterDelay", 99}, {"onFocusChange", 600001}} {
		if err := validateAutoSaveConfig(config); err == nil {
			t.Fatal("invalid settings accepted", config)
		}
	}
}

func TestAutoSaveConfigBoundedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autosave.json")
	valid := []struct {
		body string
		want autoSaveConfig
	}{
		{`{}`, defaultAutoSaveConfig()},
		{`{"files.autoSave":"afterDelay"}`, autoSaveConfig{"afterDelay", 1000}},
		{`{"files.autoSaveDelay":600000}`, autoSaveConfig{"off", 600000}},
		{` {"files.autoSaveDelay":100,"files.autoSave":"onWindowChange"} `, autoSaveConfig{"onWindowChange", 100}},
	}
	for _, test := range valid {
		if err := os.WriteFile(path, []byte(test.body), 0600); err != nil {
			t.Fatal(err)
		}
		if got, err := readAutoSaveConfig(path); err != nil || got != test.want {
			t.Fatal(test.body, got, err)
		}
	}
	invalid := []string{
		"", `null`, `[]`, `"off"`, `true`, `{`, `{} {}`, `{} null`, `{} trailing`,
		`{"files.autoSave":null}`, `{"files.autoSave":true}`, `{"files.autoSave":7}`,
		`{"files.autoSave":"always"}`, `{"files.autoSaveDelay":null}`, `{"files.autoSaveDelay":"1000"}`,
		`{"files.autoSaveDelay":100.5}`, `{"files.autoSaveDelay":1e3}`, `{"files.autoSaveDelay":-100}`,
		`{"files.autoSaveDelay":99}`, `{"files.autoSaveDelay":600001}`, `{"files.autoSaveDelay":999999999999999999999999999999999999999}`,
		`{"extra":true}`, strings.Repeat(" ", maxAutoSaveConfigBytes-1) + `{}`,
	}
	for _, body := range invalid {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readAutoSaveConfig(path); err == nil {
			t.Fatalf("invalid JSON accepted: %.120q", body)
		}
	}
	// Exactly 4 KiB remains accepted, including insignificant whitespace.
	if err := os.WriteFile(path, []byte(strings.Repeat(" ", maxAutoSaveConfigBytes-2)+`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readAutoSaveConfig(path); err != nil {
		t.Fatal("4 KiB boundary rejected", err)
	}
}

func TestAutoSaveConfigIdempotentAtomicPersistence(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "gocode", "autosave.json")
	for range 3 {
		for _, mode := range []string{"afterDelay", "onFocusChange", "onWindowChange", "off"} {
			config := autoSaveConfig{Mode: mode, DelayMS: 1250}
			if err := writeAutoSaveConfig(path, config); err != nil {
				t.Fatal(err)
			}
			if got, err := readAutoSaveConfig(path); err != nil || got != config {
				t.Fatal(got, err)
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil || len(entries) != 1 || entries[0].Name() != "autosave.json" {
				t.Fatal("repeated writes left temporary files", entries, err)
			}
		}
	}
	// Set an old timestamp so an accidental rewrite cannot hide in the same
	// filesystem timestamp tick; formatting is preserved for equal settings.
	body := []byte("{\n  \"files.autoSaveDelay\": 1250,\n  \"files.autoSave\": \"off\"\n}\n")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1234567890, 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeAutoSaveConfig(path, autoSaveConfig{Mode: "off", DelayMS: 1250}); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path)
	if err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("equivalent configuration rewrote mtime", err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != string(body) {
		t.Fatal("equivalent configuration rewrote formatting", err)
	}
	if err := writeAutoSaveConfig(path, autoSaveConfig{Mode: "invalid", DelayMS: 1000}); err == nil {
		t.Fatal("invalid write accepted")
	}
	actual, err = os.ReadFile(path)
	if err != nil || string(actual) != string(body) {
		t.Fatal("invalid write changed file", err)
	}
}

func TestAutoSaveConfigWriteFailureLeavesNoTemporaryFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "autosave.json")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeAutoSaveConfig(target, defaultAutoSaveConfig()); err == nil {
		t.Fatal("directory target accepted")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 1 || !entries[0].IsDir() {
		t.Fatal("failed persistence left temporary files", entries, err)
	}
	if _, err := readAutoSaveConfig(target); err == nil {
		t.Fatal("directory target read as defaults")
	}
}
