package update

import (
	"strconv"
	"strings"
)

// VersionNewer reports whether a is a higher semver than b (major.minor.patch).
// Suffixes like "-dirty" and leading "v" are ignored. Non-semver values do not compare as newer.
func VersionNewer(a, b string) bool {
	am, ai, ap, aok := parseVersionCore(a)
	bm, bi, bp, bok := parseVersionCore(b)
	if !aok {
		return false
	}
	if !bok {
		return true
	}
	if am != bm {
		return am > bm
	}
	if ai != bi {
		return ai > bi
	}
	return ap > bp
}

// IsUpdateAvailable is true when remote release is newer than the running panel version.
func IsUpdateAvailable(remote, current string) bool {
	return VersionNewer(remote, current)
}

func parseVersionCore(v string) (major, minor, patch int, ok bool) {
	v = strings.TrimSpace(v)
	for len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	if v == "" {
		return 0, 0, 0, false
	}
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, 0, false
	}
	var err error
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	if len(parts) >= 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, 0, 0, false
		}
	}
	return major, minor, patch, true
}
