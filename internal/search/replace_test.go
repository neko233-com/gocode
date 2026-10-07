package search

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/godesktop/editor"
)

func TestReplacementPreviewsUnsavedAndClosedUTF16CRLF(t *testing.T) {
	root := t.TempDir()
	disk := "😀 needle\r\nneedle end\r\n"
	for _, path := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(disk), 0600); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := editor.New("unsaved 😀 needle\r\nneedle end\r\n")
	overlays := map[string]Overlay{"a.txt": {Snapshot: b.Snapshot(), Identity: 4}}
	q := Query{Text: "needle", CaseSensitive: true}
	report := Run(context.Background(), root, q, overlays)
	plan, err := PrepareReplacement(context.Background(), root, q, report, "界😀\nnew", overlays)
	if err != nil || plan.Count != 4 || len(plan.Files) != 2 {
		t.Fatalf("preview %+v %v", plan, err)
	}
	if b.Text() != overlays["a.txt"].Snapshot.Text() || b.Dirty() {
		t.Fatal("preview mutated the live buffer")
	}
	for _, file := range plan.Files {
		if !strings.Contains(file.Prepared.Snapshot().Text(), "界😀\r\nnew") {
			t.Fatal("UTF-16/EOL replacement preview is incorrect")
		}
		target := b
		if file.Path == "b.txt" {
			target = file.New
			if file.DiskHash != sha256.Sum256([]byte(disk)) {
				t.Fatal("source hash missing")
			}
		}
		if _, err := target.CommitPrepared(file.Prepared); err != nil {
			t.Fatal(err)
		}
		if !target.Dirty() {
			t.Fatal("committed replacement is not undoable/dirty")
		}
		if _, ok := target.Undo(); !ok || target.Text() != file.Before.Text() {
			t.Fatal("preview commit lost original undo source")
		}
		data, _ := os.ReadFile(filepath.Join(root, file.Path))
		if string(data) != disk {
			t.Fatal("preview or buffer commit wrote source")
		}
	}
}

func TestReplacementCaptureExpansionAgainstJavaScript(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node oracle unavailable")
	}
	source := "😀 aaB\r\naaa c aa\n"
	patterns := []struct{ pattern, value string }{
		{`(a+)(B)?`, `$2:$1:$10:$$:$&`}, {`(?P<word>a+)`, `$<word>/$1-tail`}, {`(a+)`, "$`[$1]$'"}, {`(a+)(B)?`, "$99/$2/end"},
	}
	for _, test := range patterns {
		t.Run(test.value, func(t *testing.T) {
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "a"), []byte(source), 0600)
			q := Query{Text: test.pattern, CaseSensitive: true, Regex: true}
			report := Run(context.Background(), root, q, nil)
			plan, err := PrepareReplacement(context.Background(), root, q, report, test.value, nil)
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command("node", "-e", `const x=JSON.parse(require('fs').readFileSync(0,'utf8')); process.stdout.write(JSON.stringify(x.source.replace(new RegExp(x.pattern,'gmu'),x.value)));`)
			input, _ := json.Marshal(map[string]string{"source": source, "pattern": strings.ReplaceAll(test.pattern, "?P<", "?<"), "value": test.value})
			command.Stdin = strings.NewReader(string(input))
			raw, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("Node oracle %s: %v", raw, err)
			}
			var want string
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			oracle, _ := editor.New(want) // Existing editor normalizes mixed line endings.
			if got := plan.Files[0].Prepared.Snapshot().Text(); got != oracle.Text() {
				t.Fatalf("capture expansion differs from JS: got %q want %q", got, oracle.Text())
			}
		})
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a"), []byte("aBc abc"), 0600)
	q := Query{Text: `(a)(Bc|bc)`, Regex: true, CaseSensitive: true}
	report := Run(context.Background(), root, q, nil)
	plan, err := PrepareReplacement(context.Background(), root, q, report, `\u$1-\L$2\n$0\t\\`, nil)
	if err != nil || plan.Files[0].Prepared.Snapshot().Text() != "A-bc\naBc\t\\ A-bc\nabc\t\\" {
		t.Fatalf("case/escapes/whole match: %+v %v", plan, err)
	}
	literal := Query{Text: "abc", CaseSensitive: true}
	report = Run(context.Background(), root, literal, nil)
	plan, err = PrepareReplacement(context.Background(), root, literal, report, `$1\n`, nil)
	if err != nil || plan.Files[0].Prepared.Snapshot().Text() != `aBc $1\n` {
		t.Fatal("literal replacement parsed regex syntax", err)
	}
}

func TestReplacementRejectsStaleIncompleteAndBoundedOutput(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a")
	os.WriteFile(path, []byte("😀 needle"), 0600)
	q := Query{Text: "needle", CaseSensitive: true}
	report := Run(context.Background(), root, q, nil)
	for _, bad := range []Report{{Matches: report.Matches, Limited: true}, {Matches: report.Matches, Unreadable: 1}, {Matches: report.Matches, Err: context.Canceled}, {}} {
		if _, err := PrepareReplacement(context.Background(), root, q, bad, "new", nil); err == nil {
			t.Fatal("incomplete replacement accepted")
		}
	}
	os.WriteFile(path, []byte("xxx needle"), 0600) // Same byte offset; different UTF-16 prefix.
	if _, err := PrepareReplacement(context.Background(), root, q, report, "new", nil); err == nil {
		t.Fatal("stale coordinates accepted")
	}
	os.WriteFile(path, []byte("😀 needle needle"), 0600)
	if _, err := PrepareReplacement(context.Background(), root, q, report, "new", nil); err == nil {
		t.Fatal("newly added match missed by replace-all")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PrepareReplacement(ctx, root, q, report, "new", nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("needle")
	f.Truncate(MaxReplaceFileBytes + 1)
	f.Close()
	report = Run(context.Background(), root, q, nil)
	if _, err := PrepareReplacement(context.Background(), root, q, report, "new", nil); err == nil {
		t.Fatal("read-only large file entered replacement")
	}
	os.WriteFile(path, []byte(strings.Repeat("a", 500)), 0600)
	q = Query{Text: `(a)`, CaseSensitive: true, Regex: true}
	report = Run(context.Background(), root, q, nil)
	if _, err := PrepareReplacement(context.Background(), root, q, report, strings.Repeat("$'", 2000), nil); err == nil {
		t.Fatal("expanded output escaped preview limit")
	}
}
