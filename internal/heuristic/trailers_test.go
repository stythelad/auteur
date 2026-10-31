package heuristic

import "testing"

func TestDetectAgent(t *testing.T) {
	cases := []struct {
		msg  string
		want string
	}{
		{"fix bug\n\nCo-Authored-By: Claude <noreply@anthropic.com>", "claude-code"},
		{"fix bug\n\nCo-authored-by: Claude Opus 4.5 <noreply@anthropic.com>", "claude-code"},
		{"feat: x\n\nCo-authored-by: cursoragent <cursoragent@cursor.com>", "cursor"},
		{"chore\n\nCo-Authored-By: Copilot <noreply@github.com>", "copilot"},
		{"refactor\n\nCo-Authored-By: aider <aider@aider.chat>", "aider"},
		{"chore\n\nCo-Authored-By: Bob <bob@example.com>", ""},
		{"plain commit message", ""},
		{"", ""},
		{"Co-Authored-By: Alice <alice@example.com>\nCo-Authored-By: Claude <noreply@anthropic.com>", "claude-code"},
	}
	for _, c := range cases {
		got := DetectAgent(c.msg)
		if c.want == "" {
			if got != nil {
				t.Errorf("msg=%q want nil, got %+v", c.msg, got)
			}
			continue
		}
		if got == nil || got.Name != c.want {
			t.Errorf("msg=%q want %q, got %+v", c.msg, c.want, got)
		}
	}
}
