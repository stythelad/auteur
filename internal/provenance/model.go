package provenance

import "time"

// Origin classifies who authored a line range.
type Origin string

const (
	OriginHuman   Origin = "human"
	OriginAgent   Origin = "agent"
	OriginMixed   Origin = "mixed"
	OriginUnknown Origin = "unknown"
)

// Confidence expresses how sure we are about the attribution.
type Confidence string

const (
	ConfidenceHigh   Confidence = "high"   // recorded by `sentinel wrap`
	ConfidenceMedium Confidence = "medium" // inferred from commit trailers
	ConfidenceLow    Confidence = "low"    // heuristic / best-effort
)

// LineRange is a contiguous span of lines in a file.
// Start and End are 1-indexed and inclusive.
type LineRange struct {
	File   string `json:"file"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Origin Origin `json:"origin"`
}

// Agent describes the agent that produced the change.
// Empty on records where no agent was involved.
type Agent struct {
	Name        string `json:"name,omitempty"`
	Model       string `json:"model,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	PromptHash  string `json:"prompt_hash,omitempty"`
	ToolVersion string `json:"tool_version,omitempty"`
}

// Author is the human who made the commit.
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Review captures post-hoc human signoff and CI state.
type Review struct {
	HumanSignoff []string `json:"human_signoff,omitempty"`
	CIPassed     bool     `json:"ci_passed,omitempty"`
}

// Record is one entry in the provenance ledger — one per commit.
type Record struct {
	Commit     string      `json:"commit"`
	Parent     string      `json:"parent,omitempty"`
	Timestamp  time.Time   `json:"timestamp"`
	Author     Author      `json:"author"`
	Agent      *Agent      `json:"agent,omitempty"`
	Confidence Confidence  `json:"confidence"`
	Lines      []LineRange `json:"lines"`
	Review     Review      `json:"review,omitempty"`
}
