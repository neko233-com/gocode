// gocode-localreleasecheck verifies offline release bytes; it never signs,
// creates tags, uploads assets, discovers latest releases or installs user data.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/gocode/internal/localrelease"
)

func run(args []string) error {
	flags := flag.NewFlagSet("gocode-localreleasecheck", flag.ContinueOnError)
	mode := flags.String("mode", "plan", "plan, framework, inputs, stage, probe, gate, rollback or verify")
	project := flags.String("project", ".", "Exact clean product checkout")
	root := flags.String("root", "", "Owned local release or empty staging root")
	archive := flags.String("archive", "", "Actual local updater ZIP")
	envelope := flags.String("manifest", "", "Actual production-signed local update manifest")
	receipt := flags.String("receipt", "", "Complete source/package/gate receipt")
	version := flags.String("version", "", "Exact VERSION")
	source := flags.String("source", "", "Exact immutable product source")
	frameworkVersion := flags.String("framework-version", "v0.17.0", "Actual published framework version")
	frameworkSource := flags.String("framework-source", "996b5ff16189d95ea72298ee48bd03366b27757e", "Actual published framework source")
	output := flags.String("output", "", "Fixed JSON result path")
	gateID := flags.String("gate", "", "One exact ordered Windows native gate ID")
	priorArchive := flags.String("prior-archive", "", "Actual prior signed release ZIP for production rollback")
	priorManifest := flags.String("prior-manifest", "", "Actual production-signed prior manifest")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected local-release positional arguments")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var result any
	switch *mode {
	case "plan":
		result = struct {
			Platforms       []string                `json:"platforms"`
			Checks          []string                `json:"checks"`
			Gates           []localrelease.GateSpec `json:"gates"`
			PendingCommands []string                `json:"pendingCommands"`
		}{[]string{"windows/amd64"}, localrelease.RequiredChecks(), localrelease.WindowsPlan(), localrelease.PendingCommands()}
	case "framework":
		actual, err := localrelease.CheckFramework(ctx, *project, *frameworkVersion, *frameworkSource)
		if err != nil {
			return err
		}
		result = actual
	case "inputs":
		actual, err := localrelease.InputDigest(ctx, *project)
		if err != nil {
			return err
		}
		result = struct {
			InputDigest string `json:"inputDigest"`
		}{actual}
	case "stage", "probe", "gate", "rollback", "verify":
		if *version == "" || !localrelease.ValidSource(*source) || *root == "" {
			return errors.New("exact version/source/owned root are required")
		}
		if err := localrelease.CheckCleanSource(ctx, *project, *source); err != nil {
			return err
		}
		if *mode == "rollback" {
			current, err := localrelease.ReadStaged(*receipt)
			if err != nil || current.Version != *version || current.Source != *source || *output == "" {
				return errors.New("rollback requires the actual bounded new staged result and fixed output")
			}
			proof, captured, runErr := localrelease.RunRollback(ctx, *root, *priorArchive, *priorManifest, current, os.Environ())
			stem := strings.TrimSuffix(*output, filepath.Ext(*output))
			writeErr := os.WriteFile(stem+".stdout.log", captured.Stdout, 0600)
			writeErr = errors.Join(writeErr, os.WriteFile(stem+".stderr.log", captured.Stderr, 0600))
			writeErr = errors.Join(writeErr, localrelease.WriteJSON(stem+".native.json", localrelease.GateResult{Launch: proof.Native.Launch, Report: proof.Native.Result}))
			writeErr = errors.Join(writeErr, localrelease.WriteJSON(*output, proof))
			return errors.Join(runErr, writeErr)
		} else if *mode == "gate" {
			var spec *localrelease.GateSpec
			for _, candidate := range localrelease.WindowsPlan() {
				if candidate.ID == *gateID {
					selected := candidate
					spec = &selected
					break
				}
			}
			if spec == nil || len(spec.Args) == 0 || *output == "" {
				return errors.New("a real wired gate and fixed output file are required")
			}
			captured, bound, runErr := localrelease.RunGate(ctx, *root, *version, *source, *spec, os.Environ())
			stem := strings.TrimSuffix(*output, filepath.Ext(*output))
			writeErr := os.WriteFile(stem+".stdout.log", captured.Stdout, 0600)
			writeErr = errors.Join(writeErr, os.WriteFile(stem+".stderr.log", captured.Stderr, 0600), localrelease.WriteJSON(*output, bound))
			if err := errors.Join(runErr, writeErr); err != nil {
				return err
			}
			return nil
		} else if *mode == "verify" {
			actual, err := localrelease.VerifyReceipt(ctx, *root, *project, *receipt, *version, *source)
			if err != nil {
				return err
			}
			result = actual
		} else if *mode == "stage" {
			staged, err := localrelease.StageSigned(ctx, *root, *archive, *envelope, *version, *source, "windows/amd64")
			if err != nil {
				return err
			}
			if err := localrelease.ProbeStaged(ctx, *root, staged); err != nil {
				return err
			}
			result = staged
		} else {
			if *receipt == "" {
				return errors.New("probe requires the actual local staged JSON result")
			}
			staged, err := localrelease.ReadStaged(*receipt)
			if err != nil || staged.Version != *version || staged.Source != *source {
				return errors.New("staged source/version differs")
			}
			if err := localrelease.ProbeStaged(ctx, *root, staged); err != nil {
				return err
			}
			result = staged
		}
	default:
		return fmt.Errorf("unknown local release mode: %s", *mode)
	}
	if strings.TrimSpace(*output) != "" {
		return localrelease.WriteJSON(*output, result)
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
