package search

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/neko233-com/godesktop/editor"
)

func TestReaderRegexMatchesStandardOracle(t *testing.T) {
	texts := []string{"a abc aaaa\nabc\n", "😀a界 a界\r\nABC abc\nend", "", "aaa", "a\rb", strings.Repeat("x", 65534) + "😀needle😀\n"}
	patterns := []string{"a", "a*", "a*?", "a+|b", "^a", "a$", "\\A.", "\\b[a-z]+\\b", "(?s)a.*c", "(?s).*", "^|$", "😀|needle|界", "a|"}
	for _, text := range texts {
		for _, p := range patterns {
			q := Query{Text: p, Regex: true, CaseSensitive: true}
			e, err := compile(q)
			if err != nil {
				t.Fatal(err)
			}
			var report Report
			err = searchSource(context.Background(), strings.NewReader(text), int64(len(text)), "x", e, 0, 0, &report)
			if err != nil {
				t.Fatal(err)
			}
			var got [][2]int
			for _, m := range report.Matches {
				got = append(got, [2]int{int(m.Start), int(m.End)})
			}
			var want [][2]int
			for _, m := range regexp.MustCompile("(?m:"+p+")").FindAllStringIndex(text, MaxResults) {
				want = append(want, [2]int{m[0], m[1]})
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("pattern=%q text=%q got=%v want=%v", p, text[:min(80, len(text))], got, want)
			}
		}
	}
}

func TestLiteralChunkBoundaryUnicodeCRLFAndWholeWord(t *testing.T) {
	text := strings.Repeat("x", 65532) + "\n😀界needle😀\r\nNEEDLE needlex needle\n"
	for _, sensitive := range []bool{true, false} {
		e, err := compile(Query{Text: "needle", CaseSensitive: sensitive, WholeWord: true})
		if err != nil {
			t.Fatal(err)
		}
		var report Report
		if err := searchSource(context.Background(), strings.NewReader(text), int64(len(text)), "x", e, 0, 0, &report); err != nil {
			t.Fatal(err)
		}
		want := 1
		if !sensitive {
			want = 2
		}
		if len(report.Matches) != want {
			t.Fatalf("sensitive=%t matches=%+v", sensitive, report.Matches)
		}
		last := report.Matches[len(report.Matches)-1]
		if last.Range.Start.Line != 2 || last.Range.Start.Character != 15 {
			t.Fatal(last.Range)
		}
	}
	boundary := strings.Repeat("x", 65534) + "needle" + strings.Repeat("z", 70000) + "needle"
	e, _ := compile(Query{Text: "needle", CaseSensitive: true})
	var report Report
	if err := searchSource(context.Background(), strings.NewReader(boundary), int64(len(boundary)), "x", e, 0, 0, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Matches) != 2 || report.Matches[0].Start != 65534 || report.Matches[1].Start != 135540 {
		t.Fatal(report.Matches)
	}
	u := "prefix 😀界 needle\r\nsecond needle"
	e, _ = compile(Query{Text: "needle", CaseSensitive: true})
	report = Report{}
	if err := searchSource(context.Background(), strings.NewReader(u), int64(len(u)), "x", e, 0, 0, &report); err != nil {
		t.Fatal(err)
	}
	if report.Matches[0].Range.Start != (editor.Position{Line: 0, Character: 11}) || report.Matches[1].Range.Start != (editor.Position{Line: 1, Character: 7}) {
		t.Fatal(report.Matches)
	}
	if report.Matches[0].Hash != sha256.Sum256([]byte("needle")) {
		t.Fatal("match digest")
	}
}

