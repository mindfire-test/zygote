package vfs

// EffectType represents the type of a journal effect.
type EffectType string

const (
	EffectWrite  EffectType = "write"
	EffectRemove EffectType = "remove"
)

// Effect represents a single mutation applied to a World.
type Effect interface {
	Type() EffectType
	Path() string
}

// WriteEffect represents a file creation or modification.
type WriteEffect struct {
	P    string `json:"path"`
	H    Hash   `json:"hash"`
	M    uint32 `json:"mode"`
	Size int64  `json:"size"`
}

// Type returns the type of the effect.
func (e *WriteEffect) Type() EffectType { return EffectWrite }
// Path returns the affected path.
func (e *WriteEffect) Path() string     { return e.P }

// RemoveEffect represents a file or directory deletion.
type RemoveEffect struct {
	P string `json:"path"`
}

// Type returns the type of the effect.
func (e *RemoveEffect) Type() EffectType { return EffectRemove }
// Path returns the affected path.
func (e *RemoveEffect) Path() string     { return e.P }

// Journal is an in-memory sequential log of effects applied to a World.
type Journal struct {
	Effects []Effect
}

// Append adds a new effect to the journal.
func (j *Journal) Append(e Effect) {
	j.Effects = append(j.Effects, e)
}

// Clear removes all effects from the journal.
func (j *Journal) Clear() {
	j.Effects = nil
}
