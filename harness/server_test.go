package harness

import (
	"bytes"
	"strings"
	"testing"
)

type mockHandler struct {
	opCalled bool
}

func (m *mockHandler) Handshake(version int) (int, error) { return version, nil }
func (m *mockHandler) Effect(_, _ string, value []byte) ([]byte, error) {
	m.opCalled = true
	return value, nil
}
func (m *mockHandler) Step(_ string) (*StepResult, error) {
	return &StepResult{Match: true}, nil
}

func TestServerGarbageLog(t *testing.T) {
	// A stream with raw agent logs mixed with JSON-RPC
	input := `Loading AI models...
{"jsonrpc": "2.0", "method": "effect", "params": {"op": "model", "key": "prompt-1", "value": "SGVsbG8="}, "id": 1}
Agent finished.
`
	in := strings.NewReader(input)
	var out bytes.Buffer

	h := &mockHandler{}
	srv := NewServer(in, &out, h)

	if err := srv.Serve(); err != nil {
		t.Fatalf("Serve failed: %v", err)
	}

	if !h.opCalled {
		t.Fatal("Expected Effect handler to be called despite garbage logs")
	}

	if !strings.Contains(out.String(), "result") {
		t.Errorf("Expected valid JSON-RPC response, got: %s", out.String())
	}
}
