// Package journal provides effects.
package journal

import (
	"time"
)

// ClockRecord records or replays the current time.
// Since time.Time serializes well to text, we marshal it as RFC3339Nano.
func ClockRecord(r *Recorder, key string) (time.Time, error) {
	val, err := r.Record("clock", key, func() ([]byte, error) {
		t := time.Now()
		b, err := t.MarshalText()
		return b, err
	})
	if err != nil {
		return time.Time{}, err
	}
	var t time.Time
	err = t.UnmarshalText(val)
	return t, err
}

// ClockReplay replays the clock.
func ClockReplay(r *Replayer, key string) (time.Time, error) {
	val, err := r.Replay("clock", key)
	if err != nil {
		return time.Time{}, err
	}
	var t time.Time
	err = t.UnmarshalText(val)
	return t, err
}

// ModelRecord records or replays an LLM response.
func ModelRecord(r *Recorder, key string, call func() (string, error)) (string, error) {
	val, err := r.Record("model", key, func() ([]byte, error) {
		resp, err := call()
		if err != nil {
			return nil, err
		}
		return []byte(resp), nil
	})
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// ModelReplay replays the model.
func ModelReplay(r *Replayer, key string) (string, error) {
	val, err := r.Replay("model", key)
	if err != nil {
		return "", err
	}
	return string(val), nil
}

// ToolRecord records or replays a named tool call.
func ToolRecord(r *Recorder, name, key string, call func() ([]byte, error)) ([]byte, error) {
	return r.Record("tool:"+name, key, call)
}

// ToolReplay replays the tool.
func ToolReplay(r *Replayer, name, key string) ([]byte, error) {
	return r.Replay("tool:"+name, key)
}
