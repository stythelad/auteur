package policy

import "strings"

// MatchGlob reports whether path matches pattern.
// Supports the segment wildcard ** (matches zero or more path segments),
// the single-segment wildcard * (matches within a segment),
// and the single-character wildcard ?.
func MatchGlob(pattern, path string) bool {
	patSegs := strings.Split(strings.Trim(pattern, "/"), "/")
	pathSegs := strings.Split(strings.Trim(path, "/"), "/")
	// `**` alone matches everything, including empty paths.
	if len(patSegs) == 1 && patSegs[0] == "**" {
		return true
	}
	return matchSegs(patSegs, pathSegs)
}

func matchSegs(pat, path []string) bool {
	if len(pat) == 0 {
		return len(path) == 0
	}
	if pat[0] == "**" {
		if len(pat) == 1 {
			// Trailing ** requires at least one remaining path segment.
			return len(path) > 0
		}
		for i := 0; i <= len(path); i++ {
			if matchSegs(pat[1:], path[i:]) {
				return true
			}
		}
		return false
	}
	if len(path) == 0 {
		return false
	}
	if !matchSegment(pat[0], path[0]) {
		return false
	}
	return matchSegs(pat[1:], path[1:])
}

func matchSegment(pat, s string) bool {
	for len(pat) > 0 {
		switch pat[0] {
		case '*':
			pat = pat[1:]
			if len(pat) == 0 {
				return true
			}
			for i := 0; i <= len(s); i++ {
				if matchSegment(pat, s[i:]) {
					return true
				}
			}
			return false
		case '?':
			if len(s) == 0 {
				return false
			}
			pat, s = pat[1:], s[1:]
		default:
			if len(s) == 0 || pat[0] != s[0] {
				return false
			}
			pat, s = pat[1:], s[1:]
		}
	}
	return len(s) == 0
}

// AnyMatch reports whether path matches any of patterns.
func AnyMatch(patterns []string, path string) bool {
	for _, p := range patterns {
		if MatchGlob(p, path) {
			return true
		}
	}
	return false
}
