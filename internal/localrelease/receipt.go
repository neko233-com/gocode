package localrelease

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/neko233-com/gocode/internal/installlayout"
	"github.com/neko233-com/gocode/internal/nativeguard"
	"github.com/neko233-com/gocode/internal/update"
)

type Check struct {
	ID       string `json:"id"`
	ExitCode int    `json:"exitCode"`
	Evidence File   `json:"evidence"`
}

type CheckEvidence struct {
	Schema        int       `json:"schema"`
	ID            string    `json:"id"`
	Version       string    `json:"version"`
	Source        string    `json:"source"`
	Command       []string  `json:"command"`
	PID           int       `json:"pid"`
	ExitCode      int       `json:"exitCode"`
	ElapsedMS     int64     `json:"elapsedMS"`
	RootReaped    bool      `json:"rootReaped"`
	TreeClosed    bool      `json:"treeClosed"`
	InputDigest   string    `json:"inputDigest"`
	Stdout        File      `json:"stdout"`
	Stderr        File      `json:"stderr"`
	Rollback      *Rollback `json:"rollback,omitempty"`
	PackageInputs []File    `json:"packageInputs,omitempty"`
}

type Gate struct {
	ID      string             `json:"id"`
	Program File               `json:"program"`
	Report  File               `json:"reportFile"`
	Stdout  File               `json:"stdoutFile"`
	Stderr  File               `json:"stderrFile"`
	Result  nativeguard.Report `json:"result"`
	Launch  Launch             `json:"launch"`
}

type Receipt struct {
	Schema              int       `json:"schema"`
	Version             string    `json:"version"`
	Source              string    `json:"source"`
	Platforms           []string  `json:"platforms"`
	Artifacts           []File    `json:"artifacts"`
	Envelope            File      `json:"envelope"`
	Framework           Framework `json:"framework"`
	Checks              []Check   `json:"checks"`
	Gates               []Gate    `json:"gates"`
	PrivateTMPAbsent    bool      `json:"privateTMPAbsent"`
	PrivateConfigAbsent bool      `json:"privateConfigAbsent"`
	StageAbsent         bool      `json:"stageAbsent"`
	Complete            bool      `json:"complete"`
	InputDigest         string    `json:"inputDigest"`
}

func ReadReceipt(path string) (Receipt, error) {
	var result Receipt
	err := decodeBounded(path, &result)
	return result, err
}

func (r Receipt) validateShape(version, source string) error {
	if r.Schema != 1 || !r.Complete || r.Version != version || !installlayout.ValidVersion(version) || r.Source != source || !ValidSource(source) || !sha256Text(r.InputDigest) {
		return errors.New("local release receipt is incomplete or belongs to another immutable source/version")
	}
	if !slices.Equal(r.Platforms, []string{"windows/amd64"}) {
		return errors.New("local native publication currently requires an honest Windows-only receipt")
	}
	if !r.PrivateTMPAbsent || !r.PrivateConfigAbsent || !r.StageAbsent {
		return errors.New("local release receipt does not prove private cleanup")
	}
	if len(r.Artifacts) != 2 {
		return errors.New("Windows-only receipt requires exactly its ZIP and MSI")
	}
	wanted := []string{"gocode-" + version + "-windows-amd64.msi", "gocode-" + version + "-windows-amd64.zip"}
	for i, item := range r.Artifacts {
		if filepath.Base(item.Path) != wanted[i] || item.Bytes <= 0 || !sha256Text(item.SHA256) {
			return errors.New("local release artifact identity differs")
		}
	}
	plan := WindowsPlan()
	if len(r.Gates) != len(plan) {
		return errors.New("all exact ordered native/service/rollback gates are required")
	}
	for i, spec := range plan {
		gate := r.Gates[i]
		if gate.ID != spec.ID || gate.Result.PID <= 0 || gate.Result.ExitCode != 0 || gate.Result.Error != "" || gate.Result.TimedOut || gate.Result.OutputLimit || !gate.Result.RootReaped || !gate.Result.TreeClosed {
			return fmt.Errorf("native gate failed, incomplete or reordered: %s", spec.ID)
		}
		if gate.Result.DeadlineMS <= 0 || gate.Result.DeadlineMS > spec.Timeout.Milliseconds() || gate.Result.ElapsedMS < 0 {
			return fmt.Errorf("native gate deadline differs: %s", spec.ID)
		}
		if len(spec.Args) == 0 {
			return fmt.Errorf("real service/installed-extension/rollback command remains unwired: %s", spec.ID)
		}
		if !slices.Equal(gate.Result.Arguments, GateArguments(spec, filepath.Dir(filepath.Dir(filepath.Dir(gate.Launch.FullPath))))) {
			return fmt.Errorf("native gate command differs: %s", spec.ID)
		}
		program := "gocode-app.exe"
		if spec.GUI {
			program = "gocode-app-gui.exe"
		}
		if gate.Program.Path != "versions/"+version+"/"+program {
			return fmt.Errorf("native console/GUI program selection differs: %s", spec.ID)
		}
		if !gate.Launch.UnchangedAfter || gate.Launch.ArchiveEntry != gate.Program.Path || gate.Launch.Bytes != gate.Program.Bytes || gate.Launch.SHA256 != gate.Program.SHA256 || gate.Result.Executable != gate.Launch.FullPath {
			return fmt.Errorf("native launch bytes/path are not bound: %s", spec.ID)
		}
	}
	checks := RequiredChecks()
	if len(r.Checks) != len(checks) {
		return errors.New("all actual local source/MSI checks are required")
	}
	for i, id := range checks {
		if r.Checks[i].ID != id || r.Checks[i].ExitCode != 0 {
			return fmt.Errorf("missing, failed or reordered local source check: %s", id)
		}
	}
	return nil
}

