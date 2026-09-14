package redact

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Action string

const (
	ActionRedact     Action = "redact"
	ActionDigestOnly Action = "digest-only"
	ActionBlock      Action = "block"
	ActionIgnore     Action = "ignore"
)

type PolicyRule struct {
	ID     string `yaml:"id"`
	Path   string `yaml:"path"` // glob pattern
	Action Action `yaml:"action"`
}

type Policy struct {
	Rules []PolicyRule `yaml:"rules"`
}

func LoadPolicy(path string) (*Policy, error) {
	b, err := os.ReadFile(path)
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

func (p *Policy) GetActionForSecret(id string) Action {
	for _, r := range p.Rules {
		if r.ID == id {
			return r.Action
		}
	}
	return ActionBlock // FR-9.5 Fail closed
}

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
