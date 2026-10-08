package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"image"
	"image/color"
	"math"
	"os"
	"reflect"
	"testing"

	ui "github.com/neko233-com/godesktop"
)

func TestPopupFrameGateDoesNotChaseInFlightCompletion(t *testing.T) {
	source, err := os.ReadFile("popup_shadow_acceptance_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "popup_shadow_acceptance_windows.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	var gate ast.Expr
	ast.Inspect(file, func(node ast.Node) bool {
		statement, ok := node.(*ast.IfStmt)
		if !ok {
			return true
		}
		usesFloor, requestsFrame := false, false
		ast.Inspect(statement.Cond, func(node ast.Node) bool {
			if id, ok := node.(*ast.Ident); ok && id.Name == "floor" {
				usesFloor = true
			}
			return true
		})
		ast.Inspect(statement.Body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Invalidate" {
					requestsFrame = true
				}
			}
			return true
		})
		if usesFloor && requestsFrame {
			if gate != nil {
				t.Fatal("multiple capture-floor rendering gates")
			}
			gate = statement.Cond
		}
		return true
	})
	if gate == nil {
		t.Fatal("actual fixture capture-floor gate was not found")
	}
	var scalar func(ast.Expr, uint64, uint64) uint64
	scalar = func(expr ast.Expr, completed, inFlight uint64) uint64 {
		switch value := expr.(type) {
		case *ast.Ident:
			if value.Name == "floor" {
				return 3
			}
		case *ast.BasicLit:
			if value.Value == "0" {
				return 0
			}
		case *ast.SelectorExpr:
			if value.Sel.Name == "Completed" {
				return completed
			}
			if value.Sel.Name == "InFlight" {
				return inFlight
			}
		}
		t.Fatalf("unexpected actual gate scalar %T", expr)
		return 0
	}
	var evaluate func(ast.Expr, uint64, uint64) bool
	evaluate = func(expr ast.Expr, completed, inFlight uint64) bool {
		if parens, ok := expr.(*ast.ParenExpr); ok {
			return evaluate(parens.X, completed, inFlight)
		}
		binary, ok := expr.(*ast.BinaryExpr)
		if !ok {
			t.Fatalf("unexpected actual gate expression %T", expr)
		}
		switch binary.Op {
		case token.LOR:
			return evaluate(binary.X, completed, inFlight) || evaluate(binary.Y, completed, inFlight)
		case token.LSS:
			return scalar(binary.X, completed, inFlight) < scalar(binary.Y, completed, inFlight)
		case token.NEQ:
			return scalar(binary.X, completed, inFlight) != scalar(binary.Y, completed, inFlight)
		}
		t.Fatalf("unexpected actual gate operator %v", binary.Op)
		return false
	}
	if !evaluate(gate, 2, 1) || evaluate(gate, 3, 1) || evaluate(gate, 4, 2) {
		t.Fatal("actual gate removed its three-completion floor or requests a frame just to wait for completion")
	}
	// A deterministic scheduler model derived from the actual source condition:
	// complete one existing frame per turn, then submit only if that condition
	// requested one. The former condition never reaches idle on a slow GPU.
	quiesces := func(expr ast.Expr) bool {
		completed, inFlight := uint64(3), uint64(1)
		for turn := 0; turn < 32; turn++ {
			request := evaluate(expr, completed, inFlight)
			if inFlight > 0 {
				completed++
				inFlight--
			}
			if request {
				inFlight++
			}
			if inFlight == 0 {
				return true
			}
		}
		return false
	}
	old, err := parser.ParseExpr("stats.Completed < floor || stats.InFlight != 0")
	if err != nil {
		t.Fatal(err)
	}
	if !quiesces(gate) || quiesces(old) {
		t.Fatal("source-derived frame cycle control failed to separate the former self-wake loop")
	}
}

func TestPopupConfirmationStatusRowsPreserveOriginalGeometry(t *testing.T) {
	// Empty status has always occupied one row in both product dialogs. A
	// fixture must preserve it rather than silently move the body or buttons.
	for _, test := range []struct {
		status string
		lines  []string
	}{
		{"", []string{""}},
		{"Reading disk", []string{"Reading disk"}},
		{"First\r\nSecond", []string{"First", "Second"}},
		{"First\n", []string{"First", ""}},
	} {
		actual := wrapChatText(test.status, 390)
		if !reflect.DeepEqual(actual, test.lines) {
			t.Fatalf("status %q changed existing rows: got%q want%q", test.status, actual, test.lines)
		}
	}
	rows := len(wrapChatText("", 390))
	if reloadHeight, reloadCancelY := 140+20*rows, 76+20*rows; reloadHeight != 160 || reloadCancelY != 96 {
		t.Fatal("empty Revert status moved the original confirmation geometry", reloadHeight, reloadCancelY)
	}
	if closeHeight, closeCancelY := 114+22+20*rows, 74+20*rows; closeHeight != 156 || closeCancelY != 94 {
		t.Fatal("empty close status moved the original single-document geometry", closeHeight, closeCancelY)
	}
}