func write(t *testing.T, root, p, text string) {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func TestWorkspaceIgnoresFiltersOverlaysAndBinary(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"main.go", "nested/main.go", "nested/keep.go", "ignored/secret.go", "阅读说明.md", "binary.dat"} {
		write(t, root, p, "disk needle\n")
	}
	write(t, root, ".gitignore", "ignored/\nnested/*.go\n!nested/keep.go\n")
	write(t, root, "nested/.gitignore", "main.go\n")
	write(t, root, "binary.dat", "\x00needle")
	b, _ := editor.New("😀 unsaved needle\r\n")
	report := Run(context.Background(), root, Query{Text: "needle", CaseSensitive: true, UseIgnore: true, Include: "**/*.{go,md}"}, map[string]Overlay{"nested/main.go": {b.Snapshot(), 17}})
	if report.Err != nil {
		t.Fatal(report.Err)
	}
	var paths []string
	for _, m := range report.Matches {
		paths = append(paths, m.Path)
	}
	if !reflect.DeepEqual(paths, []string{"main.go", "nested/keep.go", "nested/main.go", "阅读说明.md"}) {
		t.Fatal(paths)
	}
	m := report.Matches[2]
	if m.Version != b.Version() || m.Identity != 17 || m.Range.Start.Character != 11 || !strings.Contains(m.Preview, "unsaved") {
		t.Fatal(m)
	}
	report = Run(context.Background(), root, Query{Text: "needle", CaseSensitive: true, UseIgnore: false, Exclude: "**/nested/**,阅读*.md"}, nil)
	if report.Err != nil || len(report.Matches) != 2 || report.Binary != 1 {
		t.Fatalf("report=%+v", report)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r := Run(ctx, root, Query{Text: "needle"}, nil); r.Err != context.Canceled {
		t.Fatal(r.Err)
	}
}

func TestGitIgnoreOracle(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git oracle unavailable")
	}
	root := t.TempDir()
	cmd := exec.Command("git", "init", "-q", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	rules := "*.tmp\n!keep.tmp\n/foo\nbuild/**\n!build/keep.go\na/**/b.go\n[!a]file\n\\#literal\n\\!literal\nspace\\ \n阅读*.md\n"
	write(t, root, ".gitignore", rules)
	paths := []string{"x.tmp", "deep/x.tmp", "keep.tmp", "deep/keep.tmp", "foo", "deep/foo", "build/x.go", "build/keep.go", "a/b.go", "a/x/y/b.go", "bfile", "afile", "#literal", "!literal", "space ", "阅读说明.md", "deep/阅读说明.md"}
	fs, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer fs.Close()
	parsed, err := readRules(fs, "", ".gitignore")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		cmd := exec.Command("git", "-C", root, "check-ignore", "--no-index", "--", p)
		out, err := cmd.Output()
		want := err == nil && len(out) > 0
		if got := ignored(parsed, p, false); got != want {
			t.Fatalf("%s got=%t git=%t (%s)", p, got, want, out)
		}
	}
}

