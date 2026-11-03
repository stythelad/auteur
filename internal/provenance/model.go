package provenance

import "time"

type Origin string

const (
	OriginHuman   Origin = "human"
	OriginAgent   Origin = "agent"
	OriginMixed   Origin = "mixed"
	OriginUnknown Origin = "unknown"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "high"
	ConfidenceMedium Confidence = "medium"
	ConfidenceLow    Confidence = "low"
)

type LineRange struct {
	File   string `json:"file"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Origin Origin `json:"origin"`
}

type Agent struct {
	Name        string `json:"name,omitempty"`
	Model       string `json:"model,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	PromptHash  string `json:"prompt_hash,omitempty"`
	ToolVersion string `json:"tool_version,omitempty"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Signoff struct {
	Identity  string `json:"identity"`
	PublicKey string `json:"public_key"`
	Signature string `json:"signature"`
	Timestamp string `json:"timestamp"`
}

type Review struct {
	Signoffs []Signoff `json:"signoffs,omitempty"`
	CIPassed bool      `json:"ci_passed,omitempty"`
}

type Record struct {
	Commit     string      `json:"commit"`
	Parent     string      `json:"parent,omitempty"`
	Timestamp  time.Time   `json:"timestamp"`
	Author     Author      `json:"author"`
	Agent      *Agent      `json:"agent,omitempty"`
	Confidence Confidence  `json:"confidence"`
	Lines      []LineRange `json:"lines"`
	Review     Review      `json:"review,omitempty"`
	PrevHash   string      `json:"prev_hash,omitempty"`
	Hash       string      `json:"hash,omitempty"`
}
