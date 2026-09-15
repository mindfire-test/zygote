package redact

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Action represents a policy action.
type Action string

const (
	// ActionRedact strips the secret.
	ActionRedact Action = "redact"
	// ActionDigestOnly hashes the content.
	ActionDigestOnly Action = "digest-only"
	// ActionBlock blocks export.
	ActionBlock Action = "block"
	// ActionIgnore skips.
	ActionIgnore Action = "ignore"
)

// PolicyRule defines a rule.
type PolicyRule struct {
	ID     string `yaml:"id"`
	Path   string `yaml:"path"` // glob pattern
	Action Action `yaml:"action"`
}

// Policy defines the redaction configuration.
type Policy struct {
	Rules []PolicyRule `yaml:"rules"`
}

// LoadPolicy loads a policy from a file.
func LoadPolicy(path string) (*Policy, error) {
	b, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return &Policy{}, nil // default empty policy
		}
		return nil, err
	}

	var p Policy
	if err := yaml.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// GetActionForSecret looks up the action for a secret ID.
func (p *Policy) GetActionForSecret(id string) Action {
	for _, r := range p.Rules {
		if r.ID == id {
			return r.Action
		}
	}
	return ActionBlock // FR-9.5 Fail closed
}

// GetActionForPath looks up the action for a given file path.
func (p *Policy) GetActionForPath(path string) Action {
	for _, r := range p.Rules {
		if r.Path != "" {
			matched, _ := filepath.Match(r.Path, path)
			if matched {
				return r.Action
			}
		}
	}
	return ActionIgnore
}
