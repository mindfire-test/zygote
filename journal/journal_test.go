package journal_test

import (
	"errors"
	"testing"

	"github.com/mindfire/zygote/journal"
)

func TestRecord(t *testing.T) {
	r := journal.NewRecorder()

	// 1. Record successful effect
	val, err := journal.ModelRecord(r, "prompt1", func() (string, error) {
		return "hello world", nil
	})
	if err != nil || val != "hello world" {
		t.Fatalf("ModelRecord failed: %v", err)
	}

	// 2. Record failed effect (should NOT be logged)
	_, err = journal.ModelRecord(r, "prompt2", func() (string, error) {
		return "", errors.New("api rate limit")
	})
	if err == nil {
		t.Fatalf("Expected error")
	}

	entries := r.Entries()
	if len(entries) != 1 {
		t.Fatalf("Expected exactly 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Seq != 0 || e.Op != "model" || e.Key != "prompt1" || string(e.Value) != "hello world" {
		t.Fatalf("Unexpected entry state: %+v", e)
	}
}

func TestReplaySuccess(t *testing.T) {
	entries := []journal.Entry{
		{Seq: 0, Op: "model", Key: "prompt1", Value: []byte("response 1")},
		{Seq: 1, Op: "tool:bash", Key: "req1", Value: []byte("ls -la")},
	}

	replayer := journal.NewReplayer(entries)

	val, err := journal.ModelReplay(replayer, "prompt1")
	if err != nil || val != "response 1" {
		t.Fatalf("Expected response 1, got %v (err: %v)", val, err)
	}

	toolVal, err := journal.ToolReplay(replayer, "bash", "req1")
	if err != nil || string(toolVal) != "ls -la" {
		t.Fatalf("Expected ls -la, got %v (err: %v)", string(toolVal), err)
	}
}

func TestDivergenceMismatch(t *testing.T) {
	entries := []journal.Entry{
		{Seq: 0, Op: "model", Key: "prompt1", Value: []byte("response 1")},
	}

	replayer := journal.NewReplayer(entries)
	replayer.SetStep(2)

	// Agent asks for prompt2 instead of prompt1
	_, err := journal.ModelReplay(replayer, "prompt2")
	if err == nil {
		t.Fatalf("Expected divergence error")
	}

	var divErr *journal.DivergenceError
	if !errors.As(err, &divErr) {
		t.Fatalf("Expected DivergenceError type, got %T", err)
	}

	if divErr.Class != journal.ClassEffectMismatch {
		t.Fatalf("Expected ClassEffectMismatch, got %v", divErr.Class)
	}
	if divErr.Step != 2 {
		t.Fatalf("Expected Step 2, got %d", divErr.Step)
	}
}

func TestDivergenceExhausted(t *testing.T) {
	replayer := journal.NewReplayer([]journal.Entry{})

	_, err := journal.ModelReplay(replayer, "prompt1")
	if err == nil {
		t.Fatalf("Expected divergence error")
	}

	var divErr *journal.DivergenceError
	if !errors.As(err, &divErr) {
		t.Fatalf("Expected DivergenceError type, got %T", err)
	}

	if divErr.Class != journal.ClassExhausted {
		t.Fatalf("Expected ClassExhausted, got %v", divErr.Class)
	}
}

func TestClockWrapper(t *testing.T) {
	r := journal.NewRecorder()
	now, err := journal.ClockRecord(r, "start")
	if err != nil {
		t.Fatalf("ClockRecord failed: %v", err)
	}

	replayer := journal.NewReplayer(r.Entries())
	replayedTime, err := journal.ClockReplay(replayer, "start")
	if err != nil {
		t.Fatalf("ClockReplay failed: %v", err)
	}

	// Must match exactly, including nanoseconds
	if !now.Equal(replayedTime) {
		t.Fatalf("Time mismatch: %v != %v", now, replayedTime)
	}
}
