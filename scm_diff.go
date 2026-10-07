package main

import (
	"errors"
	"strconv"
	"strings"

	gitrepo "github.com/neko233-com/gocode/internal/git"
)

type diffRow struct {
	left, right      string
	oldLine, newLine int
	removed, added   bool
}

func buildDiffRows(diff gitrepo.Diff) ([]diffRow, error) {
	if diff.Binary {
		return nil, nil
	}
	if strings.Count(diff.Before, "\n") > 65536 || strings.Count(diff.After, "\n") > 65536 || strings.Count(diff.Patch, "\n") > 131072 {
		return nil, errors.New("diff exceeds 65536 lines; open the file for bounded browsing")
	}
	lines := func(text string) []string {
		if text == "" {
			return nil
		}
		value := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
		for i := range value {
			value[i] = strings.TrimSuffix(value[i], "\r")
		}
		return value
	}
	left, right := lines(diff.Before), lines(diff.After)
	rows := make([]diffRow, 0, max(len(left), len(right)))
	old, next := 0, 0
	appendPlain := func(oldEnd, newEnd int) error {
		if oldEnd < old || newEnd < next || oldEnd > len(left) || newEnd > len(right) {
			return errors.New("invalid Git diff coordinates")
		}
		for old < oldEnd || next < newEnd {
			row := diffRow{}
			if old < oldEnd {
				row.left = left[old]
				old++
				row.oldLine = old
			}
			if next < newEnd {
				row.right = right[next]
				next++
				row.newLine = next
			}
			rows = append(rows, row)
		}
		return nil
	}
	patch := strings.Split(diff.Patch, "\n")
	for index := 0; index < len(patch); index++ {
		if !strings.HasPrefix(patch[index], "@@ ") {
			continue
		}
		parts := strings.Fields(patch[index])
		if len(parts) < 4 {
			return nil, errors.New("invalid Git hunk")
		}
		coordinate := func(value string) (int, error) {
			value, count, _ := strings.Cut(value[1:], ",")
			line, err := strconv.Atoi(value)
			if err != nil || line < 0 {
				return 0, errors.New("invalid Git hunk position")
			}
			if count == "0" {
				return line, nil
			}
			return max(0, line-1), nil
		}
		oldStart, err := coordinate(parts[1])
		if err != nil {
			return nil, err
		}
		newStart, err := coordinate(parts[2])
		if err != nil {
			return nil, err
		}
		if err := appendPlain(oldStart, newStart); err != nil {
			return nil, err
		}
		for index+1 < len(patch) && !strings.HasPrefix(patch[index+1], "@@ ") {
			index++
			line := patch[index]
			if line == "" || line[0] == '\\' {
				continue
			}
			if line[0] == ' ' {
				if err := appendPlain(old+1, next+1); err != nil {
					return nil, err
				}
				continue
			}
			if line[0] != '-' && line[0] != '+' {
				break
			}
			removed, added := 0, 0
			for {
				value := patch[index]
				if len(value) > 0 && value[0] == '-' {
					removed++
				} else if len(value) > 0 && value[0] == '+' {
					added++
				} else if len(value) == 0 || value[0] != '\\' {
					break
				}
				if index+1 >= len(patch) || len(patch[index+1]) == 0 || (patch[index+1][0] != '-' && patch[index+1][0] != '+' && patch[index+1][0] != '\\') {
					break
				}
				index++
			}
			if old+removed > len(left) || next+added > len(right) {
				return nil, errors.New("Git hunk exceeds file")
			}
			for offset := range max(removed, added) {
				row := diffRow{removed: offset < removed, added: offset < added}
				if offset < removed {
					row.left = left[old]
					old++
					row.oldLine = old
				}
				if offset < added {
					row.right = right[next]
					next++
					row.newLine = next
				}
				rows = append(rows, row)
			}
		}
	}
	if err := appendPlain(len(left), len(right)); err != nil {
		return nil, err
	}
	if len(rows) > 131072 {
		return nil, errors.New("diff row limit exceeded")
	}
	return rows, nil
}
