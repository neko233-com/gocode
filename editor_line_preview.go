package main

// Render only the already documented rune limit. Decoding the whole source line
// first would allocate tens of MiB even though the native row shows 400 runes.
func codeLinePreview(line string, limit int) []rune {
	count := 0
	for index := range line {
		if count == limit {
			return []rune(line[:index])
		}
		count++
	}
	return []rune(line)
}
func runeCountUpTo(line string, limit int) int {
	count := 0
	for range line {
		if count == limit {
			return count
		}
		count++
	}
	return count
}
