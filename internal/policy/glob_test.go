package policy

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"auth/**", "auth/login.py", true},
		{"auth/**", "auth/sub/login.py", true},
		{"auth/**", "auth", false},
		{"**/migrations/**", "db/migrations/001.sql", true},
		{"**/migrations/**", "migrations/001.sql", true},
		{"migrations/**", "migrations/001.sql", true},
		{"migrations/**", "db/migrations/001.sql", false},
		{"*.py", "login.py", true},
		{"*.py", "auth/login.py", false},
		{"**/*.py", "auth/login.py", true},
		{"**/*.py", "a/b/c/x.py", true},
		{"**", "anything/at/all", true},
		{"**", "", true},
		{"auth/**", "./auth/login.py", false},
		{"a/?/c", "a/b/c", true},
		{"a/?/c", "a/bb/c", false},
	}
	for _, c := range cases {
		got := MatchGlob(c.pattern, c.path)
		if got != c.want {
			t.Errorf("MatchGlob(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
