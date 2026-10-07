package main

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"time"

	"github.com/neko233-com/gocode/internal/uidispatch"
	ui "github.com/neko233-com/godesktop"
)

const maxAutoSaveDocuments = 128

type autoSaveEntry struct {
	version, failedVersion int
	due                    time.Time
	inFlight               bool
}

// Mutable scheduling state belongs to the UI. The worker receives only the next
// deadline, never a document or buffer, and permits one unacknowledged dispatch.
type autoSaveState struct {
	config                     autoSaveConfig
	pending                    map[*document]*autoSaveEntry
	arm                        func(time.Time)
	persist                    func(autoSaveConfig)
	parent, request            context.Context
	cancel                     context.CancelFunc
	focused                    *document
	focusedGroup               uint64
	editorFocused              bool
	windowKnown, windowFocused bool
	delayFocused               bool
	delayDraft                 string
	running                    bool
}

func (m *model) autoSaveConfig() autoSaveConfig {
	c := m.autoSave.config
	if validateAutoSaveConfig(c) != nil {
		return defaultAutoSaveConfig()
	}
	return c
}

func (m *model) configureAutoSave(c autoSaveConfig) {
	if err := validateAutoSaveConfig(c); err != nil {
		m.message = "Auto Save: " + err.Error()
		return
	}
	if c == m.autoSaveConfig() {
		return
	}
	if m.autoSave.cancel != nil {
		m.autoSave.cancel()
	}
	m.autoSave.config = c
	m.autoSave.pending = nil
	if m.autoSave.parent != nil {
		m.autoSave.request, m.autoSave.cancel = context.WithCancel(m.autoSave.parent)
	}
	if m.autoSave.persist != nil {
		m.autoSave.persist(c)
	}
	if c.Mode == "afterDelay" {
		for _, d := range m.docs {
			m.autoSaveChanged(d, time.Now())
		}
	}
	m.armAutoSave(time.Now())
}

func (m *model) toggleAutoSave() {
	c := m.autoSaveConfig()
	if c.Mode == "off" {
		c.Mode = "afterDelay"
	} else {
		c.Mode = "off"
	}
	m.configureAutoSave(c)
}

func (m *model) autoSaveEligible(d *document) bool {
	return d != nil && m.ownsDocument(d) && d.buffer != nil && !d.untitled && d.large == nil && d.diskKnown && d.diskConflict == nil && d.reloadID == 0 && d.dirty()
}

func (m *model) autoSaveEntry(d *document) *autoSaveEntry {
	if entry := m.autoSave.pending[d]; entry != nil {
		return entry
	}
	if len(m.autoSave.pending) >= maxAutoSaveDocuments {
		m.message = "Auto Save is limited to 128 dirty editable files; save remaining files manually"
		return nil
	}
	if m.autoSave.pending == nil {
		m.autoSave.pending = make(map[*document]*autoSaveEntry)
	}
	entry := &autoSaveEntry{}
	m.autoSave.pending[d] = entry
	return entry
}

func (m *model) autoSaveChanged(d *document, now time.Time) {
	if m.autoSaveConfig().Mode == "off" || !m.autoSaveEligible(d) {
		if e := m.autoSave.pending[d]; e != nil && !e.inFlight {
			delete(m.autoSave.pending, d)
		}
		return
	}
	e := m.autoSaveEntry(d)
	if e == nil {
		return
	}
	e.version = d.buffer.Version()
	if m.autoSaveConfig().Mode == "afterDelay" {
		e.due = now.Add(time.Duration(m.autoSaveConfig().DelayMS) * time.Millisecond)
	}
	m.armAutoSave(now)
}

func (m *model) autoSaveOnBlur(d *document, now time.Time) {
	if !m.autoSaveEligible(d) || m.autoSaveInteractionBlocked() {
		return
	}
	if e := m.autoSaveEntry(d); e != nil {
		e.version, e.due = d.buffer.Version(), now
	}
	m.armAutoSave(now)
}

func (m *model) observeAutoSaveFocus(now time.Time) {
	d := m.current()
	focused := m.editing && d != nil && !m.palette && !m.navigation && m.menu.name == "" && !m.terminalFocused && !m.chatFocused && !m.updateFocused && !m.autoSave.delayFocused && !m.closePrompt && m.reloadPrompt == nil && m.history.prompt == nil && m.extensionsView.detail == "" && m.scm.diff == nil && (!m.autoSave.windowKnown || m.autoSave.windowFocused)
	previous, hadFocus, previousGroup := m.autoSave.focused, m.autoSave.editorFocused, m.autoSave.focusedGroup
	m.autoSave.focused, m.autoSave.editorFocused = d, focused
	m.autoSave.focusedGroup = m.groups.active
	if m.autoSaveConfig().Mode == "onFocusChange" && hadFocus && (!focused || previous != d || previousGroup != m.groups.active) {
		m.autoSaveOnBlur(previous, now)
	}
}

