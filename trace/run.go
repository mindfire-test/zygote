package trace

import (
	"encoding/hex"

	"github.com/mindfire/zygote/journal"
	"github.com/mindfire/zygote/vfs"
)

// RunRecorder orchestrates the agent's live filesystem (World) and its
// nondeterministic effects (Journal), allowing discrete Steps to be captured.
type RunRecorder struct {
	world    *vfs.World
	journal  *journal.Recorder
	steps    []Step
	meta     map[string]string
	stepBase int
}

// Record begins a new trace tracking session on top of a store.
func Record(store vfs.Store) *RunRecorder {
	return &RunRecorder{
		world:   vfs.NewWorld(store),
		journal: journal.NewRecorder(),
		steps:   make([]Step, 0),
		meta:    make(map[string]string),
	}
}

// Set stores arbitrary string metadata for the run (e.g. model version, commit).
func (r *RunRecorder) Set(key, value string) {
	r.meta[key] = value
}

// World returns the live Virtual File System the agent is currently editing.
func (r *RunRecorder) World() *vfs.World {
	return r.world
}

// Recorder returns the journal for logging nondeterministic effects.
func (r *RunRecorder) Recorder() *journal.Recorder {
	return r.journal
}

// Step captures the current world state and effect count as a named boundary.
func (r *RunRecorder) Step(name string) {
	snap := r.world.Snapshot()
	effs := len(r.journal.Entries())

	step := Step{
		N:       r.stepBase,
		Name:    name,
		Root:    hex.EncodeToString(snap.Root[:]),
		Effects: effs,
	}

	r.steps = append(r.steps, step)
	r.stepBase++
}

// Recording returns the finalized manifest of all steps and effects.
func (r *RunRecorder) Recording() Recording {
	return Recording{
		Version: 1, // SRS 9.0: FormatVersion = 1 today
		Meta:    r.meta,
		Steps:   r.steps,
		Effects: r.journal.Entries(),
	}
}
