package harness_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mindfire/zygote/harness"
	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
)

func TestPythonClientParity(t *testing.T) {
	// Skip if python3 is not available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found")
	}

	// 1. Setup a Python script that uses our new client
	script := `
import sys
import os

# Ensure the client is in PYTHONPATH
sys.path.insert(0, os.environ.get("PYTHON_CLIENT_DIR"))
from zygote_harness import ZygoteClient

# Print to simulate garbage logs - should not break harness
print("Starting agent...")
sys.stdout.flush()

client = ZygoteClient()
client.handshake(1)

# Effect
ans = client.effect("model", "q1", b"hello world")

# Print again
print(f"Got answer: {ans}")
sys.stdout.flush()

client.step("step-1")
`

	cwd, _ := os.Getwd()
	pyClientDir := filepath.Join(cwd, "..", "clients", "python")

	// 2. Set up trace
	store := vfs.NewMemStore()
	r := trace.Record(store)
	r.Set("agent", "python-tester")
	r.Step("init")

	// 3. Spawn python subprocess
	cmd := exec.CommandContext(context.Background(), "python3", "-c", script)
	cmd.Env = append(os.Environ(), "PYTHON_CLIENT_DIR="+pyClientDir)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr // we want to see python tracebacks if any

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	srv := harness.NewServer(stdoutPipe, stdinPipe, harness.NewRecordHandler(r))

	errc := make(chan error, 1)
	go func() {
		errc <- srv.Serve()
	}()

	cmdErr := cmd.Wait()
	srvErr := <-errc

	if srvErr != nil && srvErr != io.EOF {
		t.Fatalf("Harness server error: %v", srvErr)
	}
	if cmdErr != nil {
		t.Fatalf("Agent error: %v", cmdErr)
	}

	// 4. Verify Recording
	rec := r.Recording()
	if len(rec.Steps) != 2 {
		t.Fatalf("Expected 2 steps, got %d", len(rec.Steps))
	}
	if rec.Steps[1].Name != "step-1" {
		t.Fatalf("Expected step-1")
	}

	if len(rec.Effects) != 1 {
		t.Fatalf("Expected 1 effect, got %d", len(rec.Effects))
	}

	if rec.Effects[0].Op != "model" || rec.Effects[0].Key != "q1" {
		t.Fatalf("Effect mismatch: %v", rec.Effects[0])
	}

	if string(rec.Effects[0].Value) != "hello world" {
		t.Fatalf("Expected effect value 'hello world', got '%s'", string(rec.Effects[0].Value))
	}
}
