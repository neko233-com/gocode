package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neko233-com/gocode/internal/copilotservice"
	"github.com/neko233-com/gocode/internal/uidispatch"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func openCopilotURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() != "github.com" {
		return errors.New("Copilot requested an unsupported sign-in URL")
	}
	var command *exec.Cmd
	if runtime.GOOS == "windows" {
		command = exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", u.String())
	} else if runtime.GOOS == "darwin" {
		command = exec.Command("open", u.String())
	} else {
		command = exec.Command("xdg-open", u.String())
	}
	return command.Run()
}

func (m *model) startCopilot(parent context.Context, cx *ui.Context, explicit string) func() {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		root, err := copilotservice.RuntimeRoot(explicit)
		if err != nil {
			cx.Dispatch(func() { m.copilotStatus = "Copilot runtime unavailable" })
			return
		}
		language, err := copilotservice.NewLanguage(ctx, root, m.workspace)
		if err != nil {
			cx.Dispatch(func() { m.copilotStatus = "Copilot LSP: " + err.Error() })
			return
		}
		defer language.RPC.Close()
		state, err := os.UserConfigDir()
		if err != nil {
			return
		}
		startCtx, stop := context.WithTimeout(ctx, 20*time.Second)
		chat, chatErr := copilotservice.NewChat(startCtx, root, m.workspace, state+"/gocode")
		stop()
		var chatClient atomic.Pointer[copilotservice.Chat]
		chatClient.Store(chat)
		defer func() {
			if client := chatClient.Load(); client != nil {
				client.Close()
			}
		}()
		authenticated := false
		if chatErr == nil {
			authCtx, stop := context.WithTimeout(ctx, 5*time.Second)
			authenticated, chatErr = chat.Auth(authCtx)
			stop()
		}
		jobs := make(chan func(context.Context) error, 128)
		var inlineCancel, chatCancel, loginCancel context.CancelFunc
		var requests sync.WaitGroup
		enqueue := func(job func(context.Context) error) {
			select {
			case jobs <- job:
			case <-ctx.Done():
			default:
				m.message = "Copilot document queue is full; request completion to resynchronize"
			}
		}
		language.RPC.Register("window/showDocument", func(_ context.Context, params json.RawMessage) (any, error) {
			var request struct {
				URI string `json:"uri"`
			}
			if err := json.Unmarshal(params, &request); err != nil {
				return nil, err
			}
			err := openCopilotURL(request.URI)
			return map[string]bool{"success": err == nil}, err
		})
		uidispatch.Retry(ctx, cx.Dispatch, func() {
			m.copilotStatus = "Copilot: sign in required"
			if authenticated {
				m.copilotStatus = "Copilot SDK ready"
			}
			if chatErr != nil {
				m.copilotStatus = "Copilot chat: " + chatErr.Error()
			}
			previous := m.onDocument
			m.onDocument = func(kind string, d *document, change textbuffer.ChangeEvent) {
				if previous != nil {
					previous(kind, d, change)
				}
				if !d.serviceEligible() {
					enqueue(func(c context.Context) error { return language.Focus(c, "") })
					return
				}
				path, snapshot := d.path, d.buffer.Snapshot()
				switch kind {
				case "change", "open":
					enqueue(func(c context.Context) error { return language.Sync(c, path, snapshot, &change) })
				case "focus":
					enqueue(func(c context.Context) error {
						if err := language.Sync(c, path, snapshot, nil); err != nil {
							return err
						}
						return language.Focus(c, path)
					})
				case "close":
					enqueue(func(c context.Context) error { return language.CloseDocument(c, path) })
				}
			}
			for _, d := range m.docs {
				m.onDocument("open", d, textbuffer.ChangeEvent{})
			}
			if d := m.current(); d != nil {
				m.onDocument("focus", d, textbuffer.ChangeEvent{})
			}
			m.requestInline = func(d *document) {
				if !d.serviceEligible() {
					return
				}
				m.inlineGeneration++
				if inlineCancel != nil {
					inlineCancel()
				}
				requestCtx, stop := context.WithTimeout(ctx, 15*time.Second)
				inlineCancel = stop
				path, snapshot, position, generation := d.path, d.buffer.Snapshot(), d.cursor(), m.inlineGeneration
				requests.Go(func() {
					defer stop()
					timer := time.NewTimer(250 * time.Millisecond)
					defer timer.Stop()
					select {
					case <-timer.C:
					case <-requestCtx.Done():
						return
					}
					items, err := language.Inline(requestCtx, path, snapshot, position)
					if err != nil {
						if requestCtx.Err() == nil {
							cx.Dispatch(func() { m.copilotStatus = "Copilot completion: " + err.Error() })
						}
						return
					}
					cx.Dispatch(func() {
						current := m.current()
						if current == nil || current.buffer == nil || current.path != path || current.buffer.Version() != snapshot.Version || current.cursor() != position || generation != m.inlineGeneration {
							return
						}
						if len(items) == 0 {
							return
						}
						m.suggestion = &inlineSuggestion{path, snapshot.Version, position, items[0]}
						enqueue(func(c context.Context) error { return language.Shown(c, items[0]) })
					})
				})
			}
			m.cancelInline = func() {
				if inlineCancel != nil {
					inlineCancel()
				}
			}
			m.acceptInline = func(item copilotservice.InlineItem) {
				enqueue(func(c context.Context) error {
					err := language.Accepted(c, item)
					if err == nil {
						cx.Dispatch(func() { m.acceptedInline++ })
					}
					return err
				})
			}
			m.askChat = func(prompt string) {
				if m.chatLoginBusy {
					m.message = "Finish Copilot chat sign-in first"
					return
				}
				client := chatClient.Load()
				if client == nil {
					m.message = "Copilot SDK is unavailable: " + chatErr.Error()
					return
				}
				m.chatBusy = true
				if strings.HasPrefix(m.message, "Copilot:") {
					m.message = ""
				}
				if m.chatContext != "" {
					prompt = "User-attached context (" + m.chatContextLabel + "):\n" + m.chatContext + "\n\nUser request:\n" + prompt
					m.chatContext = ""
					m.chatContextLabel = ""
				}
				m.chatGeneration++
				generation := m.chatGeneration
				m.chatAnswer = ""
				requestCtx, stop := context.WithTimeout(ctx, 2*time.Minute)
				chatCancel = stop
				requests.Go(func() {
					defer stop()
					value, err := client.Ask(requestCtx, prompt, func(delta string) {
						cx.Dispatch(func() {
							if generation == m.chatGeneration && len(m.chatAnswer) < 256<<10 {
								m.chatAnswer += delta
							}
						})
					})
					uidispatch.Retry(ctx, cx.Dispatch, func() {
						if generation != m.chatGeneration {
							return
						}
						m.chatBusy = false
						if err != nil {
							m.message = "Copilot: " + err.Error()
						} else {
							m.chatAnswer = value
						}
					})
				})
			}
			m.cancelChat = func() {
				if m.chatLoginBusy && loginCancel != nil {
					loginCancel()
				}
				if chatCancel != nil {
					chatCancel()
				}
			}
			m.signInCopilot = func() {
				requests.Go(func() {
					c, stop := context.WithTimeout(ctx, 2*time.Minute)
					defer stop()
					result, err := language.SignIn(c)
					if err != nil {
						cx.Dispatch(func() { m.message = err.Error() })
						return
					}
					cx.Dispatch(func() { m.message = "GitHub device code: " + result.UserCode + " — complete sign-in in your browser" })
					if result.Command.Command != "" {
						if err := language.FinishSignIn(c, result.Command); err != nil {
							cx.Dispatch(func() { m.message = "Copilot sign-in: " + err.Error() })
						}
					}
				})
			}
			m.signInChat = func() {
				if m.chatBusy || m.chatLoginBusy {
					m.message = "Finish or cancel the current Copilot request first"
					return
				}
				m.chatLoginBusy = true
				m.copilotStatus = "Opening GitHub sign-in for Copilot chat…"
				loginCtx, stop := context.WithTimeout(ctx, 3*time.Minute)
				loginCancel = stop
				requests.Go(func() {
					defer stop()
					err := copilotservice.Login(loginCtx, root, state+"/gocode")
					if err == nil {
						next, createErr := copilotservice.NewChat(loginCtx, root, m.workspace, state+"/gocode")
						err = createErr
						if err == nil {
							if previous := chatClient.Swap(next); previous != nil {
								previous.Close()
							}
						}
					}
					uidispatch.Retry(ctx, cx.Dispatch, func() {
						m.chatLoginBusy = false
						if err != nil {
							m.copilotStatus = "Copilot chat sign-in failed: " + err.Error()
						} else {
							m.copilotStatus = "Copilot chat sign-in completed"
						}
					})
				})
			}
		})
		defer func() { cancel(); requests.Wait() }()
		for {
			select {
			case <-ctx.Done():
				return
			case job := <-jobs:
				c, stop := context.WithTimeout(ctx, 3*time.Second)
				err := job(c)
				stop()
				if err != nil {
					cx.Dispatch(func() { m.copilotStatus = "Copilot: " + err.Error() })
				}
			case notification, ok := <-language.RPC.Notifications():
				if !ok {
					return
				}
				if notification.Method == "didChangeStatus" {
					var status copilotservice.Status
					if json.Unmarshal(notification.Params, &status) == nil {
						cx.Dispatch(func() {
							m.copilotStatus = "Copilot " + status.Kind
							if status.Message != "" {
								m.copilotStatus += " — " + status.Message
							}
						})
					}
				}
			}
		}
	}()
	return func() { cancel(); <-done }
}

func ghostText(d *document, s *inlineSuggestion) string {
	if s == nil || d == nil || d.buffer == nil || d.path != s.path || d.buffer.Version() != s.version {
		return ""
	}
	text := s.item.InsertText
	if s.item.Range != nil {
		prefix, err := d.buffer.RangeText(textbuffer.Range{Start: s.item.Range.Start, End: s.position})
		if err != nil || !strings.HasPrefix(text, prefix) {
			return ""
		}
		text = strings.TrimPrefix(text, prefix)
	}
	return strings.SplitN(text, "\n", 2)[0]
}
