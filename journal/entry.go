package journal

import "fmt"

// Entry represents a single nondeterministic effect recorded during an agent's run.
// This struct strictly conforms to the zygote SRS format specification.
type Entry struct {
	Seq   int    `json:"seq"`
	Op    string `json:"op"`  // e.g., "model", "clock", "tool:<name>", "http"
	Key   string `json:"key"` // request identity within the category
	Value []byte `json:"value"`
}

// DivergenceClass represents the taxonomy of divergence defined in SRS §8.4.
type DivergenceClass string

const (
	ClassEffectMismatch DivergenceClass = "effect-mismatch"
	ClassWorldMismatch  DivergenceClass = "world-mismatch"
	ClassStepMismatch   DivergenceClass = "step-mismatch"
	ClassExhausted      DivergenceClass = "exhausted"
	ClassMissingObject  DivergenceClass = "missing-object"
	ClassUninstrumented DivergenceClass = "uninstrumented"
)

// DivergenceError is returned during replay when an agent's behavior deviates
// from the recorded timeline.
type DivergenceError struct {
	Class   DivergenceClass
	Message string
	Step    int // the step number where divergence occurred
}

func (e *DivergenceError) Error() string {
	return fmt.Sprintf("divergence [%s] at step %d: %s", e.Class, e.Step, e.Message)
}
