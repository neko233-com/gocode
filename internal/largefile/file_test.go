package largefile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, text string) *File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "text.txt")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestPagesUTF8CRLFAndLongLine(t *testing.T) {
	text := "你😀\r\n" + strings.Repeat("界", BlockBytes*2) + " tail\r\nlast\r\n"
	f := fixture(t, text)
	if err := f.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	s := f.Stats()
	if !s.Complete || s.Lines != 4 || s.EOL != "CRLF" || s.IndexBytes > MaxPoints*24 {
		t.Fatalf("stats %+v", s)
	}
	lines, err := f.Lines(context.Background(), 0, 4)
	if err != nil || len(lines) != 2 {
		t.Fatalf("page %+v %v", lines, err)
	}
	if lines[0].Text != "你😀" || !lines[1].Truncated || len(lines[1].Text) > PreviewBytes {
		t.Fatalf("unexpected page %+v", lines)
	}
	last, err := f.Lines(context.Background(), 2, 2)
	if err != nil || len(last) != 2 || last[0].Text != "last" || last[1].Text != "" {
		t.Fatalf("seek past long line %v %v", last, err)
	}
	// The preview limit cuts through a Chinese codepoint. The retained text is valid.
	if lines[1].Text != strings.Repeat("界", PreviewBytes/3) {
		t.Fatal("UTF-8 preview boundary")
	}
	w, err := f.Bytes(context.Background(), int64(strings.Index(text, " tail"))-2, 64)
	if err != nil || !strings.HasPrefix(w.Text, " tail") || w.Line != 1 {
		t.Fatalf("byte view %+v %v", w, err)
	}
}

func TestSparseRandomLineAndByteCoordinates(t *testing.T) {
	text := strings.Repeat("123😀\n", 300000)
	f := fixture(t, text)
	if err := f.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, line := range []int64{0, 1023, 1024, 100001, 299999, 300000} {
		page, err := f.Lines(context.Background(), line, 1)
		if err != nil || len(page) != 1 || page[0].Number != line || page[0].Offset != line*8 {
			t.Fatalf("line %d %+v %v", line, page, err)
		}
		if line < 300000 && page[0].Text != "123😀" {
			t.Fatal("wrong random line")
		}
		w, err := f.Bytes(context.Background(), line*8, 16)
		if err != nil || w.Line != line || w.ColumnBytes != 0 {
			t.Fatalf("coordinates %+v %v", w, err)
		}
	}
}

func TestIndexCoarsensWithoutGrowing(t *testing.T) {
	f := fixture(t, strings.Repeat("\n", MaxPoints*1024+8192))
	if err := f.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	f.mu.RLock()
	points := len(f.points)
	capacity := cap(f.points)
	f.mu.RUnlock()
	if points >= MaxPoints || capacity != MaxPoints {
		t.Fatalf("index %d/%d", points, capacity)
	}
	page, err := f.Lines(context.Background(), MaxPoints*1024+1, 2)
	if err != nil || len(page) != 2 {
		t.Fatalf("coarsened seek %v", err)
	}
}

func TestInvalidTextAndExternalChange(t *testing.T) {
	for _, text := range []string{"a\x00b", "a\xffb", "a\xe4\xbd"} {
		f := fixture(t, text)
		if err := f.Wait(context.Background()); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	f := fixture(t, "old\n")
	if err := f.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.path, []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Lines(context.Background(), 0, 1); !errors.Is(err, ErrChanged) {
		t.Fatalf("mutation %v", err)
	}
}

func TestCancellationAndInputBounds(t *testing.T) {
	f := fixture(t, strings.Repeat("long", 2<<20))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := f.Lines(ctx, 0, 1); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := f.Bytes(context.Background(), -1, 4); err == nil {
		t.Fatal("negative offset")
	}
	if _, err := f.Bytes(context.Background(), 0, MaxWindowBytes+1); err == nil {
		t.Fatal("unbounded window")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Bytes(context.Background(), 0, 4); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// Opt-in measured acceptance writes actual UTF-8 text, including a huge single
// line. It is separate from fast CI fixtures and never uses a sparse NUL file.
func TestGiBBrowsing(t *testing.T) {
	if os.Getenv("GOCODE_LARGEFILE_GIB") != "1" {
		t.Skip("set GOCODE_LARGEFILE_GIB=1 for real 1 GiB acceptance")
	}
	path := filepath.Join(t.TempDir(), "gib.txt")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	block := strings.Repeat("0123456789abcdef", 4096) // 64 KiB per line.
	for i := 0; i < 16384; i++ {
		if _, err := file.WriteString(block + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	f, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	page, err := f.Lines(context.Background(), 0, 8)
	first := time.Since(start)
	if err != nil || len(page) != 8 {
		t.Fatalf("first page %v", err)
	}
	if err := f.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	indexed := time.Since(start)
	for _, line := range []int64{8192, 16383, 16384} {
		if _, err := f.Lines(context.Background(), line, 1); err != nil {
			t.Fatal(err)
		}
	}
	w, err := f.Bytes(context.Background(), f.Stats().Size/2, MaxWindowBytes)
	if err != nil || len(w.Text) == 0 {
		t.Fatalf("middle %v", err)
	}
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > 16<<20 {
		t.Fatalf("index and random reads allocated %d bytes for a 1 GiB file", allocated)
	}
	t.Logf("actual bytes=%d first-page=%s full-index=%s fixed-index-bytes=%d max-preview-bytes=%d total-index-and-read-allocation=%d", f.Stats().Size, first, indexed, f.Stats().IndexBytes, MaxRows*PreviewBytes, allocated)
	// A second pass replaces the newlines to create one actual 1 GiB line.
	f.Close()
	file, err = os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 16384; i++ {
		if _, err := file.WriteAt([]byte(" "), int64(i+1)*65537-1); err != nil {
			t.Fatal(err)
		}
	}
	file.Close()
	f, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.Stats().Lines != 1 {
		t.Fatal("not a single-line fixture")
	}
	start = time.Now()
	for i := 0; i < 100; i++ {
		w, err = f.Bytes(context.Background(), f.Stats().Size-8192, 8192)
		if err != nil {
			t.Fatal(err)
		}
	}
	if err != nil || len(w.Text) != 8192 || w.Line != 0 {
		t.Fatalf("single-line tail %+v %v", w, err)
	}
	t.Logf("1 GiB single-line 100 direct-tail reads=%s column-bytes=%d", time.Since(start), w.ColumnBytes)
}
