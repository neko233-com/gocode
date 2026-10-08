package localrelease

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/neko233-com/gocode/internal/languageextension"
)

const rollbackCompatibilityMode = "owned-exact-native-adapters-disabled"

// Compatibility records a deliberate change only to the disposable acceptance
// root. It does not claim the previous executable can activate native adapters.
// Retained states and complete pinned inventories survive private-stage removal.
type RollbackCompatibility struct {
	Schema              int                         `json:"schema"`
	Mode                string                      `json:"mode"`
	ExtensionRoot       string                      `json:"extensionRoot"`
	TargetVersion       string                      `json:"targetVersion"`
	MinimumVersion      string                      `json:"minimumVersion"`
	DisabledIDs         []string                    `json:"disabledIDs"`
	OriginalStateAbsent bool                        `json:"originalStateAbsent"`
	OriginalState       File                        `json:"originalState"`
	AppliedState        File                        `json:"appliedState"`
	FinalState          File                        `json:"finalState"`
	PackagesBefore      []languageextension.Receipt `json:"packagesBefore"`
	PackagesAfter       []languageextension.Receipt `json:"packagesAfter"`
}

func rollbackDisabledState(before languageextension.State) languageextension.State {
	after := languageextension.State{Disabled: slices.Clone(before.Disabled), Uninstall: slices.Clone(before.Uninstall)}
	for _, kind := range []string{"go", "typescript"} {
		pin, _ := languageextension.Package(kind)
		if !slices.ContainsFunc(after.Disabled, func(id string) bool { return strings.EqualFold(id, pin.ID) }) {
			after.Disabled = append(after.Disabled, pin.ID)
		}
	}
	return after
}

func observeRollbackPackages(extensionRoot string) ([]languageextension.Receipt, error) {
	var packages []languageextension.Receipt
	for _, kind := range []string{"go", "typescript"} {
		receipt, err := languageextension.VerifyInstalled(extensionRoot, kind)
		if err != nil {
			return nil, err
		}
		packages = append(packages, receipt)
	}
	return packages, nil
}

func retainRollbackState(ctx context.Context, stageRoot, name string, data []byte) (File, error) {
	relative := "rollback-compatibility/" + name + ".json"
	path, err := Within(stageRoot, relative)
	if err != nil {
		return File{}, err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return File{}, err
	}
	file, err := HashFile(ctx, path, languageextension.MaxStateBytes)
	file.Path = relative
	return file, err
}

func prepareRollbackCompatibility(ctx context.Context, stageRoot, target string) (RollbackCompatibility, error) {
	proof := RollbackCompatibility{Schema: 1, Mode: rollbackCompatibilityMode, ExtensionRoot: filepath.Join(stageRoot, "acceptance-extensions"), TargetVersion: target, MinimumVersion: languageextension.NativeAdapterMinimumVersion, DisabledIDs: []string{"golang.go", "vscode.typescript-language-features"}}
	before, err := languageextension.ReadState(proof.ExtensionRoot)
	if err != nil {
		return proof, err
	}
	proof.PackagesBefore, err = observeRollbackPackages(proof.ExtensionRoot)
	if err != nil {
		return proof, err
	}
	data, err := readBounded(filepath.Join(proof.ExtensionRoot, ".gocode-state.json"), languageextension.MaxStateBytes)
	if errors.Is(err, os.ErrNotExist) {
		proof.OriginalStateAbsent = true
		data, err = json.Marshal(before)
	}
	if err != nil {
		return proof, err
	}
	// Fresh fixed evidence prevents reusing an earlier acceptance observation.
	if err := os.Mkdir(filepath.Join(stageRoot, "rollback-compatibility"), 0700); err != nil {
		return proof, err
	}
	proof.OriginalState, err = retainRollbackState(ctx, stageRoot, "original-state", data)
	if err != nil {
		return proof, err
	}
	after := rollbackDisabledState(before)
	if len(after.Disabled) > 512 {
		return proof, errors.New("rollback acceptance extension state exceeds limit")
	}
	data, err = json.MarshalIndent(after, "", "  ")
	if err != nil || len(data)+1 > languageextension.MaxStateBytes {
		return proof, errors.Join(err, errors.New("rollback acceptance state exceeds byte limit"))
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(proof.ExtensionRoot, ".gocode-state.json"), data, 0600); err != nil {
		return proof, err
	}
	// Read the actual persisted state, rather than recording the desired value.
	actual, err := languageextension.ReadState(proof.ExtensionRoot)
	if err != nil || !reflect.DeepEqual(actual, after) {
		return proof, errors.Join(err, errors.New("persisted rollback acceptance state differs"))
	}
	data, err = readBounded(filepath.Join(proof.ExtensionRoot, ".gocode-state.json"), languageextension.MaxStateBytes)
	if err != nil {
		return proof, err
	}
	proof.AppliedState, err = retainRollbackState(ctx, stageRoot, "applied-state", data)
	return proof, err
}

