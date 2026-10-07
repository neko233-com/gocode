package git

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxEntries = 16384

type Entry struct {
	Path, OriginalPath  string
	Index, Worktree     byte
	Submodule           string
	HeadBlob, IndexBlob string
	Conflict            bool
}

func (e Entry) Staged() bool  { return e.Index != '.' && e.Index != '?' && !e.Conflict }
func (e Entry) Changed() bool { return e.Worktree != '.' || e.Index == '?' || e.Conflict }

type State struct {
	Root, Branch, Head, Upstream string
	Ahead, Behind                int
	Entries                      []Entry
	IndexToken                   [32]byte
}

func validPath(path string) bool {
	if path == "" || len(path) > 8192 || !utf8.ValidString(path) || strings.ContainsRune(path, 0) || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return false
	}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
func validOID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func emptyOID(value string) bool { return value == "" || strings.Trim(value, "0") == "" }

func validXY(value string) bool {
	return len(value) == 2 && strings.ContainsRune(".MADRCUT", rune(value[0])) && strings.ContainsRune(".MADRCUT", rune(value[1]))
}
func validMode(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, c := range value {
		if c < '0' || c > '7' {
			return false
		}
	}
	return true
}
func validSubmodule(value string) bool {
	return value == "N..." || len(value) == 4 && value[0] == 'S' && strings.ContainsRune(".C", rune(value[1])) && strings.ContainsRune(".M", rune(value[2])) && strings.ContainsRune(".U", rune(value[3]))
}

// Porcelain v2 -z keeps spaces, tabs/newlines, Unicode and rename pairs literal.
func parseStatus(raw []byte) (State, error) {
	state := State{Entries: make([]Entry, 0)}
	if len(raw) > 4<<20 {
		return state, ErrOutputLimit
	}
	if len(raw) > 0 && raw[len(raw)-1] != 0 {
		return state, errors.New("incomplete Git status record")
	}
	records := bytes.Split(raw, []byte{0})
	seen := map[string]bool{}
	for i := 0; i < len(records)-1; i++ {
		record := string(records[i])
		if len(record) < 2 {
			return state, errors.New("invalid Git status record")
		}
		if strings.HasPrefix(record, "# ") {
			key, value, ok := strings.Cut(record[2:], " ")
			if !ok {
				return state, errors.New("invalid Git branch header")
			}
			switch key {
			case "branch.oid":
				if value != "(initial)" {
					if !validOID(value) {
						return state, errors.New("invalid HEAD object")
					}
					state.Head = value
				}
			case "branch.head":
				state.Branch = value
			case "branch.upstream":
				state.Upstream = value
			case "branch.ab":
				parts := strings.Fields(value)
				if len(parts) != 2 || !strings.HasPrefix(parts[0], "+") || !strings.HasPrefix(parts[1], "-") {
					return state, errors.New("invalid branch divergence")
				}
				var err error
				state.Ahead, err = strconv.Atoi(parts[0][1:])
				if err != nil {
					return state, err
				}
				state.Behind, err = strconv.Atoi(parts[1][1:])
				if err != nil || state.Ahead < 0 || state.Behind < 0 {
					return state, errors.New("invalid branch divergence")
				}
			}
			continue
		}
		var entry Entry
		switch record[0] {
		case '?', '!':
			if record[1] != ' ' {
				return state, errors.New("invalid untracked record")
			}
			if record[0] == '!' {
				continue
			}
			entry = Entry{Path: record[2:], Index: '?', Worktree: '?'}
		case '1', '2':
			count := 9
			if record[0] == '2' {
				count = 10
			}
			parts := strings.SplitN(record, " ", count)
			if len(parts) != count || !validXY(parts[1]) || !validSubmodule(parts[2]) || !validMode(parts[3]) || !validMode(parts[4]) || !validMode(parts[5]) || !validOID(parts[6]) || !validOID(parts[7]) {
				return state, errors.New("invalid tracked status record")
			}
			entry = Entry{Path: parts[count-1], Index: parts[1][0], Worktree: parts[1][1], Submodule: parts[2], HeadBlob: parts[6], IndexBlob: parts[7]}
			if record[0] == '2' {
				score := parts[8]
				if len(score) < 2 || (score[0] != 'R' && score[0] != 'C') {
					return state, errors.New("invalid rename score")
				}
				n, err := strconv.Atoi(score[1:])
				if err != nil || n < 0 || n > 100 {
					return state, errors.New("invalid rename score")
				}
				i++
				if i >= len(records)-1 {
					return state, errors.New("missing rename origin")
				}
				entry.OriginalPath = string(records[i])
				if !validPath(entry.OriginalPath) {
					return state, errors.New("invalid rename origin")
				}
			}
		case 'u':
			parts := strings.SplitN(record, " ", 11)
			if len(parts) != 11 || !validXY(parts[1]) || !validSubmodule(parts[2]) || !validMode(parts[3]) || !validMode(parts[4]) || !validMode(parts[5]) || !validMode(parts[6]) || !validOID(parts[7]) || !validOID(parts[8]) || !validOID(parts[9]) {
				return state, errors.New("invalid conflict record")
			}
			entry = Entry{Path: parts[10], Index: parts[1][0], Worktree: parts[1][1], Submodule: parts[2], Conflict: true}
		default:
			return state, fmt.Errorf("unknown Git status record %q", record[0])
		}
		if !validPath(entry.Path) || seen[entry.Path] {
			return state, errors.New("invalid or duplicate Git path")
		}
		seen[entry.Path] = true
		state.Entries = append(state.Entries, entry)
		if len(state.Entries) > MaxEntries {
			return state, errors.New("Git status exceeds 16384 changed files")
		}
	}
	sort.Slice(state.Entries, func(i, j int) bool { return state.Entries[i].Path < state.Entries[j].Path })
	return state, nil
}
