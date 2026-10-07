package main

import (
	"strings"
	"testing"
)

func TestNativeRowPreviewWorkDoesNotScaleWithLongSourceLine(t *testing.T) {
	source := strings.Repeat("😀界", 1<<20)
	var preview []rune
	result := testing.Benchmark(func(b *testing.B) {
		for range b.N {
			preview = codeLinePreview(source, 400)
		}
	})
	if len(preview) != 400 || string(preview) != strings.Repeat("😀界", 200) || result.AllocedBytesPerOp() > 4096 {
		t.Fatal("400-rune native row copies a whole 7 MiB line", result.AllocedBytesPerOp(), len(preview))
	}
	if got := runeCountUpTo(source, 70); got != 70 {
		t.Fatal("minimap count differs", got)
	}
	for _, text := range []string{"", "😀界", "a\t界😀"} {
		if got := string(codeLinePreview(text, 400)); got != text {
			t.Fatal("short Unicode preview changed", got)
		}
	}
}