func finishRollbackCompatibility(ctx context.Context, stageRoot string, proof *RollbackCompatibility) error {
	data, err := readBounded(filepath.Join(proof.ExtensionRoot, ".gocode-state.json"), languageextension.MaxStateBytes)
	if err != nil {
		return err
	}
	proof.FinalState, err = retainRollbackState(ctx, stageRoot, "final-state", data)
	if err != nil {
		return err
	}
	proof.PackagesAfter, err = observeRollbackPackages(proof.ExtensionRoot)
	if err != nil {
		return err
	}
	return verifyRollbackCompatibility(ctx, stageRoot, stageRoot, proof.TargetVersion, proof)
}

func verifyRollbackCompatibility(ctx context.Context, root, stageRoot, target string, proof *RollbackCompatibility) error {
	if proof == nil || proof.Schema != 1 || proof.Mode != rollbackCompatibilityMode || proof.ExtensionRoot != filepath.Join(stageRoot, "acceptance-extensions") || proof.TargetVersion != target || proof.MinimumVersion != languageextension.NativeAdapterMinimumVersion || !slices.Equal(proof.DisabledIDs, []string{"golang.go", "vscode.typescript-language-features"}) {
		return errors.New("explicit owned rollback compatibility state is missing or differs")
	}
	var states [3]languageextension.State
	for i, input := range []File{proof.OriginalState, proof.AppliedState, proof.FinalState} {
		name := []string{"original-state", "applied-state", "final-state"}[i]
		if input.Path != "rollback-compatibility/"+name+".json" {
			return errors.New("retained rollback state path differs")
		}
		if err := VerifyFile(ctx, root, input, languageextension.MaxStateBytes); err != nil {
			return err
		}
		path, err := Within(root, input.Path)
		if err != nil {
			return err
		}
		data, err := readBounded(path, languageextension.MaxStateBytes)
		if err != nil || json.Unmarshal(data, &states[i]) != nil || len(states[i].Disabled) > 512 || len(states[i].Uninstall) > 512 {
			return errors.New("retained rollback extension preferences are invalid")
		}
	}
	if proof.OriginalStateAbsent && (len(states[0].Disabled) != 0 || len(states[0].Uninstall) != 0) || !reflect.DeepEqual(states[1], rollbackDisabledState(states[0])) || !reflect.DeepEqual(states[1], states[2]) || proof.AppliedState.Bytes != proof.FinalState.Bytes || proof.AppliedState.SHA256 != proof.FinalState.SHA256 {
		return errors.New("rollback acceptance changed other preferences or did not keep adapters disabled")
	}
	if len(proof.PackagesBefore) != 2 || !reflect.DeepEqual(proof.PackagesBefore, proof.PackagesAfter) {
		return errors.New("original native VSIX inventories changed during prior rollback")
	}
	for i, receipt := range proof.PackagesBefore {
		if receipt.Kind != []string{"go", "typescript"}[i] {
			return errors.New("rollback native package observation order differs")
		}
		if err := languageextension.ValidateReceipt(receipt); err != nil {
			return err
		}
	}
	return nil
}