func TestPopupShadowStylesMatchPinnedCodeOSS(t *testing.T) {
	for _, test := range []struct {
		name        string
		style       func() ui.ShadowStyle
		alpha, blur float32
	}{{"menu-lg", menuPopupShadow, .14, 12}, {"quick-and-modal-xl", modalPopupShadow, .15, 20}} {
		t.Run(test.name, func(t *testing.T) {
			actual := test.style()
			wanted := ui.ShadowStyle{Color: ui.RGBA(0, test.alpha), Blur: test.blur}
			if actual != wanted {
				t.Fatalf("pinned native shadow: got %+v, want %+v", actual, wanted)
			}
			actual.Blur, actual.Color.A = 128, 1
			if test.style() != wanted {
				t.Fatal("one popup changed another popup's style")
			}
		})
	}
}

func TestPopupGaussianReferenceUsesTheMeasuredDIPBlur(t *testing.T) {
	body := ui.Bounds{X: 40, Y: 30, Width: 324, Height: 240}
	for _, style := range []ui.ShadowStyle{menuPopupShadow(), modalPopupShadow()} {
		sigma := float64(style.Blur) / 2
		for _, distance := range []float64{-12, -1, 0, 1, 12} {
			x, y := float64(body.X+body.Width)+distance, float64(body.Y+body.Height/2)
			actual := popupShadowCoverage(body, x, y, style)
			wanted := .5 * (1 + math.Erf(-distance/(sigma*math.Sqrt2)))
			if math.Abs(actual-wanted) > 1e-7 {
				t.Fatalf("blur%v distance%v: Gaussian%v want%v", style.Blur, distance, actual, wanted)
			}
			mirrored := popupShadowCoverage(body, float64(body.X)-distance, y, style)
			if math.Abs(actual-mirrored) > 1e-12 {
				t.Fatal("independent mask is not symmetric")
			}
		}
	}
}

func TestPopupPixelGateRejectsAnAbsentDarkThemeHalo(t *testing.T) {
	body := ui.Bounds{X: 40, Y: 30, Width: 324, Height: 240}
	for _, scale := range []float32{1, 1.5, 2} {
		for _, style := range []ui.ShadowStyle{menuPopupShadow(), modalPopupShadow()} {
			for _, grey := range []uint8{11, 31, 128} {
				baseline := image.NewRGBA(image.Rect(0, 0, 1000, 700))
				actual := image.NewRGBA(baseline.Bounds())
				samples := []popupShadowPixel{}
				for _, dy := range []int{-8, 0, 8} {
					for _, dx := range []int{0, 1, 2} {
						point := image.Pt(int(math.Ceil(float64(body.X+body.Width)*float64(scale)))+dx, int(float64(body.Y+body.Height/2)*float64(scale))+dy)
						sample := popupShadowPixelAt(body, style, scale, point, [3]uint8{grey, grey, grey})
						samples = append(samples, sample)
						baseline.SetRGBA(point.X, point.Y, color.RGBA{grey, grey, grey, 255})
						actual.SetRGBA(point.X, point.Y, color.RGBA{sample.Expected[0], sample.Expected[1], sample.Expected[2], 255})
					}
				}
				if _, err := checkPopupShadowPixels(actual, samples); err != nil {
					t.Fatalf("density%v blur%v grey%d independent expected pixels rejected: %v", scale, style.Blur, grey, err)
				}
				if _, err := checkPopupShadowPixels(baseline, samples); err == nil {
					t.Fatalf("density%v blur%v grey%d admitted absent halo", scale, style.Blur, grey)
				}
			}
		}
	}
}

func TestPopupOutsideProbeMissesBothOverlappingMenuBodies(t *testing.T) {
	main := ui.Bounds{X: 35, Y: 36, Width: 324, Height: 415}
	submenu := ui.Bounds{X: 0, Y: 280, Width: 324, Height: 130}
	point, ok := popupHaloClickPoint(submenu, []ui.Bounds{main}, 640, 480)
	if !ok || insideBounds(main, point.X, point.Y) || insideBounds(submenu, point.X, point.Y) || point.X != 16 || point.Y != 413 {
		t.Fatalf("native submenu halo probe would activate another menu: %+v valid=%t", point, ok)
	}
	if _, ok = popupHaloClickPoint(ui.Bounds{Width: 324, Height: 415}, nil, 200, 200); ok {
		t.Fatal("invented visible halo when the body covers the entire window")
	}
}
