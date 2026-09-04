package journal

import "sync"

// Recorder maintains an append-only log of non-deterministic effects.
type Recorder struct {
	mu      sync.Mutex
	entries []Entry
	seq     int
}

// NewRecorder initializes a new empty recorder.
func NewRecorder() *Recorder {
	return &Recorder{
		entries: make([]Entry, 0),
		seq:     0,
	}
}

// Record invokes the provided function and, if successful, logs the result.
// Fulfills FR-2.5: A failed effect aborts the recording and is not logged.
func (r *Recorder) Record(op, key string, fn func() ([]byte, error)) ([]byte, error) {
	val, err := fn()
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	entry := Entry{
		Seq:   r.seq,
		Op:    op,
		Key:   key,
		Value: val, // In M2 (FR-2.9), large values may be CAS hashed instead of inlined
	}
	r.entries = append(r.entries, entry)
	r.seq++

	return val, nil
}

// Entries returns a copy of the recorded effect log.
func (r *Recorder) Entries() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	out := make([]Entry, len(r.entries))
	copy(out, r.entries)
	return out
}

// Replayer strictly substitutes recorded effects for live execution.
type Replayer struct {
	mu      sync.Mutex
	entries []Entry
	pos     int
	step    int
}

// NewReplayer initializes a replayer with a pre-recorded log.
func NewReplayer(entries []Entry) *Replayer {
	return &Replayer{
		entries: entries,
		pos:     0,
		step:    0,
	}
}

// SetStep updates the current step number for divergence reporting.
func (r *Replayer) SetStep(step int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.step = step
}

// Replay returns the recorded value for the requested operation.
// Fulfills FR-2.2: The underlying effect function is never invoked.
// Fulfills FR-2.3 and FR-2.4: Mismatches and exhaustion return DivergenceError.
func (r *Replayer) Replay(op, key string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.pos >= len(r.entries) {
		return nil, &DivergenceError{
			Class:   ClassExhausted,
			Message: "run wants more effects than recorded",
			Step:    r.step,
		}
	}

	entry := r.entries[r.pos]
	if entry.Op != op || entry.Key != key {
		return nil, &DivergenceError{
			Class:   ClassEffectMismatch,
			Message: "different op/key requested than recorded",
			Step:    r.step,
		}
	}

	r.pos++
	return entry.Value, nil
}