func (m *model) autoSaveWindowFocus(focused bool, now time.Time) {
	changed := !m.autoSave.windowKnown || focused != m.autoSave.windowFocused
	m.autoSave.windowKnown, m.autoSave.windowFocused = true, focused
	mode := m.autoSaveConfig().Mode
	if changed && !focused && (mode == "onWindowChange" || mode == "onFocusChange") {
		for _, d := range m.docs {
			m.autoSaveOnBlur(d, now)
		}
	}
	m.observeAutoSaveFocus(now)
}

func (m *model) autoSaveBlocked() bool {
	return m.autoSaveInteractionBlocked() || m.saveBusy || len(m.saveJobs) != 0
}

func (m *model) autoSaveInteractionBlocked() bool {
	return m.closePrompt || m.closeBusy || m.reloadPrompt != nil || m.reloadBusy || m.history.prompt != nil || m.fileActions.busy
}

func (m *model) armAutoSave(now time.Time) {
	if m.autoSave.arm == nil {
		return
	}
	if m.requestSave == nil {
		m.autoSave.arm(time.Time{})
		return // A missing writer must never spin an already expired deadline.
	}
	var deadline time.Time
	for d, e := range m.autoSave.pending {
		if !m.autoSaveEligible(d) && !e.inFlight {
			delete(m.autoSave.pending, d)
			continue
		}
		if e.inFlight || e.due.IsZero() || e.failedVersion == e.version {
			continue
		}
		if deadline.IsZero() || e.due.Before(deadline) {
			deadline = e.due
		}
	}
	if !deadline.IsZero() && !deadline.After(now) && m.autoSaveBlocked() {
		deadline = now.Add(250 * time.Millisecond)
	}
	m.autoSave.arm(deadline)
}

func (m *model) autoSaveTick(now time.Time) {
	if m.autoSaveConfig().Mode == "off" || m.autoSave.request == nil || m.autoSave.request.Err() != nil {
		return
	}
	if m.autoSaveBlocked() || m.requestSave == nil {
		m.armAutoSave(now)
		return
	}
	// Stable document order breaks equal deadline ties. Only one automatic job
	// is accepted at once; manual saves and close barriers retain queue priority.
	var target *document
	var selected *autoSaveEntry
	for _, d := range m.docs {
		e := m.autoSave.pending[d]
		if e == nil || e.inFlight || e.due.IsZero() || e.due.After(now) || e.failedVersion == e.version || !m.autoSaveEligible(d) {
			continue
		}
		if selected == nil || e.due.Before(selected.due) {
			target, selected = d, e
		}
	}
	if target == nil {
		m.armAutoSave(now)
		return
	}
	selected.inFlight, selected.due = true, time.Time{}
	version := target.buffer.Version()
	request := m.autoSave.request
	m.requestSave(request, []*document{target}, func(err error) {
		if m.autoSave.pending[target] != selected {
			return // A changed policy/closed identity owns a different generation.
		}
		selected.inFlight = false
		if err != nil && !errors.Is(err, errSaveChanged) && !errors.Is(err, context.Canceled) {
			selected.failedVersion = version
			if m.ownsDocument(target) {
				m.message = "Auto Save " + filepath.Base(target.path) + ": " + err.Error()
			}
		}
		if !m.autoSaveEligible(target) {
			delete(m.autoSave.pending, target)
		} else if target.buffer.Version() != version && m.autoSaveConfig().Mode == "afterDelay" && selected.due.IsZero() {
			m.autoSaveChanged(target, time.Now())
		}
		m.armAutoSave(time.Now())
	})
	m.armAutoSave(now)
}

func replaceAutoSaveDeadline(ch chan time.Time, value time.Time) {
	select {
	case ch <- value:
	default:
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- value:
		default:
		}
	}
}

