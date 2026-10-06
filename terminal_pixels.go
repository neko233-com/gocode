package main

import "math"

// Gray glyph coverage blends foreground with the terminal background. Small
// fonts may have no fully opaque pixels; color direction still distinguishes
// shell token colors from neutral text, backgrounds and other token colors.
func matchesTerminalInk(pixel, bg, fg uint32) bool {
	channel := func(value uint32, shift uint) float64 { return float64((value >> shift) & 255) }
	alpha := (channel(pixel, 16) - channel(bg, 16)) / (channel(fg, 16) - channel(bg, 16))
	if alpha < .30 || alpha > 1.02 {
		return false
	}
	for _, shift := range []uint{16, 8, 0} {
		expected := channel(bg, shift) + alpha*(channel(fg, shift)-channel(bg, shift))
		if math.Abs(channel(pixel, shift)-expected) > 3 {
			return false
		}
	}
	return true
}
