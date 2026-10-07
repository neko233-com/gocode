package terminal

import (
	"runtime"
	"strings"
)

// ProcessEnvironment supplies terminal defaults only when not explicitly set.
// Callers applying extension overrides afterwards can delete these defaults.
func ProcessEnvironment(environment []string, strict bool) []string {
	result := append([]string{}, environment...)
	if strict {
		return result
	}
	keyOf := func(key string) string {
		if runtime.GOOS == "windows" {
			return strings.ToUpper(key)
		}
		return key
	}
	keys := map[string]bool{}
	for _, entry := range environment {
		if key, _, ok := strings.Cut(entry, "="); ok {
			keys[keyOf(key)] = true
		}
	}
	for _, entry := range []string{"TERM=xterm-256color", "COLORTERM=truecolor", "TERM_PROGRAM=gocode"} {
		key, _, _ := strings.Cut(entry, "=")
		if !keys[keyOf(key)] {
			result = append(result, entry)
		}
	}
	return result
}
