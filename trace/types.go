package trace

import (
	"github.com/mindfire/zygote/journal"
	"github.com/mindfire/zygote/vfs"
)

// Step represents a snapshot of the agent's world at a named boundary.
type Step struct {
	N        int               `json:"n"`
	Name     string            `json:"name"`
	Root     string            `json:"root"`    // hex world hash
	Effects  int               `json:"effects"` // effect count at end of step
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Recording is the canonical manifest of a Run, containing steps and effects.
type Recording struct {
	Version int               `json:"version"`
	Meta    map[string]string `json:"meta,omitempty"`
	Steps   []Step            `json:"steps"`
	Effects []journal.Entry   `json:"effects"`
}

// Check represents the per-step replay verdict, comparing the expected
// recorded world against the actual replayed world.
type Check struct {
	N        int
	Name     string
	Expected vfs.Snapshot
	Actual   vfs.Snapshot
	Match    bool
}

// Bundle combines a Recording with the Store containing all reachable objects.
// This fulfills the type contract required by meiosis (SRS 12.2).
type Bundle struct {
	Recording Recording
	Store     vfs.Store
}
