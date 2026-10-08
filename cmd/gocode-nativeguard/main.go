// gocode-nativeguard is an acceptance supervisor, not a distributed launcher.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/neko233-com/gocode/internal/nativeguard"
)

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gocode-nativeguard", flag.ContinueOnError)
	flags.SetOutput(stderr)
	reportPath := flags.String("report", "", "Fixed owned JSON report; adjacent .stdout.log and .stderr.log are replaced")
	deadline := flags.Duration("timeout", nativeguard.MaxRuntime, "Owned process ceiling, at most120s; fixture work90s is unchanged")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *reportPath == "" || filepath.Ext(*reportPath) != ".json" {
		fmt.Fprintln(stderr, "native guard requires a fixed -report path.json")
		return 2
	}
	// Prepare the report destination before a native child can run. No native
	// callback performs report I/O; all output is captured by bounded workers.
	if err := os.MkdirAll(filepath.Dir(*reportPath), 0700); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	pending, _ := json.Marshal(nativeguard.Report{ExitCode: -1, Error: "native guard has not completed"})
	if err := os.WriteFile(*reportPath, append(pending, '\n'), 0600); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	result, err := nativeguard.Run(context.Background(), nativeguard.Options{Command: flags.Args(), Timeout: *deadline})
	stem := (*reportPath)[:len(*reportPath)-len(".json")]
	writeErr := os.WriteFile(stem+".stdout.log", result.Stdout, 0600)
	writeErr = errors.Join(writeErr, os.WriteFile(stem+".stderr.log", result.Stderr, 0600))
	if writeErr != nil {
		result.Report.Error = errors.Join(err, writeErr).Error()
	}
	data, encodeErr := json.MarshalIndent(result.Report, "", "  ")
	writeErr = errors.Join(writeErr, encodeErr)
	if encodeErr == nil {
		writeErr = errors.Join(writeErr, os.WriteFile(*reportPath, append(data, '\n'), 0600))
	}
	_, outErr := stdout.Write(result.Stdout)
	_, stderrErr := stderr.Write(result.Stderr)
	if err != nil {
		fmt.Fprintln(stderr, "native guard:", err)
	}
	if writeErr != nil {
		fmt.Fprintln(stderr, "native guard report:", writeErr)
	}
	if err != nil || writeErr != nil || outErr != nil || stderrErr != nil {
		return 1
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
