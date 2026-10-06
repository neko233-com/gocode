package copilotservice

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sdk "github.com/github/copilot-sdk/go"
	"github.com/github/copilot-sdk/go/rpc"
)

type Chat struct {
	client  *sdk.Client
	mu      sync.Mutex
	session *sdk.Session
}

func NewChat(ctx context.Context, root, workspace, stateDir string) (*Chat, error) {
	cli, err := CLIPath(root)
	if err != nil {
		return nil, err
	}
	client := sdk.NewClient(&sdk.ClientOptions{Connection: sdk.StdioConnection{Path: cli}, WorkingDirectory: workspace, BaseDirectory: filepath.Join(stateDir, "copilot"), Mode: sdk.ModeEmpty, LogLevel: "none"})
	if err = client.Start(ctx); err != nil {
		client.ForceStop()
		return nil, err
	}
	return &Chat{client: client}, nil
}
func (c *Chat) Auth(ctx context.Context) (bool, error) {
	status, err := c.client.GetAuthStatus(ctx)
	if err != nil {
		return false, err
	}
	return status.IsAuthenticated, nil
}
func (c *Chat) Close() { c.client.ForceStop() }

// Ask streams text into the caller's UI dispatcher. Sessions persist chat context.
// Workspace tools are disabled until the editor has an explicit permission UI.
func (c *Chat) Ask(ctx context.Context, prompt string, delta func(string)) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil {
		session, err := c.client.CreateSession(ctx, &sdk.SessionConfig{
			ClientName: "gocode", Streaming: sdk.Bool(true), AvailableTools: []string{},
			EnableConfigDiscovery: sdk.Bool(false), EnableHostGitOperations: sdk.Bool(false), EnableSkills: sdk.Bool(false), EnableFileHooks: sdk.Bool(false),
			OnPermissionRequest: func(sdk.PermissionRequest, sdk.PermissionInvocation) (rpc.PermissionDecision, error) {
				return &rpc.PermissionDecisionReject{}, nil
			},
		})
		if err != nil {
			return "", err
		}
		c.session = session
	}
	var chunks sync.Mutex
	var text strings.Builder
	unsubscribe := c.session.On(func(event sdk.SessionEvent) {
		if d, ok := event.Data.(*sdk.AssistantMessageDeltaData); ok {
			chunks.Lock()
			if text.Len() < 1<<20 {
				text.WriteString(d.DeltaContent)
			}
			chunks.Unlock()
			if delta != nil {
				delta(d.DeltaContent)
			}
		}
	})
	defer unsubscribe()
	message, err := c.session.SendAndWait(ctx, sdk.MessageOptions{Prompt: prompt})
	if err != nil {
		if ctx.Err() != nil {
			abortCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_ = c.session.Abort(abortCtx)
			cancel()
		}
		return "", err
	}
	if message != nil {
		if d, ok := message.Data.(*sdk.AssistantMessageData); ok {
			return d.Content, nil
		}
	}
	chunks.Lock()
	defer chunks.Unlock()
	if text.Len() == 0 {
		return "", errors.New("Copilot returned no assistant message")
	}
	return text.String(), nil
}