func validateCheckEvidence(evidence CheckEvidence, id, version, source, inputs string) error {
	if evidence.Schema != 1 || evidence.ID != id || evidence.Version != version || evidence.Source != source || evidence.PID <= 0 || evidence.ExitCode != 0 || evidence.ElapsedMS < 0 || !evidence.RootReaped || !evidence.TreeClosed || len(evidence.Command) == 0 || len(evidence.Command) > 64 || evidence.InputDigest != inputs || !sha256Text(inputs) {
		return fmt.Errorf("actual source/MSI check evidence differs: %s", id)
	}
	for _, arg := range evidence.Command {
		if len(arg) > 4096 || strings.ContainsRune(arg, '\x00') {
			return errors.New("source check command exceeds bound")
		}
	}
	if id == "real-prior-release-rollback" && evidence.Rollback == nil {
		return errors.New("actual prior-release rollback report is required")
	}
	if id != "real-prior-release-rollback" && evidence.Rollback != nil {
		return errors.New("prior-release report is attached to an unrelated check")
	}
	return nil
}

// VerifyReceipt recomputes real package/log bytes and actual module Origin.
// It cannot accept a count-only marker, an old source or claimed framework sum.
func VerifyReceipt(ctx context.Context, root, project, receiptPath, version, source string) (Receipt, error) {
	r, err := ReadReceipt(receiptPath)
	if err != nil {
		return r, err
	}
	if err := r.validateShape(version, source); err != nil {
		return r, err
	}
	for _, relative := range []string{"private-stage", "private-tmp", "private-config"} {
		path, err := Within(root, relative)
		if err != nil {
			return r, err
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return r, errors.New("actual private stage/TMP/config still exists")
		}
	}
	actualFramework, err := CheckFramework(ctx, project, r.Framework.Version, r.Framework.Source)
	if err != nil || !reflect.DeepEqual(actualFramework, r.Framework) {
		return r, errors.Join(err, errors.New("actual published framework identity differs from tested receipt"))
	}
	actualInputs, err := InputDigest(ctx, project)
	if err != nil || actualInputs != r.InputDigest {
		return r, errors.New("actual tracked source input bytes differ from tested release")
	}
	for _, artifact := range r.Artifacts {
		if err := VerifyFile(ctx, root, artifact, update.MaxArchiveBytes); err != nil {
			return r, err
		}
	}
	if err := VerifyFile(ctx, root, r.Envelope, update.MaxManifestBytes); err != nil {
		return r, err
	}
	envelopePath, _ := Within(root, r.Envelope.Path)
	envelope, err := readBounded(envelopePath, update.MaxManifestBytes)
	if err != nil {
		return r, err
	}
	key, err := update.PublisherKey()
	if err != nil {
		return r, err
	}
	manifest, err := update.Verify(envelope, key)
	if err != nil || manifest.Version != version || manifest.Source != source || len(manifest.Assets) != 1 {
		return r, errors.New("production signed receipt source/platform differs")
	}
	asset, err := manifest.Asset("windows/amd64")
	if err != nil || asset.Name != filepath.Base(r.Artifacts[1].Path) || asset.Size != r.Artifacts[1].Bytes || asset.SHA256 != r.Artifacts[1].SHA256 {
		return r, errors.New("production signed receipt archive differs")
	}
	for _, check := range r.Checks {
		if err := VerifyFile(ctx, root, check.Evidence, MaxReceiptBytes); err != nil {
			return r, err
		}
		path, _ := Within(root, check.Evidence.Path)
		var evidence CheckEvidence
		if err := decodeBounded(path, &evidence); err != nil {
			return r, fmt.Errorf("actual source/MSI check evidence differs: %s", check.ID)
		}
		if err := validateCheckEvidence(evidence, check.ID, version, source, actualInputs); err != nil {
			return r, err
		}
		for _, output := range []File{evidence.Stdout, evidence.Stderr} {
			if err := VerifyFile(ctx, root, output, 4<<20); err != nil {
				return r, err
			}
		}
		if check.ID == "msi-install-upgrade-rollback-uninstall" {
			if err := validatePackageInputs(evidence.PackageInputs, r.Artifacts); err != nil {
				return r, err
			}
			for _, input := range evidence.PackageInputs {
				if err := VerifyFile(ctx, root, input, update.MaxArchiveBytes); err != nil {
					return r, err
				}
			}
		} else if len(evidence.PackageInputs) != 0 {
			return r, errors.New("package input evidence belongs only to the actual MSI check")
		}
		if check.ID == "real-prior-release-rollback" {
			if err := verifyRollback(ctx, root, version, source, evidence.Rollback); err != nil {
				return r, err
			}
		} else if evidence.Rollback != nil {
			return r, errors.New("prior rollback evidence belongs only to its required check")
		}
	}
	archivePath, _ := Within(root, r.Artifacts[1].Path)
	programs, err := archivePrograms(ctx, archivePath, version)
	if err != nil {
		return r, err
	}
	for _, gate := range r.Gates {
		if err := VerifyFile(ctx, root, gate.Report, MaxReceiptBytes); err != nil {
			return r, err
		}
		for _, output := range []File{gate.Stdout, gate.Stderr} {
			if err := VerifyFile(ctx, root, output, nativeguard.MaxOutputBytes); err != nil {
				return r, err
			}
		}
		reportPath, _ := Within(root, gate.Report.Path)
		var actualBound GateResult
		if err := decodeBounded(reportPath, &actualBound); err != nil || !reflect.DeepEqual(actualBound.Report, gate.Result) || !reflect.DeepEqual(actualBound.Launch, gate.Launch) {
			return r, errors.New("actual owned process report differs from receipt")
		}
		actualReport := actualBound.Report
		if int64(actualReport.StdoutBytes) != gate.Stdout.Bytes || actualReport.StdoutSHA256 != gate.Stdout.SHA256 || int64(actualReport.StderrBytes) != gate.Stderr.Bytes || actualReport.StderrSHA256 != gate.Stderr.SHA256 {
			return r, errors.New("actual captured process output differs from report")
		}
		program, ok := programs[gate.Program.Path]
		stagedExpected, err := Within(root, filepath.Join("private-stage", filepath.FromSlash(gate.Program.Path)))
		if err != nil || !ok || program != gate.Program || actualReport.Executable != stagedExpected || gate.Launch.FullPath != stagedExpected {
			return r, errors.New("native gate executable differs from the signed tested archive")
		}
		outputPath, _ := Within(root, gate.Stdout.Path)
		output, err := readBounded(outputPath, nativeguard.MaxOutputBytes)
		if err != nil {
			return r, err
		}
		for _, spec := range WindowsPlan() {
			if spec.ID == gate.ID {
				if err := validateGateSuccess(spec, version, source, output); err != nil {
					return r, err
				}
				break
			}
		}
	}
	return r, nil
}

