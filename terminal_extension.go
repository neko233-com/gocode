package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/neko233-com/gocode/internal/terminal"
	ui "github.com/neko233-com/godesktop"
	"github.com/neko233-com/godesktop/extensions"
)

type terminalLaunchOptions struct {
	Name         string             `json:"name,omitempty"`
	ShellPath    string             `json:"shellPath,omitempty"`
	ShellArgs    json.RawMessage    `json:"shellArgs,omitempty"`
	CWD          string             `json:"cwd,omitempty"`
	Env          map[string]*string `json:"env,omitempty"`
	StrictEnv    bool               `json:"strictEnv,omitempty"`
	HideFromUser bool               `json:"hideFromUser,omitempty"`
	Message      string             `json:"message,omitempty"`
	IsTransient  bool               `json:"isTransient,omitempty"`
	Location     *int               `json:"location,omitempty"`
}
type terminalExitStatus struct {
	Code   *int `json:"code,omitempty"`
	Reason int  `json:"reason"`
}
type nativeTerminalRecord struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	ProcessID  *int                  `json:"processId,omitempty"`
	Options    terminalLaunchOptions `json:"creationOptions"`
	Interacted bool                  `json:"isInteractedWith"`
	ExitStatus *terminalExitStatus   `json:"exitStatus,omitempty"`
}
type nativeTerminalState struct {
	CreationError string                 `json:"creationError,omitempty"`
	Generation    uint64                 `json:"generation"`
	Terminals     []nativeTerminalRecord `json:"terminals"`
	Closed        []nativeTerminalRecord `json:"closed"`
	ActiveID      string                 `json:"activeId,omitempty"`
}

func decodeTerminalOptions(raw json.RawMessage) (terminalLaunchOptions, error) {
	var options terminalLaunchOptions
	if len(bytes.TrimSpace(raw)) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return options, errors.New("terminal options must be an object")
	}
	if len(raw) > 65536 {
		return options, errors.New("terminal options exceed 64 KiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&options); err != nil {
		return options, err
	}
	valid := func(value string, limit int) bool {
		return utf8.ValidString(value) && !strings.ContainsRune(value, 0) && len(value) <= limit
	}
	if !valid(options.Name, 256) || !valid(options.ShellPath, 8192) || !valid(options.CWD, 8192) || !valid(options.Message, 8192) || len(options.Env) > 256 || (options.Location != nil && *options.Location != 1) {
		return options, errors.New("invalid native terminal options")
	}
	for key, value := range options.Env {
		if key == "" || strings.Contains(key, "=") || !valid(key, 256) || (value != nil && !valid(*value, 8192)) {
			return options, errors.New("invalid native terminal environment")
		}
	}
	if len(options.ShellArgs) > 0 {
		if options.ShellArgs[0] == '"' {
			var rawArgs string
			if runtime.GOOS != "windows" || json.Unmarshal(options.ShellArgs, &rawArgs) != nil || !valid(rawArgs, 32768) {
				return options, errors.New("string shellArgs require Windows command-line syntax")
			}
		} else {
			var args []string
			if string(options.ShellArgs) == "null" || json.Unmarshal(options.ShellArgs, &args) != nil || len(args) > 128 {
				return options, errors.New("invalid terminal shellArgs")
			}
			for _, arg := range args {
				if !valid(arg, 8192) {
					return options, errors.New("invalid terminal shell argument")
				}
			}
		}
	}
	return options, nil
}

