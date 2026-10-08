package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/neko233-com/gocode/internal/copilotservice"
	"github.com/neko233-com/gocode/internal/languageextension"
	"github.com/neko233-com/gocode/internal/languageserver"
	"github.com/neko233-com/gocode/internal/uidispatch"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

// Binding fields are UI-owned; Supervisor/Session own all process state.
type languageBinding struct {
	lifetime context.Context // Immutable actor lifetime for worker receipts.
	config   languageserver.Config
	service  *languageserver.Supervisor
	session  *languageserver.Session
	state    string
	notice   string
	cancel   context.CancelFunc
}

// The actor's binding inventory is UI-owned; only its cancellation and worker
// set are shared. Hooks are composed once even if no server is installed yet.
type languageActor struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	dispatch            func(func()) bool
	workers             sync.WaitGroup
	closed              bool
	suppressAutomaticGo bool // UI-owned, monotonic after an installed-Go/marker receipt.
}

func (m *model) newLanguageBinding(actor *languageActor, config languageserver.Config) *languageBinding {
	ctx, cancel := context.WithCancel(actor.ctx)
	config.ClientVersion = appVersion()
	b := &languageBinding{lifetime: ctx, config: config, state: "starting", cancel: cancel}
	b.service = languageserver.Supervise(ctx, m.workspace, config)
	actor.workers.Go(func() {
		defer b.service.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-b.service.Events():
				if !ok {
					return
				}
				if event.Diagnostics != nil {
					path, err := languageserver.PathFromURI(event.Diagnostics.URI)
					if err != nil {
						continue
					}
					physical, err := canonicalPath(path)
					if err != nil {
						continue
					}
					copy := *event.Diagnostics
					copy.URI = copilotservice.FileURI(physical)
					event.Diagnostics = &copy
				}
				ack := make(chan struct{})
				if !uidispatch.Retry(ctx, actor.dispatch, func() {
					defer close(ack)
					if ctx.Err() == nil {
						m.applyLanguageEvent(b, event)
					}
				}) {
					return
				}
				select {
				case <-ack:
				case <-ctx.Done():
					return
				}
			}
		}
	})
	return b
}

// UI-only: stop removed identities before spawning replacements; no old
// generation can publish a diagnostic, completion or edit into a new binding.
func (m *model) reconcileLanguageExtensionConfigs(configs []languageserver.Config, installedGo bool) error {
	a := m.languageActor
	if a == nil || a.closed || a.ctx.Err() != nil {
		return nil
	}
	suppressAutomaticGo := a.suppressAutomaticGo || installedGo
	for _, config := range configs {
		if config.Name == "extension:golang.go" {
			suppressAutomaticGo = true
		}
	}
	independent := 0
	for _, b := range m.languageBindings {
		if !strings.HasPrefix(b.config.Name, "extension:") && !(suppressAutomaticGo && b.config.AutomaticGoFallback) {
			independent++
		}
	}
	if independent+len(configs) > 16 {
		return fmt.Errorf("at most 16 language servers can be active, including native VSIX adapters")
	}
	a.suppressAutomaticGo = suppressAutomaticGo
	retained := make([]*languageBinding, 0, len(m.languageBindings)+len(configs))
	var independentBindings []*languageBinding
	for _, b := range m.languageBindings {
		if suppressAutomaticGo && b.config.AutomaticGoFallback {
			m.clearLanguage(b)
			b.cancel()
			continue
		}
		if !strings.HasPrefix(b.config.Name, "extension:") {
			independentBindings = append(independentBindings, b)
			continue
		}
		keep := false
		for _, config := range configs {
			if config.Name == b.config.Name && languageextension.EqualConfig(config, b.config) {
				keep = true
				break
			}
		}
		if keep {
			retained = append(retained, b)
		} else {
			m.clearLanguage(b)
			b.cancel()
		}
	}
	m.languageBindings = retained
	for _, config := range configs {
		if !strings.HasPrefix(config.Name, "extension:") {
			continue
		}
		found := false
		for _, b := range m.languageBindings {
			if b.config.Name == config.Name {
				found = true
				break
			}
		}
		if !found {
			m.languageBindings = append(m.languageBindings, m.newLanguageBinding(a, config))
		}
	}
	// Requests scan backwards: preserve the startup policy and independent
	// provider order without replacing any retained session identity.
	m.languageBindings = append(m.languageBindings, independentBindings...)
	m.refreshLanguageStatus()
	return nil
}

