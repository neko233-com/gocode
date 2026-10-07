package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxAutoSaveConfigBytes = 4096

type autoSaveConfig struct {
	Mode    string `json:"files.autoSave"`
	DelayMS int    `json:"files.autoSaveDelay"`
}

func defaultAutoSaveConfig() autoSaveConfig {
	return autoSaveConfig{Mode: "off", DelayMS: 1000}
}

func validateAutoSaveConfig(config autoSaveConfig) error {
	switch config.Mode {
	case "off", "afterDelay", "onFocusChange", "onWindowChange":
	default:
		return fmt.Errorf("unknown Auto Save mode %q", config.Mode)
	}
	if config.DelayMS < 100 || config.DelayMS > 600000 {
		return errors.New("Auto Save delay must be between 100 and 600000 milliseconds")
	}
	return nil
}

func autoSaveConfigPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "gocode", "autosave.json"), nil
}

func decodeAutoSaveConfig(body []byte) (autoSaveConfig, error) {
	config := defaultAutoSaveConfig()
	if len(body) > maxAutoSaveConfigBytes {
		return config, errors.New("Auto Save settings exceed 4 KiB")
	}
	// Raw fields distinguish absent properties (defaults) from explicit null,
	// which JSON otherwise silently accepts for a Go string or integer.
	var fields struct {
		Mode    json.RawMessage `json:"files.autoSave"`
		DelayMS json.RawMessage `json:"files.autoSaveDelay"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fields); err != nil {
		return config, err
	}
	if bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		return config, errors.New("Auto Save settings must be a JSON object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			err = errors.New("Auto Save settings contain more than one JSON document")
		}
		return config, err
	}
	if len(fields.Mode) != 0 {
		if bytes.Equal(fields.Mode, []byte("null")) {
			return config, errors.New("Auto Save mode must be a string")
		}
		if err := json.Unmarshal(fields.Mode, &config.Mode); err != nil {
			return config, err
		}
	}
	if len(fields.DelayMS) != 0 {
		if bytes.Equal(fields.DelayMS, []byte("null")) {
			return config, errors.New("Auto Save delay must be an integer")
		}
		if err := json.Unmarshal(fields.DelayMS, &config.DelayMS); err != nil {
			return config, err
		}
	}
	return config, validateAutoSaveConfig(config)
}

func readAutoSaveConfig(path string) (autoSaveConfig, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultAutoSaveConfig(), nil
	}
	if err != nil {
		return defaultAutoSaveConfig(), err
	}
	defer f.Close()
	body, err := io.ReadAll(io.LimitReader(f, maxAutoSaveConfigBytes+1))
	if err != nil {
		return defaultAutoSaveConfig(), err
	}
	return decodeAutoSaveConfig(body)
}

func writeAutoSaveConfig(path string, config autoSaveConfig) error {
	if err := validateAutoSaveConfig(config); err != nil {
		return err
	}
	// Configuration equality is semantic: preserve hand formatting and mtime
	// when the existing object already describes the requested mode and delay.
	if f, err := os.Open(path); err == nil {
		body, readErr := io.ReadAll(io.LimitReader(f, maxAutoSaveConfigBytes+1))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil {
			return errors.Join(readErr, closeErr)
		}
		if current, parseErr := decodeAutoSaveConfig(body); parseErr == nil && current == config {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	body, err := json.Marshal(config)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".autosave-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(body); err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
