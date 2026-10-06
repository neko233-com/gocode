package main

import (
	"context"
	"errors"
	"path/filepath"

	textbuffer "github.com/neko233-com/godesktop/editor"
)

// Pure model tests exercise the disk writer synchronously. Production routes all
// native keys, extension saves and close saves through the acknowledged actor.
func (m *model) save() error {
	d := m.current()
	if d == nil || d.buffer == nil {
		return errors.New("no editable document; large-file browsing is read-only")
	}
	var expected *[32]byte
	if d.diskKnown {
		value := d.diskHash
		expected = &value
	}
	hash, err := writeDocumentSnapshot(context.Background(), d.path, d.buffer.Snapshot(), expected)
	if err != nil {
		return err
	}
	d.diskHash = hash
	d.diskKnown = true
	d.buffer.MarkSaved()
	m.documentEvent("save", d, textbuffer.ChangeEvent{})
	m.message = "Saved " + filepath.Base(d.path)
	return nil
}
