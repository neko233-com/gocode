//go:build windows

package terminal

import (
	"context"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRealWindowsRawTerminalArguments(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("Node is required", err)
	}
	program := `if(JSON.stringify(process.argv.slice(1))!==JSON.stringify(['space value','世界😀','quote"value','']))process.exit(43);console.log('RAW_ARGS_OK');setTimeout(()=>{},1000);`
	raw := windows.ComposeCommandLine([]string{"-e", program, "--", "space value", "世界😀", `quote"value`, ""})
	session, err := Start(context.Background(), Config{Command: []string{node}, WindowsArguments: &raw, Directory: t.TempDir(), Environment: os.Environ()}, Size{Columns: 80, Rows: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer session.CloseAndWait()
	terminalUntil(t, session, func(frame *Frame) bool { return strings.Contains(frame.Text(), "RAW_ARGS_OK") })
}
