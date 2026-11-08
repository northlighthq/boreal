package update

import (
	"strconv"
	"strings"
)

type semver struct {
	core [3]int
	pre  []string
}

func parse(v string) (semver, bool) {
	v, _, _ = strings.Cut(v, "+")
	main, pre, hasPre := strings.Cut(v, "-")
	parts := strings.Split(main, ".")

	if len(parts) != 3 {
		return semver{}, false
	}

	var s semver

	for i, p := range parts {
		n, err := strconv.Atoi(p)

		if err != nil || n < 0 {
			return semver{}, false
		}

		s.core[i] = n
	}

	if hasPre {
		if pre == "" {
			return semver{}, false
		}

		s.pre = strings.Split(pre, ".")
	}

	return s, true
}

func valid(v string) bool {
	_, ok := parse(v)
	return ok
}

func Dev(v string) bool {
	s, ok := parse(v)
	return !ok || (len(s.pre) > 0 && s.pre[0] == "dev")
}

func Newer(a, b string) bool {
	x, okA := parse(a)
	y, okB := parse(b)

	if !okA || !okB {
		return false
	}

	return compare(x, y) > 0
}

func compare(x, y semver) int {
	for i := range x.core {
		if x.core[i] != y.core[i] {
			return sign(x.core[i] - y.core[i])
		}
	}

	switch {
	case len(x.pre) == 0 && len(y.pre) == 0:
		return 0
	case len(x.pre) == 0:
		return 1
	case len(y.pre) == 0:
		return -1
	}

	for i := 0; i < min(len(x.pre), len(y.pre)); i++ {
		if c := comparePart(x.pre[i], y.pre[i]); c != 0 {
			return c
		}
	}

	return sign(len(x.pre) - len(y.pre))
}

func comparePart(a, b string) int {
	na, errA := strconv.Atoi(a)
	nb, errB := strconv.Atoi(b)

	switch {
	case errA == nil && errB == nil:
		return sign(na - nb)
	case errA == nil:
		return -1
	case errB == nil:
		return 1
	}

	return strings.Compare(a, b)
}

func sign(n int) int {
	switch {
	case n > 0:
		return 1
	case n < 0:
		return -1
	}

	return 0
}
