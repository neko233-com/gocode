package git

import (
	"strconv"
	"strings"
	"testing"
)

func TestPorcelainLiteralRecordsAndMalformedBounds(t *testing.T) {
	oid := strings.Repeat("a", 40)
	raw := "# branch.oid " + oid + "\x00# branch.head main\x00# branch.upstream origin/main\x00# branch.ab +2 -3\x002 R. N... 100644 100644 100644 " + oid + " " + oid + " R100 new\t世界😀.go\x00old\nname.go\x00? literal [*].go\x00! ignored.go\x00"
	state, err := parseStatus([]byte(raw))
	if err != nil || len(state.Entries) != 2 || state.Ahead != 2 || state.Behind != 3 || state.Upstream != "origin/main" {
		t.Fatal(state, err)
	}
	if state.Entries[1].Path != "new\t世界😀.go" || state.Entries[1].OriginalPath != "old\nname.go" {
		t.Fatal(state.Entries)
	}
	for _, invalid := range []string{"? ../escape\x00", "? same\x00? same\x00", "? file", "2 R. N... 100644 100644 100644 " + oid + " " + oid + " R100 new\x00", "# branch.oid invalid\x00", "# branch.ab +1 -invalid\x00", "u UU N... 100644 100644 100644 100644 bad bad bad file\x00", "1 M. N... 100644 100644 100644 bad bad file\x00"} {
		if _, err := parseStatus([]byte(invalid)); err == nil {
			t.Fatalf("accepted malformed %q", invalid)
		}
	}
	var resources strings.Builder
	for i := 0; i <= MaxEntries; i++ {
		resources.WriteString("? file" + strconv.Itoa(i) + "\x00")
	}
	if _, err := parseStatus([]byte(resources.String())); err == nil {
		t.Fatal("accepted excessive resources")
	}
}

func FuzzPorcelainStatus(f *testing.F) {
	f.Add([]byte("? 世界😀.go\x00"))
	f.Add([]byte("# branch.oid (initial)\x00# branch.head main\x00"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		state, err := parseStatus(raw)
		if err != nil {
			return
		}
		if len(state.Entries) > MaxEntries {
			t.Fatal("unbounded resources")
		}
		seen := map[string]bool{}
		for _, entry := range state.Entries {
			if !validPath(entry.Path) || seen[entry.Path] {
				t.Fatal("invalid accepted resource")
			}
			seen[entry.Path] = true
		}
	})
}
