package localrelease

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/neko233-com/gocode/internal/nativeguard"
	"github.com/neko233-com/gocode/internal/update"
)

type Launch struct {
	FullPath       string `json:"fullPath"`
	ArchiveEntry   string `json:"archiveEntry"`
	Bytes          int64  `json:"bytes"`
	SHA256         string `json:"sha256"`
	UnchangedAfter bool   `json:"unchangedAfter"`
}

type GateResult struct {
	Launch Launch             `json:"launch"`
	Report nativeguard.Report `json:"report"`
}

func GateArguments(spec GateSpec, stageRoot string) []string {
	args := slices.Clone(spec.Args)
	if spec.ID == "gui-version" {
		return args
	}
	if spec.ID != "go-lsp" && spec.ID != "filewatch" && spec.ID != "typescript-lsp" && spec.ID != "installed-go-service" && spec.ID != "installed-typescript-service" {
		args = append(args, "-lsp=false")
	}
	return append(args, "-copilot=false", "-extensions-dir", filepath.Join(stageRoot, "acceptance-extensions"))
}

// RunGate binds the exact executable bytes before and after the existing owned
// native supervisor runs. It performs no UI-thread work and cannot accept an
// old same-named program, early-exit auxiliary flag, or changed executable.
func RunGate(ctx context.Context, stageRoot, version, source string, spec GateSpec, environment []string) (nativeguard.Result, GateResult, error) {
	var bound GateResult
	if len(spec.Args) == 0 {
		return nativeguard.Result{}, bound, errors.New("real required gate command remains unwired")
	}
	name := "gocode-app.exe"
	if spec.GUI {
		name = "gocode-app-gui.exe"
	}
	entry := "versions/" + version + "/" + name
	program, err := Within(stageRoot, filepath.FromSlash(entry))
	if err != nil {
		return nativeguard.Result{}, bound, err
	}
	before, err := HashFile(ctx, program, update.MaxArchiveBytes)
	if err != nil {
		return nativeguard.Result{}, bound, err
	}
	result, runErr := nativeguard.Run(ctx, nativeguard.Options{Command: append([]string{program}, GateArguments(spec, stageRoot)...), Directory: stageRoot, Environment: environment, Timeout: spec.Timeout})
	after, hashErr := HashFile(ctx, program, update.MaxArchiveBytes)
	bound = GateResult{Launch: Launch{FullPath: program, ArchiveEntry: entry, Bytes: before.Bytes, SHA256: before.SHA256, UnchangedAfter: hashErr == nil && reflect.DeepEqual(before, after)}, Report: result.Report}
	if hashErr != nil || !bound.Launch.UnchangedAfter || result.Report.Executable != program {
		return result, bound, errors.Join(runErr, hashErr, errors.New("actual launched staged executable path/bytes changed"))
	}
	if runErr == nil {
		runErr = validateGateSuccess(spec, version, source, result.Stdout)
	}
	return result, bound, runErr
}
