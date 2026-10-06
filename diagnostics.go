package main

import (
	"encoding/json"
	"sort"
)

func (m *model) setDiagnostics(path, channel string, data json.RawMessage) {
	var items []diagnostic
	if json.Unmarshal(data, &items) != nil {
		return
	}
	if m.diagnostics == nil {
		m.diagnostics = map[string][]diagnostic{}
	}
	for i := range items {
		items[i].Path = path
	}
	m.diagnostics[channel+"\x00"+path] = items
}
func (m *model) problems() []diagnostic {
	var all []diagnostic
	for _, items := range m.diagnostics {
		all = append(all, items...)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Path != all[j].Path {
			return all[i].Path < all[j].Path
		}
		return all[i].Range.Start.Line < all[j].Range.Start.Line
	})
	return all
}
func (m *model) diagnosticCounts() (errors, warnings int) {
	for _, items := range m.diagnostics {
		for _, item := range items {
			if item.Severity == 0 {
				errors++
			} else if item.Severity == 1 {
				warnings++
			}
		}
	}
	return
}
