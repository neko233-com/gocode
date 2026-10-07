package terminal

import (
	"reflect"
	"runtime"
	"testing"
)

func TestProcessEnvironmentStrictAndExplicitDefaults(t *testing.T) {
	base := []string{"TERM=custom", "COLORTERM=", "TERM_PROGRAM=other", "VALUE=世界😀"}
	if got := ProcessEnvironment(base, false); !reflect.DeepEqual(got, base) {
		t.Fatal("explicit empty/custom defaults overwritten", got)
	}
	got := ProcessEnvironment(base, true)
	got[0] = "TERM=mutated"
	if base[0] != "TERM=custom" {
		t.Fatal("environment aliases caller")
	}
	if got := ProcessEnvironment(nil, true); got == nil || len(got) != 0 {
		t.Fatal("strict empty environment inherited defaults", got)
	}
	if got := ProcessEnvironment(nil, false); len(got) != 3 {
		t.Fatal(got)
	}
	if runtime.GOOS == "windows" {
		got := ProcessEnvironment([]string{"term=custom", "colorterm="}, false)
		if len(got) != 3 {
			t.Fatal("Windows environment key casing", got)
		}
	}
}
