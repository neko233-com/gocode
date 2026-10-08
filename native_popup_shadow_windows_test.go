//go:build windows && cgo

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/gocode/internal/nativeguard"
	"github.com/neko233-com/godesktop/testing/winprobe"
)

func TestNativePopupShadowsAreIdempotent(t *testing.T) {
	root, err := filepath.Abs(filepath.Join(".cache", "popup-shadow-tests"))
	if err != nil {
		t.Fatal(err)
	}
	if err = validatePopupShadowDirectory(root); err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "gocode-popup-shadows.exe")
	expectedCommit := buildCommit()
	if expectedCommit == "development" {
		expectedCommit = "owned-popup-native-test"
	}
	buildCtx, stopBuild := context.WithTimeout(context.Background(), 2*time.Minute)
	defer stopBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-race", "-ldflags=-X main.version="+appVersion()+" -X main.sourceCommit="+expectedCommit, "-o", exe, ".")
	build.WaitDelay = 3 * time.Second
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("bounded popup build: %v\n%s", err, output)
	}
	if err = winprobe.ValidateAMD64PE(exe); err != nil {
		t.Fatal(err)
	}
	scratch, settings, evidence := filepath.Join(root, "process-temp"), filepath.Join(root, "settings"), filepath.Join(root, "evidence")
	for _, path := range []string{scratch, settings, evidence} {
		if err = os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(settings, "preserved.json")
	const userSettings = "unchanged 用户设置 😀\n"
	if err = os.WriteFile(marker, []byte(userSettings), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(marker); _ = os.Remove(settings); _ = os.Remove(scratch) })
	var firstCount int
	for _, adapter := range []string{"hardware", "warp"} {
		adapterEvidence := filepath.Join(evidence, adapter)
		environment := popupShadowEnvironment(map[string]string{"TMP": scratch, "TEMP": scratch, "APPDATA": settings, "GOCODE_POPUP_SHADOW_OUTPUT": adapterEvidence, "GODESKTOP_GPU_ADAPTER": adapter, "GODESKTOP_GPU_DEBUG": "0", "GODESKTOP_DX12_TRACE": "0"})
		result, err := nativeguard.Run(t.Context(), nativeguard.Options{Command: []string{exe, "-popup-shadow-smoke"}, Environment: environment, Timeout: 40 * time.Second})
		if err != nil {
			t.Fatalf("owned popup %s: %v\n%s\n%s", adapter, err, result.Stdout, result.Stderr)
		}
		if !result.Report.RootReaped || !result.Report.TreeClosed || result.Report.ExitCode != 0 {
			t.Fatalf("popup owned process closure: %+v", result.Report)
		}
		body, err := os.ReadFile(filepath.Join(adapterEvidence, "current.json"))
		if err != nil {
			t.Fatal(err)
		}
		var proof popupShadowReport
		if err = json.Unmarshal(body, &proof); err != nil {
			t.Fatal(err)
		}
		if proof.PID != result.Report.PID || proof.Version != appVersion() || proof.Commit != expectedCommit || len(proof.Stages) != 24 || len(proof.Runs) != 3 || !proof.ScratchRemoved || proof.Error != "" {
			t.Fatalf("popup real receipt incomplete: %+v", proof)
		}
		for i, stage := range proof.Stages {
			if stage.Run != i/8 || stage.Stage != popupShadowStages[i%8] || stage.Renderer.Completed < stage.CompletedFloor || stage.CompletedFloor < 3 || stage.Renderer.InFlight != 0 || stage.DPI == 0 {
				t.Fatalf("popup stage order/real completion: %+v", stage)
			}
			if err = winprobe.ValidateWindowsPresentation(stage.Renderer, stage.Presentation); err != nil {
				t.Fatal(err)
			}
			if adapter == "warp" && stage.Presentation != "committed-dib" {
				t.Fatal("forced WARP did not use its actual software presentation path", stage.Presentation)
			}
			if stage.Stage != "base" && (len(stage.Samples) == 0 || stage.Darkening < 1) {
				t.Fatalf("popup absent-halo gate has no real evidence: %+v", stage)
			}
		}
		for i, closed := range proof.Runs {
			if closed.Run != i || !closed.HWNDGone || !closed.OldContextRejected || !closed.WorkerJoined || closed.Renderer.InFlight != 0 || closed.Renderer.Completed != closed.Renderer.Submitted {
				t.Fatalf("popup Run closure: %+v", closed)
			}
		}
		if data, err := os.ReadFile(marker); err != nil || string(data) != userSettings {
			t.Fatal("popup acceptance changed private user settings", err)
		}
		entries, err := os.ReadDir(settings)
		if err != nil || len(entries) != 1 {
			t.Fatal("popup wrote unexpected settings", entries, err)
		}
		entries, err = os.ReadDir(scratch)
		if err != nil || len(entries) != 0 {
			t.Fatal("popup left process temporary files", entries, err)
		}
		if _, err := os.Lstat(filepath.Join(adapterEvidence, ".scratch")); !os.IsNotExist(err) {
			t.Fatal("popup workspace remains", err)
		}
		entries, err = os.ReadDir(adapterEvidence)
		if err != nil || len(entries) != 25 {
			t.Fatal("popup evidence incomplete/accumulated", entries, err)
		}
		if firstCount == 0 {
			firstCount = len(entries)
		} else if len(entries) != firstCount {
			t.Fatal("repeated popup evidence accumulated files")
		}
		t.Logf("popup policy=%s ownedPID=%d actual3Runs/24GPU captures: %s", adapter, proof.PID, result.Stdout)
	}
}

func popupShadowEnvironment(overrides map[string]string) []string {
	environment := []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		replaced := false
		for key := range overrides {
			if strings.EqualFold(name, key) {
				replaced = true
				break
			}
		}
		if !replaced {
			environment = append(environment, entry)
		}
	}
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}
	return environment
}
