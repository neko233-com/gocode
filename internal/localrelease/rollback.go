package localrelease

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/neko233-com/gocode/internal/installlayout"
	"github.com/neko233-com/gocode/internal/nativeguard"
	"github.com/neko233-com/gocode/internal/update"
)

type Rollback struct {
	Version  string                 `json:"version"`
	Source   string                 `json:"source"`
	Archive  File                   `json:"archive"`
	Envelope File                   `json:"envelope"`
	Before   installlayout.Manifest `json:"before"`
	After    installlayout.Manifest `json:"after"`
	Native   Gate                   `json:"native"`
}

func priorGate(stageRoot string) GateSpec {
	return GateSpec{ID: "prior-rollback", Args: []string{"-smoke", "-workspace", filepath.Join(stageRoot, "rollback-workspace")}, Timeout: 60 * time.Second}
}

// RunRollback stages both genuine signed versions in the same owned root,
// uses the production rollback selection, then starts the real prior editor
// with the same extension root. The caller retains the actual owned-process
// report and captured outputs; no public update discovery is involved.
func RunRollback(ctx context.Context, stageRoot, archive, envelopePath string, current Staged, environment []string) (Rollback, nativeguard.Result, error) {
	var result Rollback
	key, err := update.PublisherKey()
	if err != nil {
		return result, nativeguard.Result{}, err
	}
	envelope, err := readBounded(envelopePath, update.MaxManifestBytes)
	if err != nil {
		return result, nativeguard.Result{}, err
	}
	prior, err := update.Verify(envelope, key)
	if err != nil {
		return result, nativeguard.Result{}, err
	}
	if !ValidSource(prior.Source) || prior.Source == current.Source || installlayout.CompareVersion(prior.Version, current.Version) >= 0 {
		return result, nativeguard.Result{}, errors.New("rollback requires a genuinely older signed immutable release")
	}
	asset, err := prior.Asset("windows/amd64")
	if err != nil {
		return result, nativeguard.Result{}, err
	}
	result.Archive, err = HashFile(ctx, archive, update.MaxArchiveBytes)
	if err != nil || filepath.Base(archive) != asset.Name || result.Archive.Bytes != asset.Size || result.Archive.SHA256 != asset.SHA256 {
		return result, nativeguard.Result{}, errors.New("prior signed archive bytes differ")
	}
	result.Envelope, err = HashFile(ctx, envelopePath, update.MaxManifestBytes)
	if err != nil {
		return result, nativeguard.Result{}, err
	}
	manifest, err := update.Stage(ctx, stageRoot, archive, envelope, key, "windows/amd64")
	if err != nil {
		return result, nativeguard.Result{}, err
	}
	if err := update.Probe(ctx, stageRoot, manifest); err != nil {
		return result, nativeguard.Result{}, err
	}
	if err := ProbeStaged(ctx, stageRoot, current); err != nil {
		return result, nativeguard.Result{}, err
	}
	if err := installlayout.Activate(stageRoot, manifest); err != nil {
		return result, nativeguard.Result{}, err
	}
	newManifest := installlayout.Manifest{Owner: installlayout.Owner, Schema: 1, Version: current.Version, Source: current.Source}
	if err := installlayout.Activate(stageRoot, newManifest); err != nil {
		return result, nativeguard.Result{}, err
	}
	_, result.Before, err = installlayout.Resolve(stageRoot, false)
	if err != nil || result.Before != newManifest {
		return result, nativeguard.Result{}, errors.New("new selection is not the actual staged release")
	}
	if err := update.Rollback(ctx, stageRoot, key); err != nil {
		return result, nativeguard.Result{}, err
	}
	_, result.After, err = installlayout.Resolve(stageRoot, false)
	if err != nil || result.After != manifest {
		return result, nativeguard.Result{}, errors.New("actual rollback did not select the signed prior source")
	}
	workspace := filepath.Join(stageRoot, "rollback-workspace")
	if err := os.Mkdir(workspace, 0700); err != nil {
		return result, nativeguard.Result{}, err
	}
	if err := os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n"), 0600); err != nil {
		return result, nativeguard.Result{}, err
	}
	captured, bound, runErr := RunGate(ctx, stageRoot, prior.Version, prior.Source, priorGate(stageRoot), environment)
	result.Version, result.Source = prior.Version, prior.Source
	result.Native = Gate{ID: "prior-rollback", Program: File{Path: bound.Launch.ArchiveEntry, Bytes: bound.Launch.Bytes, SHA256: bound.Launch.SHA256}, Result: bound.Report, Launch: bound.Launch}
	return result, captured, runErr
}

