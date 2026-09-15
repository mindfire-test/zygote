package redact

import (
	"fmt"
)

// Engine executes redaction policies.
type Engine struct {
	scanner *Scanner
	policy  *Policy
}

// NewEngine initializes a redaction engine with the given policy.
func NewEngine(policy *Policy) (*Engine, error) {
	s, err := NewScanner()
	if err != nil {
		return nil, err
	}
	return &Engine{scanner: s, policy: policy}, nil
}

// ProcessResult contains the result of redacting a payload.
type ProcessResult struct {
	Content    []byte
	DigestOnly bool
}

// Process evaluates a payload (file content or effect value) against the policy.
// It returns the redacted content, a boolean indicating if it should be digest-only,
// or an error if the policy blocks it.
func (e *Engine) Process(pathOrKey string, content []byte) (ProcessResult, error) {
	// First check path-based policy
	pathAction := e.policy.GetActionForPath(pathOrKey)
	if pathAction == ActionBlock {
		return ProcessResult{}, fmt.Errorf("blocked by path policy: %s", pathOrKey)
	}
	if pathAction == ActionDigestOnly {
		return ProcessResult{Content: nil, DigestOnly: true}, nil
	}

	// Scan for secrets
	findings := e.scanner.Scan(content)
	if len(findings) == 0 {
		return ProcessResult{Content: content, DigestOnly: false}, nil
	}

	// Sort findings by start index, reverse order (so replacements don't shift earlier indices)
	for i := 0; i < len(findings)-1; i++ {
		for j := i + 1; j < len(findings); j++ {
			if findings[j].Start > findings[i].Start {
				findings[i], findings[j] = findings[j], findings[i]
			}
		}
	}

	result := append([]byte(nil), content...)

	for _, f := range findings {
		action := e.policy.GetActionForSecret(f.RuleID)
		if action == ActionBlock {
			return ProcessResult{}, fmt.Errorf("fail-closed: secret %s found in %s with no redaction policy", f.RuleID, pathOrKey)
		}
		if action == ActionDigestOnly {
			return ProcessResult{Content: nil, DigestOnly: true}, nil
		}
		if action == ActionRedact {
			// Replace with ***
			prefix := result[:f.Start]
			suffix := result[f.End:]
			var newResult []byte
			newResult = append(newResult, prefix...)
			newResult = append(newResult, []byte("***")...)
			newResult = append(newResult, suffix...)
			result = newResult
		}
	}

	return ProcessResult{Content: result, DigestOnly: false}, nil
}