func TestStreamingLimitCancellationAndGiB(t *testing.T) {
	size := int64(64 << 20)
	if os.Getenv("GOCODE_SEARCH_GIB") == "1" {
		size = 1 << 30
	}
	root := t.TempDir()
	name := filepath.Join(root, "huge.txt")
	f, err := os.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	block := bytes.Repeat([]byte("x"), 128<<10)
	for off := int64(0); off < size; off += int64(len(block)) {
		if _, err := f.Write(block); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.WriteAt([]byte("😀needle"), size-256); err != nil {
		t.Fatal(err)
	}
	f.Close()
	start := time.Now()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	r := Run(context.Background(), root, Query{Text: "needle", CaseSensitive: true}, nil)
	runtime.ReadMemStats(&after)
	if r.Err != nil || len(r.Matches) != 1 || r.Matches[0].Start != size-252 || r.Matches[0].Range.Start.Line != 0 || r.Matches[0].Range.Start.Character != int(size-254) {
		t.Fatalf("report=%+v", r)
	}
	if after.TotalAlloc-before.TotalAlloc > 64<<20 {
		t.Fatalf("streaming search allocated %d bytes", after.TotalAlloc-before.TotalAlloc)
	}
	t.Logf("actual single-line bytes=%d elapsed=%s I/O=%d Go-allocation=%d", size, time.Since(start), r.Bytes, after.TotalAlloc-before.TotalAlloc)
	write(t, root, "many.txt", strings.Repeat("needle ", MaxResults+5))
	r = Run(context.Background(), root, Query{Text: "needle", CaseSensitive: true, Include: "many.txt"}, nil)
	if len(r.Matches) != MaxResults || !r.Limited {
		t.Fatal(len(r.Matches), r.Limited)
	}
	ctx, cancel := context.WithCancel(context.Background())
	e, _ := compile(Query{Text: "(?s)never.*end", Regex: true})
	cancel()
	var out Report
	if err := searchSource(ctx, strings.NewReader(strings.Repeat("a", 1000)), 1000, "x", e, 0, 0, &out); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestUnicodeFoldedLiteralMatchesAndStaleCoordinates(t *testing.T) {
	text := "K k K S s ſ Σ σ ς 😀needle\r\n" + strings.Repeat("x", 65533) + "Kneedleſ"
	for _, query := range []string{"k", "s", "σ", "needle", "Kneedleſ"} {
		e, err := compile(Query{Text: query})
		if err != nil {
			t.Fatal(err)
		}
		var report Report
		if err := searchSource(context.Background(), strings.NewReader(text), int64(len(text)), "x", e, 0, 0, &report); err != nil {
			t.Fatal(err)
		}
		want := regexp.MustCompile("(?mi:"+regexp.QuoteMeta(query)+")").FindAllStringIndex(text, MaxResults)
		if len(want) != len(report.Matches) {
			t.Fatal(query, len(want), len(report.Matches))
		}
		for i, m := range report.Matches {
			if int(m.Start) != want[i][0] || int(m.End) != want[i][1] {
				t.Fatal(query, m.Start, m.End, want[i])
			}
		}
	}
	text = "😀needle"
	e, _ := compile(Query{Text: "needle"})
	var report Report
	_ = searchSource(context.Background(), strings.NewReader(text), int64(len(text)), "x", e, 0, 0, &report)
	m := report.Matches[0]
	if err := Verify(context.Background(), strings.NewReader(text), int64(len(text)), m); err != nil {
		t.Fatal(err)
	}
	changed := "xxxxneedle"
	if err := Verify(context.Background(), strings.NewReader(changed), int64(len(changed)), m); err == nil {
		t.Fatal("same byte offset/hash with changed UTF-16 prefix accepted")
	}
	if _, err := compile(Query{Text: "(a{1000}){1000}", Regex: true}); err == nil {
		t.Fatal("unbounded regex expansion accepted")
	}
}

func FuzzReaderMatches(f *testing.F) {
	f.Add("a b\n😀a", "a|b")
	f.Add("abc\nabc", "^.*$")
	f.Add("aaa", "a*?")
	f.Fuzz(func(t *testing.T, text, pattern string) {
		if len(text) > 4096 || len(pattern) > 128 || pattern == "" || strings.ContainsRune(text, 0) {
			return
		}
		q := Query{Text: pattern, Regex: true, CaseSensitive: true}
		e, err := compile(q)
		if err != nil {
			return
		}
		var r Report
		if err := searchSource(context.Background(), strings.NewReader(text), int64(len(text)), "x", e, 0, 0, &r); err != nil {
			return
		}
		want := regexp.MustCompile("(?m:"+pattern+")").FindAllStringIndex(text, MaxResults)
		if len(r.Matches) != len(want) {
			t.Fatal(fmt.Sprintf("%q", text), pattern, len(r.Matches), len(want))
		}
		for i, m := range r.Matches {
			if int(m.Start) != want[i][0] || int(m.End) != want[i][1] {
				t.Fatal(i, m.Start, m.End, want[i])
			}
		}
	})
}
