package terminal

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed all:assets/zsh-syntax-highlighting
var shellAssets embed.FS

// DefaultShell prepares a session-local profile. It never edits the user's
// PowerShell profile, .zshrc or Oh My Zsh installation. Call Start with this
// config; Start owns cleanup on both failed startup and completed shutdown.
func DefaultShell(directory string) (Config, error) {
	return Shell(directory, false)
}

// Shell's isolated mode uses an owned profile and disables history writes for
// native acceptance. Normal terminals retain the user's startup configuration.
func Shell(directory string, isolated bool) (Config, error) {
	config := Config{Directory: directory, Environment: os.Environ()}
	if runtime.GOOS == "windows" {
		path, err := exec.LookPath("pwsh.exe")
		if err != nil {
			path, err = exec.LookPath("powershell.exe")
		}
		if err != nil {
			return config, fmt.Errorf("PowerShell is unavailable: %w", err)
		}
		config.Name = "PowerShell"
		// ConPTY uses UTF-8, while Console.WriteLine and native pipes can retain
		// the inherited OEM code page. Configure only this owned shell session.
		encoding := `[Console]::InputEncoding = [Console]::OutputEncoding = $OutputEncoding = [Text.UTF8Encoding]::new($false); `
		// char 27 also works with Windows PowerShell 5.1, which lacks `e.
		script := encoding + `try { Import-Module PSReadLine -ErrorAction Stop; $e=[char]27; Set-PSReadLineOption -EditMode Windows -Colors @{ Command="$e[38;2;220;220;170m"; String="$e[38;2;206;145;120m"; Number="$e[38;2;181;206;168m"; Keyword="$e[38;2;197;134;192m"; Variable="$e[38;2;156;220;254m"; Parameter="$e[38;2;156;220;254m"; Operator="$e[38;2;204;204;204m"; Comment="$e[38;2;106;153;85m"; Default="$e[38;2;204;204;204m"; Error="$e[38;2;244;71;71m" }; } catch { Write-Warning ('gocode shell highlighting: ' + $_.Exception.Message) }`
		config.Command = []string{path, "-NoLogo", "-NoExit", "-Command", script}
		if isolated {
			config.Command = append([]string{path, "-NoProfile"}, config.Command[1:]...)
			config.Command[len(config.Command)-1] += "; Set-PSReadLineOption -HistorySaveStyle SaveNothing; function prompt { 'gocode-test> ' }"
		}
		return config, nil
	}
	path := os.Getenv("SHELL")
	if isolated && runtime.GOOS == "darwin" {
		path = "/bin/zsh"
	}
	if path == "" {
		if runtime.GOOS == "darwin" {
			path = "/bin/zsh"
		} else {
			path = "/bin/bash"
		}
	}
	var err error
	path, err = exec.LookPath(path)
	if err != nil {
		return config, err
	}
	config.Command, config.Name = []string{path, "-i"}, filepath.Base(path)
	if config.Name != "zsh" {
		if isolated && config.Name == "bash" {
			config.Command = []string{path, "--noprofile", "--norc", "-i"}
			config.Environment = append(config.Environment, "PS1=gocode-test> ", "HISTFILE=/dev/null")
		}
		return config, nil
	}
	root, err := os.MkdirTemp("", "gocode-zsh-profile-")
	if err != nil {
		return config, err
	}
	config.cleanup = func() { _ = os.RemoveAll(root) }
	success := false
	defer func() {
		if !success {
			config.cleanup()
		}
	}()
	const prefix = "assets/zsh-syntax-highlighting/"
	err = fs.WalkDir(shellAssets, strings.TrimSuffix(prefix, "/"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := shellAssets.ReadFile(path)
		if err != nil {
			return err
		}
		target := filepath.Join(root, "highlighting", filepath.FromSlash(strings.TrimPrefix(path, prefix)))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
	if err != nil {
		return config, err
	}
	// Paths travel through environment variables, never shell interpolation.
	// Restore original ZDOTDIR before sourcing user startup files so nested
	// Oh My Zsh/plugin configuration retains its normal directory semantics.
	original := os.Getenv("ZDOTDIR")
	if isolated {
		original = filepath.Join(root, "original")
		if err := os.Mkdir(original, 0700); err != nil {
			return config, err
		}
		if err := os.WriteFile(filepath.Join(original, ".zshrc"), []byte("PROMPT='gocode-test> '\nunset HISTFILE\n"), 0600); err != nil {
			return config, err
		}
	}
	if original == "" {
		original, err = os.UserHomeDir()
		if err != nil {
			return config, err
		}
	}
	zshenv := `if [[ -n $GOCODE_ORIGINAL_ZDOTDIR && -r $GOCODE_ORIGINAL_ZDOTDIR/.zshenv ]]; then
  source "$GOCODE_ORIGINAL_ZDOTDIR/.zshenv"
fi
export ZDOTDIR=$GOCODE_ZSH_PROFILE
`
	zshrc := `export ZDOTDIR=$GOCODE_ORIGINAL_ZDOTDIR
if [[ -r $ZDOTDIR/.zshrc ]]; then source "$ZDOTDIR/.zshrc"; fi
source "$GOCODE_ZSH_PROFILE/highlighting/zsh-syntax-highlighting.zsh"
ZSH_HIGHLIGHT_STYLES[command]='fg=#dcdcaa'
ZSH_HIGHLIGHT_STYLES[builtin]='fg=#dcdcaa'
ZSH_HIGHLIGHT_STYLES[single-quoted-argument]='fg=#ce9178'
ZSH_HIGHLIGHT_STYLES[double-quoted-argument]='fg=#ce9178'
ZSH_HIGHLIGHT_STYLES[unknown-token]='fg=#f44747'
`
	if err := os.WriteFile(filepath.Join(root, ".zshenv"), []byte(zshenv), 0600); err != nil {
		return config, err
	}
	if err := os.WriteFile(filepath.Join(root, ".zshrc"), []byte(zshrc), 0600); err != nil {
		return config, err
	}
	config.Environment = append(config.Environment, "GOCODE_ORIGINAL_ZDOTDIR="+original, "GOCODE_ZSH_PROFILE="+root, "ZDOTDIR="+root)
	success = true
	return config, nil
}
