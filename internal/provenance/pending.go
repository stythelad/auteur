package provenance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const PendingDir = ".sentinel/pending"

type Pending struct {
	UUID      string    `json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	Command   []string  `json:"command"`
	BeforeSHA string    `json:"before_sha,omitempty"`
	AfterSHA  string    `json:"after_sha,omitempty"`
	Record    Record    `json:"record"`
}

func PendingPath(repoRoot, uuid string) string {
	return filepath.Join(repoRoot, PendingDir, uuid+".json")
}

func WritePending(repoRoot string, p Pending) error {
	dir := filepath.Join(repoRoot, PendingDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create pending dir: %w", err)
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal pending: %w", err)
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(PendingPath(repoRoot, p.UUID), raw, 0o644); err != nil {
		return fmt.Errorf("write pending: %w", err)
	}
	return nil
}

func ListPending(repoRoot string) ([]Pending, error) {
	dir := filepath.Join(repoRoot, PendingDir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read pending dir: %w", err)
	}
	var out []Pending
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		p, err := LoadPending(repoRoot, strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func LoadPending(repoRoot, uuid string) (Pending, error) {
	raw, err := os.ReadFile(PendingPath(repoRoot, uuid))
	if err != nil {
		return Pending{}, fmt.Errorf("read pending %s: %w", uuid, err)
	}
	var p Pending
	if err := json.Unmarshal(raw, &p); err != nil {
		return Pending{}, fmt.Errorf("parse pending %s: %w", uuid, err)
	}
	return p, nil
}

func DeletePending(repoRoot, uuid string) error {
	if err := os.Remove(PendingPath(repoRoot, uuid)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete pending %s: %w", uuid, err)
	}
	return nil
}