func terminalEnvironment(base []string, changes map[string]*string, strict bool) []string {
	values := map[string]string{}
	keyOf := func(key string) string {
		if runtime.GOOS == "windows" {
			return strings.ToUpper(key)
		}
		return key
	}
	if !strict {
		for _, entry := range base {
			index := strings.IndexByte(entry, '=')
			if index == 0 {
				index = strings.IndexByte(entry[1:], '=') + 1
			}
			if index > 0 {
				values[keyOf(entry[:index])] = entry
			}
		}
	}
	for key, value := range changes {
		if value == nil {
			delete(values, keyOf(key))
		} else {
			values[keyOf(key)] = key + "=" + *value
		}
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

// Called only by a startup worker; path/profile/process work never runs on UI.
func prepareExtensionTerminal(ctx context.Context, workspace string, options terminalLaunchOptions, isolated bool) (terminal.Config, error) {
	if err := ctx.Err(); err != nil {
		return terminal.Config{}, err
	}
	cwd := options.CWD
	if cwd == "" {
		cwd = workspace
	} else if !filepath.IsAbs(cwd) {
		cwd = filepath.Join(workspace, cwd)
	}
	var err error
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return terminal.Config{}, err
	}
	if info, err := os.Stat(cwd); err != nil || !info.IsDir() {
		return terminal.Config{}, errors.New("terminal cwd must be an existing directory")
	}
	var args []string
	var rawArgs string
	if len(options.ShellArgs) > 0 {
		if options.ShellArgs[0] == '"' {
			_ = json.Unmarshal(options.ShellArgs, &rawArgs)
		} else {
			_ = json.Unmarshal(options.ShellArgs, &args)
		}
	}
	var config terminal.Config
	if options.ShellPath != "" {
		path, err := exec.LookPath(options.ShellPath)
		if err != nil {
			return config, err
		}
		config = terminal.Config{Command: []string{path}, Directory: cwd, Environment: os.Environ(), Name: filepath.Base(path)}
	} else {
		config, err = terminal.Shell(cwd, isolated)
		if err != nil {
			return config, err
		}
	}
	if len(options.ShellArgs) > 0 {
		config.Command = append(config.Command[:1:1], args...)
		if options.ShellArgs[0] == '"' {
			config.WindowsArguments = &rawArgs
		}
	}
	config.Environment = terminalEnvironment(terminal.ProcessEnvironment(config.Environment, options.StrictEnv), options.Env, options.StrictEnv)
	// Overrides are final: env null must also remove TERM/COLORTERM defaults.
	config.StrictEnvironment = true
	config.InitialMessage = options.Message
	if options.Name != "" {
		config.Name = options.Name
	}
	if len(config.Name) > 256 {
		config.Name = config.Name[:256]
		for !utf8.ValidString(config.Name) {
			config.Name = config.Name[:len(config.Name)-1]
		}
	}
	return config, nil
}

func recordTerminal(tab *terminalTab, reason int) nativeTerminalRecord {
	result := nativeTerminalRecord{ID: tab.wireID, Name: tab.name, Options: tab.options, Interacted: tab.interacted}
	if tab.session != nil {
		pid := tab.session.PID()
		result.ProcessID = &pid
	}
	if tab.frame != nil && tab.frame.Exited {
		code := tab.frame.ExitCode
		result.ExitStatus = &terminalExitStatus{Code: &code, Reason: 2}
	}
	if reason >= 0 {
		result.ExitStatus = &terminalExitStatus{Reason: reason}
		if tab.frame != nil && tab.frame.Exited {
			code := tab.frame.ExitCode
			result.ExitStatus.Code = &code
		}
	}
	return result
}
func (m *model) terminalState() nativeTerminalState {
	state := nativeTerminalState{Terminals: make([]nativeTerminalRecord, 0, len(m.terminals)), Closed: append([]nativeTerminalRecord{}, m.closedTerminals...)}
	for _, tab := range m.terminals {
		state.Terminals = append(state.Terminals, recordTerminal(tab, -1))
	}
	if tab := m.currentTerminal(); tab != nil {
		state.ActiveID = tab.wireID
	}
	previous := m.lastTerminalState
	previous.Generation = 0
	if !reflect.DeepEqual(state, previous) {
		m.terminalGeneration++
	}
	state.Generation = m.terminalGeneration
	m.lastTerminalState = state
	return state
}
func (m *model) terminalChanged() {
	if m.publishTerminals != nil {
		m.publishTerminals()
	}
}
func (m *model) extensionTerminal(id string) *terminalTab {
	for _, tab := range m.terminals {
		if tab.wireID == id {
			return tab
		}
	}
	return nil
}

func (m *model) registerTerminalRPC(cx *ui.Context, host *extensions.Host) {
	host.Register("window/createTerminal", func(ctx context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			ID      string          `json:"id"`
			Options json.RawMessage `json:"options"`
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		if len(request.ID) > 96 || !strings.HasPrefix(request.ID, "extension:") || !utf8.ValidString(request.ID) || strings.ContainsRune(request.ID, 0) {
			return nil, errors.New("invalid extension terminal identity")
		}
		options, err := decodeTerminalOptions(request.Options)
		if err != nil {
			return nil, err
		}
		type answer struct {
			value any
			err   error
		}
		reply := make(chan answer, 1)
		if !cx.Dispatch(func() {
			if err := ctx.Err(); err != nil {
				reply <- answer{err: err}
				return
			}
			if m.requestTerminal == nil {
				reply <- answer{err: errors.New("native terminal workers unavailable")}
				return
			}
			m.requestTerminal(ctx, request.ID, options, func(err error) {
				state := m.terminalState()
				if err != nil {
					state.CreationError = err.Error()
				}
				reply <- answer{value: state}
			})
		}) {
			return nil, errors.New("native workbench closed")
		}
		select {
		case result := <-reply:
			return result.value, result.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	host.Register("window/terminalAction", onUI(cx, func(_ context.Context, raw json.RawMessage) (any, error) {
		var request struct {
			ID, Kind, Text               string
			ShouldExecute, PreserveFocus bool
		}
		if err := json.Unmarshal(raw, &request); err != nil {
			return nil, err
		}
		tab := m.extensionTerminal(request.ID)
		if tab == nil {
			if request.Kind == "dispose" {
				return m.terminalState(), nil
			}
			return nil, errors.New("terminal has closed")
		}
		switch request.Kind {
		case "show":
			for index, item := range m.terminals {
				if item == tab {
					m.activeTerminal = index
				}
			}
			tab.hidden = false
			m.panel, m.showPanel = "TERMINAL", true
			if !request.PreserveFocus {
				m.focusTerminal()
			}
		case "hide":
			if m.currentTerminal() == tab {
				m.hidePanel()
			}
		case "dispose":
			m.closeTerminal(tab, 4)
		case "sendText":
			if tab.session == nil || (tab.frame != nil && tab.frame.Exited) {
				return nil, errors.New("terminal process is unavailable")
			}
			if !utf8.ValidString(request.Text) || len(request.Text) > terminal.MaxInputBytes-2 {
				return nil, errors.New("terminal input exceeds 64 KiB")
			}
			text := request.Text
			if request.ShouldExecute {
				text += "\r"
			}
			if err := tab.session.SendText(text); err != nil {
				return nil, err
			}
			if text != "" {
				tab.interacted = true
			}
		default:
			return nil, fmt.Errorf("unknown terminal operation %q", request.Kind)
		}
		m.terminalChanged()
		return m.terminalState(), nil
	}))
}
