package installlayout

import "strings"

func numeric(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func compareNumber(a, b string) int {
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return strings.Compare(a, b)
}

// CompareVersion compares validated semantic versions, including prereleases.
// Numeric fields have no arbitrary machine-integer overflow limit.
func CompareVersion(a, b string) int {
	av, ap, ah := strings.Cut(a, "-")
	bv, bp, bh := strings.Cut(b, "-")
	aa, bb := strings.Split(av, "."), strings.Split(bv, ".")
	for i := 0; i < 3; i++ {
		if n := compareNumber(aa[i], bb[i]); n != 0 {
			return n
		}
	}
	if !ah && !bh {
		return 0
	}
	if !ah {
		return 1
	}
	if !bh {
		return -1
	}
	aa, bb = strings.Split(ap, "."), strings.Split(bp, ".")
	for i := 0; i < min(len(aa), len(bb)); i++ {
		an, bn := numeric(aa[i]), numeric(bb[i])
		if an && !bn {
			return -1
		}
		if !an && bn {
			return 1
		}
		n := strings.Compare(aa[i], bb[i])
		if an {
			n = compareNumber(aa[i], bb[i])
		}
		if n != 0 {
			return n
		}
	}
	if len(aa) < len(bb) {
		return -1
	}
	if len(aa) > len(bb) {
		return 1
	}
	return 0
}