func (m *model) startAutoSaveActor(parent context.Context, dispatch func(func()) bool) func() {
	ctx, stop := context.WithCancel(parent)
	deadlines, done := make(chan time.Time, 1), make(chan struct{})
	m.autoSave.parent = ctx
	m.autoSave.request, m.autoSave.cancel = context.WithCancel(ctx)
	m.autoSave.running = true
	m.autoSave.arm = func(deadline time.Time) { replaceAutoSaveDeadline(deadlines, deadline) }
	go func() {
		defer close(done)
		timer := time.NewTimer(time.Hour)
		defer timer.Stop()
		timer.Stop()
		var tick <-chan time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case deadline := <-deadlines:
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				tick = nil
				if !deadline.IsZero() {
					timer.Reset(max(time.Nanosecond, time.Until(deadline)))
					tick = timer.C
				}
			case now := <-tick:
				tick = nil
				ack := make(chan struct{}, 1)
				if !uidispatch.Retry(ctx, dispatch, func() {
					if ctx.Err() == nil {
						m.autoSaveTick(now)
					}
					ack <- struct{}{}
				}) {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-ack:
				}
			}
		}
	}()
	for _, d := range m.docs {
		m.autoSaveChanged(d, time.Now())
	}
	m.observeAutoSaveFocus(time.Now())
	return func() {
		m.autoSave.arm, m.autoSave.running = nil, false
		m.autoSave.cancel()
		stop()
		<-done
		m.autoSave.pending = nil
	}
}

func (m *model) startAutoSaveSettings(parent context.Context, dispatch func(func()) bool, path string) func() {
	persist, stop := startSettingsWriter(parent, dispatch,
		func(value autoSaveConfig) error { return writeAutoSaveConfig(path, value) },
		func(value autoSaveConfig, err error) {
			if m.autoSaveConfig() == value {
				m.message = "Auto Save settings: " + err.Error()
			}
		})
	m.autoSave.persist = persist
	return func() {
		m.autoSave.persist = nil
		stop()
	}
}

func (m *model) autoSaveSidebar() *ui.Element {
	items := []*ui.Element{label("AUTO SAVE").FontSize(11).Height(28)}
	for _, mode := range []struct{ id, title string }{{"off", "Off"}, {"afterDelay", "After Delay"}, {"onFocusChange", "On Focus Change"}, {"onWindowChange", "On Window Change"}} {
		title := mode.title
		if m.autoSaveConfig().Mode == mode.id {
			title = "✓ " + title
		}
		items = append(items, button(title, "autosave-"+mode.id, func(*ui.Context) { c := m.autoSaveConfig(); c.Mode = mode.id; m.configureAutoSave(c) }).Height(26))
	}
	delay := strconv.Itoa(m.autoSaveConfig().DelayMS)
	if m.autoSave.delayFocused {
		delay = m.autoSave.delayDraft + "▏"
	}
	items = append(items, label("Delay in ms (Enter to save)").FontSize(11).Height(24), button(delay, "autosave-delay", func(*ui.Context) {
		m.autoSave.delayFocused = true
		m.autoSave.delayDraft = strconv.Itoa(m.autoSaveConfig().DelayMS)
		m.editing, m.updateFocused, m.terminalFocused, m.chatFocused = false, false, false, false
	}).Height(26).Background(ui.RGB(0x313131)))
	return ui.Column(items...).PaddingXY(12, 0)
}

func (m *model) openSettings() {
	m.hideSidebar, m.activity = false, "settings"
	m.editing, m.terminalFocused, m.chatFocused = false, false, false
	m.palette, m.navigation = false, false
	m.autoSave.delayFocused = false
}

func (m *model) autoSaveSettingsInput(e ui.InputEvent) bool {
	if !m.autoSave.delayFocused {
		return false
	}
	if e.Kind == ui.PointerPressed {
		m.autoSave.delayFocused = false
		return false
	}
	if e.Kind == ui.Character && e.Key >= '0' && e.Key <= '9' && e.Modifiers&(ui.ModifierControl|ui.ModifierAlt|ui.ModifierCommand) == 0 && len(m.autoSave.delayDraft) < 7 {
		m.autoSave.delayDraft += string(rune(e.Key))
		return true
	}
	if e.Kind == ui.KeyPressed {
		switch e.Key {
		case 8:
			if len(m.autoSave.delayDraft) > 0 {
				m.autoSave.delayDraft = m.autoSave.delayDraft[:len(m.autoSave.delayDraft)-1]
			}
			return true
		case 'A':
			if e.Modifiers&(ui.ModifierControl|ui.ModifierCommand) != 0 {
				m.autoSave.delayDraft = ""
				return true
			}
		case 27:
			m.autoSave.delayFocused = false
			return true
		case 13:
			value, err := strconv.Atoi(m.autoSave.delayDraft)
			if err == nil {
				c := m.autoSaveConfig()
				c.DelayMS = value
				m.configureAutoSave(c)
			} else {
				m.message = "Auto Save delay must be an integer between 100 and 600000 milliseconds"
			}
			m.autoSave.delayFocused = false
			return true
		}
	}
	return e.Kind == ui.Character || e.Kind == ui.KeyPressed || e.Kind == ui.KeyReleased
}
