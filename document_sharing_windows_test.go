//go:build windows

package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWindowsSaveSharingRetryRechecksDiskAndCancellation(t *testing.T) {
	for _, mode := range []string{"release", "external", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			m := testModel(t)
			d := m.current()
			original := d.buffer.Text()
			text(m, "local edit")
			f, err := os.Open(d.path)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				if mode == "cancel" {
					time.Sleep(35 * time.Millisecond)
					cancel()
				}
				time.Sleep(80 * time.Millisecond)
				var failure error
				if mode == "external" {
					failure = os.WriteFile(d.path, []byte("external during blocked rename"), 0600)
				}
				failure = errors.Join(failure, f.Close())
				done <- failure
			}()
			_, err = writeDocumentSnapshot(ctx, d.path, d.buffer.Snapshot(), &d.diskHash)
			if closeErr := <-done; closeErr != nil {
				t.Fatal(closeErr)
			}
			data, readErr := os.ReadFile(d.path)
			if readErr != nil {
				t.Fatal(readErr)
			}
			switch mode {
			case "release":
				if err != nil || string(data) != d.buffer.Text() {
					t.Fatal("transient read lock prevented save", err)
				}
			case "external":
				if err == nil || !strings.Contains(err.Error(), "changed on disk") || string(data) != "external during blocked rename" {
					t.Fatal("retry overwrote a newer external revision", err)
				}
			case "cancel":
				if !errors.Is(err, context.Canceled) || string(data) != original {
					t.Fatal("cancelled retry committed", err)
				}
			}
			if !d.dirty() {
				t.Fatal("worker mutated UI dirty state")
			}
		})
	}
}