func verifyRollback(ctx context.Context, root, version, source string, prior *Rollback) error {
	if prior == nil || !ValidSource(prior.Source) || prior.Source == source || !installlayout.ValidVersion(prior.Version) || installlayout.CompareVersion(prior.Version, version) >= 0 {
		return errors.New("genuine prior-release rollback evidence is required")
	}
	for _, input := range []struct {
		file  File
		limit int64
	}{{prior.Archive, update.MaxArchiveBytes}, {prior.Envelope, update.MaxManifestBytes}} {
		if err := VerifyFile(ctx, root, input.file, input.limit); err != nil {
			return err
		}
	}
	envelopePath, _ := Within(root, prior.Envelope.Path)
	data, err := readBounded(envelopePath, update.MaxManifestBytes)
	if err != nil {
		return err
	}
	key, err := update.PublisherKey()
	if err != nil {
		return err
	}
	manifest, err := update.Verify(data, key)
	if err != nil || manifest.Version != prior.Version || manifest.Source != prior.Source {
		return errors.New("prior production signature/source differs")
	}
	asset, err := manifest.Asset("windows/amd64")
	if err != nil || asset.Name != filepath.Base(prior.Archive.Path) || asset.Size != prior.Archive.Bytes || asset.SHA256 != prior.Archive.SHA256 {
		return errors.New("prior signed archive differs")
	}
	wantedBefore := installlayout.Manifest{Owner: installlayout.Owner, Schema: 1, Version: version, Source: source}
	wantedAfter := installlayout.Manifest{Owner: installlayout.Owner, Schema: 1, Version: prior.Version, Source: prior.Source}
	if prior.Before != wantedBefore || prior.After != wantedAfter {
		return errors.New("actual old/new rollback selection differs")
	}
	gate := prior.Native
	stageRoot, _ := Within(root, "private-stage")
	spec := priorGate(stageRoot)
	entry := "versions/" + prior.Version + "/gocode-app.exe"
	expected, _ := Within(stageRoot, filepath.FromSlash(entry))
	if gate.ID != spec.ID || gate.Program.Path != entry || gate.Result.Executable != expected || gate.Launch.FullPath != expected || !slices.Equal(gate.Result.Arguments, GateArguments(spec, stageRoot)) || gate.Result.PID <= 0 || gate.Result.ExitCode != 0 || gate.Result.Error != "" || gate.Result.TimedOut || gate.Result.OutputLimit || !gate.Result.RootReaped || !gate.Result.TreeClosed || gate.Result.DeadlineMS <= 0 || gate.Result.DeadlineMS > spec.Timeout.Milliseconds() || gate.Result.ElapsedMS < 0 || !gate.Launch.UnchangedAfter || gate.Launch.ArchiveEntry != entry || gate.Launch.Bytes != gate.Program.Bytes || gate.Launch.SHA256 != gate.Program.SHA256 {
		return errors.New("real prior native rollback process/path/bytes differs")
	}
	archivePath, _ := Within(root, prior.Archive.Path)
	programs, err := archivePrograms(ctx, archivePath, prior.Version)
	if err != nil || programs[entry] != gate.Program {
		return errors.New("prior native executable does not belong to signed archive")
	}
	if err := verifyGateFiles(ctx, root, gate); err != nil {
		return err
	}
	// The previous editor must actually render and execute its installed VSIX;
	// its -version health check alone is insufficient.
	outputPath, _ := Within(root, gate.Stdout.Path)
	output, err := readBounded(outputPath, nativeguard.MaxOutputBytes)
	if err != nil || !strings.Contains(string(output), "gocode smoke passed: native rendering + installed VSIX activation + command execution") {
		return errors.New("prior native/VSIX startup success is absent")
	}
	return nil
}

func verifyGateFiles(ctx context.Context, root string, gate Gate) error {
	if err := VerifyFile(ctx, root, gate.Report, MaxReceiptBytes); err != nil {
		return err
	}
	for _, output := range []File{gate.Stdout, gate.Stderr} {
		if err := VerifyFile(ctx, root, output, nativeguard.MaxOutputBytes); err != nil {
			return err
		}
	}
	path, _ := Within(root, gate.Report.Path)
	var actual GateResult
	if err := decodeBounded(path, &actual); err != nil || !reflect.DeepEqual(actual.Report, gate.Result) || !reflect.DeepEqual(actual.Launch, gate.Launch) {
		return errors.New("actual owned-process receipt differs")
	}
	if int64(actual.Report.StdoutBytes) != gate.Stdout.Bytes || actual.Report.StdoutSHA256 != gate.Stdout.SHA256 || int64(actual.Report.StderrBytes) != gate.Stderr.Bytes || actual.Report.StderrSHA256 != gate.Stderr.SHA256 {
		return errors.New("captured process outputs differ from owned report")
	}
	return nil
}
