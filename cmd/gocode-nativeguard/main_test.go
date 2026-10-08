package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/gocode/internal/nativeguard"
)

func TestGuardCLIPrivateProcess(t *testing.T) {
	for i, arg := range os.Args {
		if arg == "--nativeguard-cli-helper" {
			if os.Args[i+1] == "nonzero" {
				fmt.Fprintln(os.Stderr, "real CLI error7")
				os.Exit(7)
			}
			fmt.Fprintln(os.Stdout, "real CLI success 世界😀")
			os.Exit(0)
		}
	}
}

func TestGuardCLIActualReportsReplacePriorSuccess(t *testing.T) {
	directory := t.TempDir()
	report := filepath.Join(directory, "current.json")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"success", "nonzero"} {
		var stdout, stderr bytes.Buffer
		code := run([]string{"-report", report, "-timeout", "5s", "--", executable, "-test.run=^TestGuardCLIPrivateProcess$", "--", "--nativeguard-cli-helper", mode}, &stdout, &stderr)
		data, err := os.ReadFile(report)
		if err != nil {
			t.Fatal(err)
		}
		var actual nativeguard.Report
		if err := json.Unmarshal(data, &actual); err != nil {
			t.Fatal(err)
		}
		if !actual.RootReaped || !actual.TreeClosed || actual.PID <= 0 || actual.DeadlineMS != 5000 {
			t.Fatalf("actual CLI ownership report: %+v", actual)
		}
		if mode == "success" && (code != 0 || actual.ExitCode != 0 || !strings.Contains(stdout.String(), "real CLI success")) {
			t.Fatal("actual success was not reported")
		}
		if mode == "nonzero" && (code == 0 || actual.ExitCode != 7 || actual.Error == "" || !strings.Contains(stderr.String(), "real CLI error7")) {
			t.Fatal("old success masqueraded as this failed process")
		}
		if output, err := os.ReadFile(filepath.Join(directory, "current.stdout.log")); err != nil || !bytes.Equal(output, stdout.Bytes()) {
			t.Fatal("actual stdout log changed")
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 3 {
		t.Fatalf("fixed reports accumulated files: %d %v", len(entries), err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-report", report, "-timeout", "121s", "--", executable}, &stdout, &stderr); code == 0 {
		t.Fatal("CLI admitted a deadline beyond the ceiling")
	}
	data, err := os.ReadFile(report)
	var rejected nativeguard.Report
	if err != nil || json.Unmarshal(data, &rejected) != nil || rejected.PID != 0 || rejected.Error == "" || rejected.ExitCode != -1 {
		t.Fatal("rejected command retained a prior success report")
	}
}