func (m *model) currentLanguageBinding(b *languageBinding) bool {
	for _, current := range m.languageBindings {
		if current == b {
			return true
		}
	}
	return false
}

func (m *model) startLanguages(parent context.Context, cx *ui.Context, configs []languageserver.Config) func() {
	return m.bindLanguages(parent, cx.Dispatch, configs)
}

func (m *model) bindLanguages(parent context.Context, dispatch func(func()) bool, configs []languageserver.Config) func() {
	ctx, cancel := context.WithCancel(parent)
	actor := &languageActor{ctx: ctx, cancel: cancel, dispatch: dispatch}
	m.languageActor = actor
	for _, config := range configs {
		m.languageBindings = append(m.languageBindings, m.newLanguageBinding(actor, config))
	}
	previousDocument := m.onDocument
	m.onDocument = func(kind string, d *document, change textbuffer.ChangeEvent) {
		if previousDocument != nil {
			previousDocument(kind, d, change)
		}
		if d == nil {
			return
		}
		for _, b := range m.languageBindings {
			if b.config.Supports(d.path) {
				m.syncLanguageDocument(b, kind, d, change)
			}
		}
	}
	previousRequest := m.requestLSP
	m.requestLSP = func(d *document, method string) {
		if !d.serviceEligible() {
			m.message = "Language services use source snapshots up to 2 MiB"
			return
		}
		unavailable := ""
		for i := len(m.languageBindings) - 1; i >= 0; i-- {
			b := m.languageBindings[i]
			if !b.config.Supports(d.path) {
				continue
			}
			if b.session == nil || !b.session.Valid() {
				unavailable = b.config.Name + " " + b.state
				continue
			}
			if b.session.Client.Supports(method) {
				m.requestLanguage(b, d, method, dispatch)
				return
			}
		}
		if unavailable != "" {
			m.message = unavailable
		} else if previousRequest != nil {
			previousRequest(d, method)
		} else {
			m.message = "No configured language server advertises " + method
		}
	}
	previousCompletion := m.requestCompletions
	m.requestCompletions = func(d *document) {
		if previousCompletion != nil {
			previousCompletion(d)
		}
		if !d.serviceEligible() {
			return
		}
		for _, b := range m.languageBindings {
			if b.config.Supports(d.path) {
				m.requestLSP(d, "textDocument/completion")
				break
			}
		}
	}
	m.restartLanguages = func() {
		for _, b := range m.languageBindings {
			b.service.Restart()
			b.state = "restarting"
			m.clearLanguage(b)
		}
		m.refreshLanguageStatus()
	}
	m.refreshLanguageStatus()
	return func() {
		if actor.closed {
			return
		}
		actor.closed = true
		cancel()
		done := make(chan struct{})
		go func() { actor.workers.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
	}
}

func (m *model) refreshLanguageStatus() {
	var states []string
	for _, b := range m.languageBindings {
		states = append(states, b.config.Name+" "+b.state)
	}
	m.lspStatus = strings.Join(states, "; ")
}
func (m *model) clearLanguage(b *languageBinding) {
	b.session = nil
	for key := range m.diagnostics {
		if strings.HasPrefix(key, "lsp:"+b.config.Name+"\x00") {
			delete(m.diagnostics, key)
		}
	}
	m.publishCompletions("lsp:"+b.config.Name, nil)
}
func (m *model) applyLanguageEvent(b *languageBinding, e languageserver.Event) {
	if !m.currentLanguageBinding(b) || !b.service.Valid(e) {
		return
	}
	switch e.State {
	case "ready":
		m.clearLanguage(b)
		if b.notice != "" && m.message == b.notice {
			m.message = ""
		}
		b.notice = ""
		b.session, b.state = e.Session, "ready"
		for _, d := range m.docs {
			if b.config.Supports(d.path) {
				m.syncLanguageDocument(b, "open", d, textbuffer.ChangeEvent{})
			}
		}
	case "starting", "restarting", "failed":
		m.clearLanguage(b)
		b.state = e.State
		if e.Err != nil {
			b.notice = b.config.Name + ": " + e.Err.Error()
			m.message = b.notice
		}
	case "error":
		m.message = b.config.Name + ": " + e.Err.Error()
	case "diagnostics":
		if b.session != e.Session {
			return
		}
		p := e.Diagnostics
		path, err := languageserver.PathFromURI(p.URI)
		if err != nil {
			return
		}
		d := m.findDocument(path)
		if d == nil || !d.serviceEligible() || (p.Version != nil && *p.Version != d.buffer.Version()) {
			return
		}
		path = d.path
		items := make([]diagnostic, 0, len(p.Diagnostics))
		for _, item := range p.Diagnostics {
			items = append(items, diagnostic{Range: item.Range, Message: item.Message, Severity: max(0, min(3, item.Severity-1)), Path: path})
		}
		if m.diagnostics == nil {
			m.diagnostics = map[string][]diagnostic{}
		}
		m.diagnostics["lsp:"+b.config.Name+"\x00"+path] = items
	}
	m.refreshLanguageStatus()
}
func (m *model) syncLanguageDocument(b *languageBinding, kind string, d *document, change textbuffer.ChangeEvent) {
	session := b.session
	if session == nil || !session.Valid() {
		return
	}
	path := d.path
	var job func(context.Context, *languageserver.Client) error
	if kind == "close" || !d.serviceEligible() {
		job = func(ctx context.Context, client *languageserver.Client) error { return client.CloseDocument(ctx, path) }
		delete(m.diagnostics, "lsp:"+b.config.Name+"\x00"+path)
	} else {
		snapshot := d.buffer.Snapshot()
		switch kind {
		case "open", "change", "focus":
			job = func(ctx context.Context, client *languageserver.Client) error {
				return client.Sync(ctx, path, snapshot, &change)
			}
		case "save":
			job = func(ctx context.Context, client *languageserver.Client) error {
				if err := client.Sync(ctx, path, snapshot, nil); err != nil {
					return err
				}
				return client.Save(ctx, path, snapshot)
			}
		default:
			return
		}
	}
	if err := session.Submit(job); err != nil {
		m.message = b.config.Name + ": " + err.Error()
	}
}
func (m *model) requestLanguage(b *languageBinding, d *document, method string, dispatch func(func()) bool) {
	session := b.session
	path, snapshot, position, generation := d.path, d.buffer.Snapshot(), d.cursor(), m.inlineGeneration
	if !session.Request(func(ctx context.Context, client *languageserver.Client) {
		var raw json.RawMessage
		err := client.Request(ctx, path, snapshot, position, method, &raw)
		uidispatch.Retry(b.lifetime, dispatch, func() {
			current := m.findDocument(path)
			if !m.currentLanguageBinding(b) || b.session != session || !session.Valid() || current != d || !current.serviceEligible() || current.buffer.Version() != snapshot.Version {
				return
			}
			if method != "textDocument/formatting" && (current != m.current() || current.cursor() != position || generation != m.inlineGeneration) {
				return
			}
			if err != nil {
				m.message = b.config.Name + ": " + err.Error()
				return
			}
			m.applyLSPResult(b.config.Name, method, current, snapshot.Version, position, raw)
		})
	}) {
		m.message = fmt.Sprintf("%s busy or restarting; request was not accepted", b.config.Name)
	}
}
