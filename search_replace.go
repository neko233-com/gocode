package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	workspaceSearch "github.com/neko233-com/gocode/internal/search"
	ui "github.com/neko233-com/godesktop"
	textbuffer "github.com/neko233-com/godesktop/editor"
)

func (m *model) clearReplacement() {
	m.search.replaceGeneration++
	m.search.replacePlan = nil
	m.search.replaceBusy = false
	m.search.replaceStatus = ""
}
func (m *model) replacementChanged() {
	if m.search.replaceBusy && m.search.cancel != nil {
		m.search.cancel()
	}
	m.clearReplacement()
}
func (m *model) previewReplacement(cx *ui.Context) {
	if m.search.validate == nil || m.search.busy || m.search.replaceSaving {
		return
	}
	m.clearReplacement()
	generation, queryGeneration := m.search.replaceGeneration, m.search.generation
	query, replacement, root, report := m.search.query, m.search.replacement, m.workspace, workspaceSearch.CloneResult(m.search.report)
	overlays := map[string]workspaceSearch.Overlay{}
	for _, match := range report.Matches {
		if d := m.findDocument(filepath.Join(root, filepath.FromSlash(match.Path))); d != nil && d.buffer != nil {
			m.tabKey(d)
			overlays[match.Path] = workspaceSearch.Overlay{Snapshot: d.buffer.Snapshot(), Identity: d.tabID}
		}
	}
	m.search.replaceBusy = true
	m.search.replaceStatus = "Preparing replace preview…"
	var plan *workspaceSearch.ReplacementPlan
	m.search.validate(func(c context.Context) error {
		var err error
		plan, err = workspaceSearch.PrepareReplacement(c, root, query, report, replacement, overlays)
		return err
	}, func(err error) {
		if generation != m.search.replaceGeneration || queryGeneration != m.search.generation {
			return
		}
		m.search.replaceBusy = false
		if err != nil {
			m.search.replaceStatus = "Replace preview: " + err.Error()
		} else {
			m.search.replacePlan = plan
			m.search.replaceStatus = fmt.Sprintf("Preview: %d replacements in %d files", plan.Count, len(plan.Files))
		}
		if cx != nil {
			cx.Invalidate()
		}
	})
}

func (m *model) replacementButton(cx *ui.Context) {
	if m.search.replacePressed {
		matches := m.search.replaceCapture == m.search.replacePlan && m.search.replacePressGeneration == m.search.replaceGeneration
		m.search.replacePressed = false
		m.search.replaceCapture = nil
		if !matches {
			m.search.replaceStatus = "Preview changed during click; review again"
			return
		}
	}
	if m.search.replacePlan == nil {
		m.previewReplacement(cx)
	} else {
		m.applyReplacement(cx)
	}
}

type replacementTarget struct {
	document *document
	file     workspaceSearch.ReplacementFile
	path     string
	expected [32]byte
}

func (m *model) replacementTargets(plan *workspaceSearch.ReplacementPlan) ([]replacementTarget, error) {
	if plan == nil || m.saveBusy || len(m.saveJobs) > 0 {
		return nil, errors.New("wait for pending saves or prepare a new preview")
	}
	seen := map[string]bool{}
	seenDocuments := map[*document]bool{}
	targets := make([]replacementTarget, 0, len(plan.Files))
	for _, file := range plan.Files {
		path := filepath.Join(m.workspace, filepath.FromSlash(file.Path))
		if seen[pathKey(path)] {
			return nil, errors.New("duplicate replacement document")
		}
		seen[pathKey(path)] = true
		d := m.findDocument(path)
		expected := file.DiskHash
		if file.New != nil {
			if d != nil {
				return nil, errors.New("a preview file was opened; prepare again")
			}
			d = &document{path: path, buffer: file.New, diskHash: file.DiskHash, diskKnown: true}
		} else {
			if d == nil || d.buffer == nil || d.tabID != file.Identity || !d.diskKnown {
				return nil, errors.New("preview document was closed/reopened or has no saved disk revision")
			}
			expected = d.diskHash
		}
		if !d.buffer.CanCommit(file.Prepared) {
			return nil, errors.New("preview document changed; prepare again")
		}
		if seenDocuments[d] {
			return nil, errors.New("duplicate replacement buffer identity")
		}
		seenDocuments[d] = true
		targets = append(targets, replacementTarget{d, file, path, expected})
	}
	return targets, nil
}

