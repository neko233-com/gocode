package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWorkspaceScannerBoundsAndExcludedDirectories(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{".git", ".cache", "node_modules", "vendor", "bin", "nested"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, dir, "main.go"), []byte("package main"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("readme"), 0600); err != nil {
		t.Fatal(err)
	}
	r := scanWorkspace(context.Background(), root)
	if r.err != nil || r.limited || r.skipped != 0 || !reflect.DeepEqual(r.files, []string{"README.md", "nested/main.go"}) || preferredFile(r.files) != "nested/main.go" {
		t.Fatal("scan/selection incorrect", r)
	}
	for i := range maxWorkspaceFiles + 10 {
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("f%03d", i)), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	r = scanWorkspace(context.Background(), root)
	if r.err != nil || !r.limited || len(r.files) != maxWorkspaceFiles || !strings.Contains(r.status(), "scan limit reached") {
		t.Fatal("file count bound not visible", r.status())
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r = scanWorkspace(ctx, root)
	if !errors.Is(r.err, context.Canceled) || len(r.files) != 0 {
		t.Fatal("cancelled traversal ran", r)
	}
	if r := scanWorkspace(context.Background(), filepath.Join(root, "missing")); r.err == nil {
		t.Fatal("missing root hidden")
	}
}

func TestWorkspaceScannerDirectoryQueueAndDepthBounds(t *testing.T) {
	root := t.TempDir()
	for i := range maxWorkspaceDirectories + 10 {
		if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("d%03d", i)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	r := scanWorkspace(context.Background(), root)
	if r.err != nil || !r.limited || len(r.files) != 0 {
		t.Fatal("directory queue unbounded", r)
	}
	root = t.TempDir()
	path := root
	for range maxWorkspaceDepth + 2 {
		path = filepath.Join(path, "d")
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(path, "deep.txt"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	r = scanWorkspace(context.Background(), root)
	if r.err != nil || !r.limited || len(r.files) != 0 {
		t.Fatal("depth bound failed", r)
	}
}

func TestWorkspaceLateScanKeepsNewNavigation(t *testing.T) {
	m, err := newWorkspaceModel(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(m.workspace, "user.go")
	if err := os.WriteFile(path, []byte("package user"), 0600); err != nil {
		t.Fatal(err)
	}
	mailbox := openActor(t, m, nil)
	scanbox := make(chan func(), 2)
	var ready bool
	stop := m.startWorkspace(context.Background(), func(fn func()) bool { scanbox <- fn; return true }, nil, "", func(ctx context.Context, root string) workspaceScan { return workspaceScan{files: []string{"main.go"}} }, func(err error) {
		if err != nil {
			t.Error(err)
		}
		ready = true
	})
	t.Cleanup(stop)
	scanAck := saveAck(t, scanbox)
	m.open(path)
	drainOpens(t, m, mailbox)
	d := m.current()
	text(m, "// keep\n")
	scanAck()
	if !ready || m.workspaceBusy || m.current() != d || !d.dirty() || len(m.docs) != 1 {
		t.Fatal("late scan started obsolete initial open")
	}
}

func TestWorkspaceExplicitFilesAwaitAndGotoAfterOpen(t *testing.T) {
	m := testModel(t)
	mailbox := openActor(t, m, nil)
	scanbox := make(chan func(), 2)
	var ready bool
	stop := m.startWorkspace(context.Background(), func(fn func()) bool { scanbox <- fn; return true }, []string{"README.md", "main.go"}, "3", nil, func(err error) {
		if err != nil {
			t.Error(err)
		}
		ready = true
	})
	t.Cleanup(stop)
	// The cached final CLI file can finish before the queued first read. The
	// barrier still waits for all jobs; stale first focus cannot steal it.
	if ready {
		t.Fatal("startup completed before disk transfer")
	}
	drainOpens(t, m, mailbox)
	saveAck(t, scanbox)()
	if !ready || m.current().path != filepath.Join(m.workspace, "main.go") || m.current().line != 2 || len(m.docs) != 2 {
		t.Fatal("CLI focus/goto/barrier incorrect")
	}
}

func TestWorkspaceShutdownRejectsPendingUIResult(t *testing.T) {
	m, err := newWorkspaceModel(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mailbox := make(chan func(), 2)
	stop := m.startWorkspace(context.Background(), func(fn func()) bool { mailbox <- fn; return true }, nil, "", func(context.Context, string) workspaceScan { return workspaceScan{files: []string{"late.go"}} }, func(error) { t.Error("shutdown signalled startup ready") })
	ack := saveAck(t, mailbox)
	stop()
	ack()
	if len(m.files) != 0 || m.current() != nil {
		t.Fatal("pending scan mutated closed UI")
	}
}
