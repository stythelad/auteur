package heuristic

import (
	"regexp"
	"strings"
)

type AgentMatch struct {
	Name  string
	Email string
}

var trailerRe = regexp.MustCompile(`(?im)^Co-authored-by:\s*(.+?)\s*<([^>]+)>\s*$`)

// knownAgents maps a lowercase substring of name-or-email to a canonical
// agent name. Order matters: first match wins.
var knownAgents = []struct {
	needle string
	name   string
}{
	{"anthropic.com", "claude-code"},
	{"claude", "claude-code"},
	{"cursor.com", "cursor"},
	{"cursoragent", "cursor"},
	{"copilot", "copilot"},
	{"aider.chat", "aider"},
	{"codeium.com", "codeium"},
	{"tabnine.com", "tabnine"},
}

func DetectAgent(msg string) *AgentMatch {
	matches := trailerRe.FindAllStringSubmatch(msg, -1)
	for _, m := range matches {
		name := strings.TrimSpace(m[1])
		email := strings.TrimSpace(m[2])
		hay := strings.ToLower(name + " " + email)
		for _, k := range knownAgents {
			if strings.Contains(hay, k.needle) {
				return &AgentMatch{Name: k.name, Email: email}
			}
		}
	}
	return nil
}