// All buffers are committed in one UI turn after complete disk/state preflight.
// Existing per-file save transactions follow asynchronously; failures preserve
// undoable dirty buffers and report unsaved counts. This is not filesystem-wide
// atomicity against unrelated processes or machine failure.
func (m *model) applyReplacement(cx *ui.Context) {
	if m.search.validate == nil || m.search.replaceBusy || m.search.replaceSaving || m.requestSave == nil {
		return
	}
	plan := m.search.replacePlan
	targets, err := m.replacementTargets(plan)
	if err != nil {
		m.search.replaceStatus = err.Error()
		return
	}
	generation, queryGeneration := m.search.replaceGeneration, m.search.generation
	m.search.replaceBusy = true
	m.search.replaceStatus = "Checking replacement sources…"
	m.search.validate(func(c context.Context) error {
		for _, target := range targets {
			if err := checkDiskVersion(c, target.path, &target.expected); err != nil {
				return fmt.Errorf("%s: %w", target.file.Path, err)
			}
		}
		return nil
	}, func(err error) {
		if generation != m.search.replaceGeneration || queryGeneration != m.search.generation {
			return
		}
		m.search.replaceBusy = false
		if err != nil {
			m.search.replaceStatus = "Replace aborted: " + err.Error()
			return
		}
		if _, err := m.replacementTargets(plan); err != nil {
			m.search.replaceStatus = "Replace aborted: " + err.Error()
			return
		}
		for _, target := range targets {
			if target.file.New != nil {
				m.adoptDocument(target.path, target.document, false)
			}
		}
		// Open notifications happen before final preflight; their hooks must not
		// interleave with the actual all-buffer commit.
		for _, target := range targets {
			if !m.ownsDocument(target.document) || !target.document.buffer.CanCommit(target.file.Prepared) || m.saveBusy || len(m.saveJobs) > 0 {
				m.search.replaceStatus = "Replace aborted: document changed while resolving files"
				return
			}
		}
		changes := make([]textbuffer.ChangeEvent, len(targets))
		documents := make([]*document, len(targets))
		// No callbacks/interleaving occurs between preflight and these commits.
		for i, target := range targets {
			change, err := target.document.buffer.CommitPrepared(target.file.Prepared)
			if err != nil {
				panic("preflighted UI-owned prepared transaction changed without interleaving: " + err.Error())
			}
			changes[i] = change
			documents[i] = target.document
		}
		m.search.replaceSaving = true
		m.search.replacePlan = nil
		for i, d := range documents {
			m.changed(d, changes[i], nil)
		}
		m.search.replaceStatus = fmt.Sprintf("Saving %d replacements in %d files…", plan.Count, len(documents))
		m.requestSave(context.Background(), documents, func(err error) {
			m.search.replaceSaving = false
			unsaved := 0
			for _, d := range documents {
				if !m.ownsDocument(d) || d.buffer.Dirty() {
					unsaved++
				}
			}
			status := fmt.Sprintf("Replaced %d matches in %d files", plan.Count, len(documents))
			if err != nil || unsaved > 0 {
				status += fmt.Sprintf(" · %d unsaved", unsaved)
				if err != nil {
					status += " · " + err.Error()
				}
			}
			m.search.replaceStatus = status
			m.message = status
			if cx != nil {
				cx.Invalidate()
			}
		})
		if cx != nil {
			cx.Invalidate()
		}
	})
}
