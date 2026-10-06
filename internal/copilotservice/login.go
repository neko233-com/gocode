package copilotservice

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Login delegates OAuth, browser opening and credential storage to the official
// CLI. Its output is discarded so credentials never enter the editor's logs.
func Login(ctx context.Context, root, stateDir string) error {
	cli, err := CLIPath(root)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, cli, "login", "--web-flow")
	configureLoginProcess(command)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !strings.EqualFold(key, "COPILOT_HOME") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "COPILOT_HOME="+filepath.Join(stateDir, "copilot"))
	command.Stdout, command.Stderr = io.Discard, io.Discard
	return command.Run()
}
