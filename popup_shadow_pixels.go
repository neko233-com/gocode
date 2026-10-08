package main

import (
	"fmt"
	"image"
	"math"

	ui "github.com/neko233-com/godesktop"
)

// Acceptance reference only: uniform Simpson integration of an independent
// binary rounded mask against a Gaussian, not the core shader's quadrature.
// Popup styles are bounded here to the measured LG/XL values (sigma 6 or 10).
func popupShadowCoverage(body ui.Bounds, x, y float64, style ui.ShadowStyle) float64 {
	sigma := float64(style.Blur) / 2
	left, top := float64(body.X+style.OffsetX-style.Spread), float64(body.Y+style.OffsetY-style.Spread)
	w, h := float64(body.Width+2*style.Spread), float64(body.Height+2*style.Spread)
	r := min(float64(8+style.Spread), min(w, h)/2)
	if sigma <= 0 || w <= 0 || h <= 0 {
		return 0
	}
	lo, hi := max(top, y-6*sigma), min(top+h, y+6*sigma)
	if lo >= hi {
		return 0
	}
	cdf := func(z float64) float64 { return .5 * (1 + math.Erf(z/math.Sqrt2)) }
	integrand := func(py float64) float64 {
		inset := float64(0)
		edge := min(py-top, top+h-py)
		if edge < r {
			inset = r - math.Sqrt(max(0, r*r-(r-edge)*(r-edge)))
		}
		span := cdf((left+w-inset-x)/sigma) - cdf((left+inset-x)/sigma)
		z := (py - y) / sigma
		return span * math.Exp(-.5*z*z) / (sigma * math.Sqrt(2*math.Pi))
	}
	const intervals = 1024
	step := (hi - lo) / intervals
	total := integrand(lo) + integrand(hi)
	for i := 1; i < intervals; i++ {
		weight := float64(2)
		if i%2 != 0 {
			weight = 4
		}
		total += weight * integrand(lo+float64(i)*step)
	}
	return min(1, max(0, total*step/3))
}

type popupShadowPixel struct {
	Point      image.Point
	Background [3]uint8
	Expected   [3]uint8
}

// Test probe placement only: submenu halos can overlap the main menu. An
// outside-body input must miss every visible body, not just the top submenu.
func popupHaloClickPoint(body ui.Bounds, other []ui.Bounds, width, height float32) (ui.Bounds, bool) {
	for _, point := range [][2]float32{
		{body.X + body.Width + 3, body.Y + body.Height/2},
		{body.X - 3, body.Y + body.Height/2},
		{body.X + body.Width/2, body.Y + body.Height + 3},
		{body.X + body.Width/2, body.Y - 3},
		{body.X + 16, body.Y + body.Height + 3},
		{body.X + body.Width - 16, body.Y + body.Height + 3},
		{body.X + 16, body.Y - 3},
		{body.X + body.Width - 16, body.Y - 3},
	} {
		x, y := point[0], point[1]
		if x < 0 || y < 0 || x+1 >= width || y+1 >= height || insideBounds(body, x, y) {
			continue
		}
		insideOther := false
		for _, b := range other {
			if insideBounds(b, x, y) {
				insideOther = true
				break
			}
		}
		if !insideOther {
			return ui.Bounds{X: x, Y: y, Width: 1, Height: 1}, true
		}
	}
	return ui.Bounds{}, false
}

func popupShadowPixelAt(body ui.Bounds, style ui.ShadowStyle, scale float32, point image.Point, background [3]uint8) popupShadowPixel {
	// Match the public raster contract: physical pixel center -> exact float32
	// drawable density -> DIP. Sigma remains in DIP.
	x := float64((float32(point.X) + .5) / scale)
	y := float64((float32(point.Y) + .5) / scale)
	a := popupShadowCoverage(body, x, y, style) * float64(style.Color.A)
	sample := popupShadowPixel{Point: point, Background: background}
	for i, channel := range background {
		sample.Expected[i] = uint8(math.Round(float64(channel) * (1 - a)))
	}
	return sample
}

func checkPopupShadowPixels(pixels image.Image, samples []popupShadowPixel) (int, error) {
	expectedDarkening, actualDarkening := 0, 0
	for _, sample := range samples {
		if !sample.Point.In(pixels.Bounds()) {
			return 0, fmt.Errorf("popup shadow sample %v outside %v", sample.Point, pixels.Bounds())
		}
		r, g, b, _ := pixels.At(sample.Point.X, sample.Point.Y).RGBA()
		actual := [3]int{int(r >> 8), int(g >> 8), int(b >> 8)}
		for i, wanted := range sample.Expected {
			difference := actual[i] - int(wanted)
			if difference < -1 || difference > 1 {
				return 0, fmt.Errorf("popup shadow %v RGB%v want%v (background%v)", sample.Point, actual, sample.Expected, sample.Background)
			}
			expectedDarkening += int(sample.Background[i]) - int(wanted)
			actualDarkening += int(sample.Background[i]) - actual[i]
		}
	}
	// A +/-1 single-pixel tolerance would admit a completely absent dark-theme
	// shadow. Require real aggregate darkening as an independent negative gate.
	if expectedDarkening < 3 || actualDarkening < max(1, expectedDarkening/2) {
		return actualDarkening, fmt.Errorf("popup halo absent/too weak: darkening%d, independent expected%d", actualDarkening, expectedDarkening)
	}
	return actualDarkening, nil
}
