package redact

import (
	_ "embed"
	"regexp"

	"github.com/BurntSushi/toml"
)

//go:embed rules.toml
var rulesTOML []byte

type rulesConfig struct {
	Rules []ruleDef `toml:"rules"`
}

type ruleDef struct {
	ID    string `toml:"id"`
	Regex string `toml:"regex"`
}

// Finding represents a detected secret.
type Finding struct {
	RuleID string
	Start  int
	End    int
}

// Scanner scans for secrets.
type Scanner struct {
	rules []compiledRule
}

type compiledRule struct {
	id string
	re *regexp.Regexp
}

// NewScanner initializes a secret scanner.
func NewScanner() (*Scanner, error) {
	var cfg rulesConfig
	if err := toml.Unmarshal(rulesTOML, &cfg); err != nil {
		return nil, err
	}

	var rules []compiledRule
	for _, r := range cfg.Rules {
		re, err := regexp.Compile(r.Regex)
		if err != nil {
			return nil, err
		}
		rules = append(rules, compiledRule{id: r.ID, re: re})
	}

	return &Scanner{rules: rules}, nil
}

// Scan scans the content for secrets.
func (s *Scanner) Scan(content []byte) []Finding {
	var findings []Finding
	for _, r := range s.rules {
		matches := r.re.FindAllIndex(content, -1)
		for _, m := range matches {
			findings = append(findings, Finding{
				RuleID: r.id,
				Start:  m[0],
				End:    m[1],
			})
		}
	}
	return findings
}
