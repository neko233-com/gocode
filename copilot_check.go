package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/gocode/internal/copilotservice"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func checkCopilot(explicit, workspace string, prompt bool) error {
	root, err := copilotservice.RuntimeRoot(explicit)
	if err != nil {
		return err
	}
	if prompt {
		fixture, err := os.MkdirTemp("", "gocode-copilot-acceptance-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(fixture)
		workspace = fixture
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	language, err := copilotservice.NewLanguage(ctx, root, workspace)
	if err != nil {
		return fmt.Errorf("official Copilot LSP: %w", err)
	}
	defer language.RPC.Close()
	state, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	chat, err := copilotservice.NewChat(ctx, root, workspace, state+"/gocode")
	if err != nil {
		return fmt.Errorf("official Copilot SDK: %w", err)
	}
	defer chat.Close()
	authenticated, err := chat.Auth(ctx)
	if err != nil {
		return err
	}
	result := map[string]any{"lspInitialized": true, "sdkConnected": true, "authenticated": authenticated, "networkPromptSent": false}
	if prompt {
		if !authenticated {
			return errors.New("Copilot SDK is not authenticated; sign in with the official Copilot CLI first")
		}
		value, err := chat.Ask(ctx, "Reply with exactly GOCODE_COPILOT_OK. Do not use tools or inspect any files.", nil)
		if err != nil {
			return err
		}
		if value != "GOCODE_COPILOT_OK" {
			return errors.New("Copilot synthetic response did not match acceptance fixture")
		}
		result["networkPromptSent"] = true
		result["chatVerified"] = true
		fixture := "package main\n\n// add returns the sum of two integers.\nfunc add(a, b int) int {\n    "
		buffer, _ := textbuffer.New(fixture)
		items, err := language.Inline(ctx, filepath.Join(workspace, "main.go"), buffer.Snapshot(), textbuffer.Position{Line: 4, Character: 4})
		if err != nil {
			return fmt.Errorf("Copilot inline acceptance: %w", err)
		}
		if len(items) == 0 || !strings.Contains(items[0].InsertText, "return") {
			return errors.New("Copilot returned no inline suggestion for the synthetic addition fixture")
		}
		result["inlineVerified"] = true
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