func archivePrograms(ctx context.Context, path, version string) (map[string]File, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer archive.Close()
	wanted := map[string]File{}
	for _, entry := range archive.File {
		if entry.Name != "versions/"+version+"/gocode-app.exe" && entry.Name != "versions/"+version+"/gocode-app-gui.exe" {
			continue
		}
		if _, duplicate := wanted[entry.Name]; duplicate || entry.UncompressedSize64 > update.MaxArchiveBytes {
			return nil, errors.New("invalid or duplicated archive program")
		}
		stream, err := entry.Open()
		if err != nil {
			return nil, err
		}
		hash := sha256.New()
		count, err := io.Copy(hash, io.LimitReader(stream, update.MaxArchiveBytes+1))
		closeErr := stream.Close()
		if err != nil || closeErr != nil || ctx.Err() != nil || count > update.MaxArchiveBytes {
			return nil, errors.Join(err, closeErr, ctx.Err(), errors.New("archive program read failed"))
		}
		wanted[entry.Name] = File{Path: entry.Name, Bytes: count, SHA256: hex.EncodeToString(hash.Sum(nil))}
	}
	if len(wanted) != 2 {
		return nil, errors.New("signed Windows archive requires both console and GUI programs")
	}
	return wanted, nil
}

func validatePackageInputs(actual, expected []File) error {
	if len(actual) != 2 || !reflect.DeepEqual(actual, expected) {
		return errors.New("MSI check is not bound to the actual final MSI/ZIP inputs")
	}
	return nil
}

func WindowsArtifactNames(version string) []string {
	return []string{"gocode-" + version + "-windows-amd64.msi", "gocode-" + version + "-windows-amd64.zip"}
}

func PendingCommands() []string {
	var ids []string
	for _, spec := range WindowsPlan() {
		if len(spec.Args) == 0 {
			ids = append(ids, spec.ID)
		}
	}
	return ids
}

func ValidSource(value string) bool {
	_, err := hex.DecodeString(value)
	return err == nil && len(value) == 40 && value == strings.ToLower(value)
}
