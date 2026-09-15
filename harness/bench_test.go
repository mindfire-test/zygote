package harness_test

import (
	"fmt"
	"io"
	"testing"

	"github.com/mindfire/zygote/harness"
)

type dummyHandler struct{}

func (d *dummyHandler) Handshake(_ int) (int, error) { return 1, nil }
func (d *dummyHandler) Effect(_, _ string, _ []byte) ([]byte, error) {
	return []byte("ok"), nil
}
func (d *dummyHandler) Step(_ string) (*harness.StepResult, error) {
	return &harness.StepResult{Match: true}, nil
}

type pipeReader struct {
	data [][]byte
	idx  int
}

func (p *pipeReader) Read(b []byte) (n int, err error) {
	if p.idx >= len(p.data) {
		return 0, io.EOF
	}
	n = copy(b, p.data[p.idx])
	p.idx++
	return n, nil
}

func BenchmarkHarnessRoundTrip(b *testing.B) {
	// NFR-1.8 Harness round trip (effect call): p99 < 1 ms
	var requests [][]byte
	requests = append(requests, []byte(`{"jsonrpc":"2.0", "id":1, "method":"handshake", "params":{"version":1}}`+"\n"))

	for i := 0; i < b.N; i++ {
		req := fmt.Sprintf(`{"jsonrpc":"2.0", "id":%d, "method":"effect", "params":{"op":"test", "key":"key", "value":""}}`+"\n", i+2)
		requests = append(requests, []byte(req))
	}

	reader := &pipeReader{data: requests}
	writer := io.Discard

	srv := harness.NewServer(reader, writer, &dummyHandler{})

	b.ResetTimer()
	_ = srv.Serve()
}
