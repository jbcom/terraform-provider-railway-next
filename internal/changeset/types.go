package changeset

import (
	"encoding/json"
	"fmt"
	"sort"
)

const Version = 1

type Set struct {
	Version     int          `json:"version"`
	Changes     []Change     `json:"changes"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Path     string `json:"path"`
	Message  string `json:"message"`
}

type Change struct {
	Kind         string `json:"kind"`
	Path         string `json:"path"`
	Summary      string `json:"summary"`
	Severity     string `json:"severity"`
	DeployEffect string `json:"deployEffect"`

	Address  string          `json:"address,omitempty"`
	Resource json.RawMessage `json:"resource,omitempty"`
	Previous json.RawMessage `json:"previous,omitempty"`
	Field    string          `json:"field,omitempty"`
	Before   json.RawMessage `json:"before,omitempty"`
	After    json.RawMessage `json:"after,omitempty"`
	Variable string          `json:"variable,omitempty"`
	Details  []string        `json:"details,omitempty"`
}

type BucketResource struct {
	Address string       `json:"address"`
	Type    string       `json:"type"`
	Name    string       `json:"name"`
	Config  BucketConfig `json:"config"`
}

type BucketConfig struct {
	Region string `json:"region,omitempty"`
}

type VariableValue struct {
	Type  string `json:"type"`
	Value string `json:"value,omitempty"`
}

func (s Set) JSON() (json.RawMessage, error) {
	if s.Version != Version {
		return nil, fmt.Errorf("unsupported Railway change-set version %d", s.Version)
	}
	normalized := s
	sort.SliceStable(normalized.Changes, func(i, j int) bool {
		if normalized.Changes[i].Path == normalized.Changes[j].Path {
			return normalized.Changes[i].Kind < normalized.Changes[j].Kind
		}
		return normalized.Changes[i].Path < normalized.Changes[j].Path
	})
	return json.Marshal(normalized)
}

func raw(value any) json.RawMessage {
	result, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return result
}
