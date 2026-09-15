package trace_test

import (
	"testing"

	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
)

func BenchmarkRecordingOverhead(b *testing.B) {
	store := vfs.NewMemStore()
	r := trace.Record(store)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = r.Recorder().Record("test", "key", func() ([]byte, error) { return []byte("val"), nil })
		r.Step("step")
	}
}

func BenchmarkReplayThroughput(b *testing.B) {
	store := vfs.NewMemStore()
	r := trace.Record(store)
	for i := 0; i < 1000; i++ {
		_, _ = r.Recorder().Record("test", "key", func() ([]byte, error) { return []byte("val"), nil })
		r.Step("step")
	}

	bundle := &trace.Bundle{
		Recording: r.Recording(),
		Store:     store,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		replayer, err := trace.Replay(store, bundle)
		if err != nil {
			b.Fatal(err)
		}
		b.StartTimer()

		for j := 0; j < len(bundle.Recording.Steps)-1; j++ {
			// Because trace.Replay initializes the world to step 0 and skips it
			if j > 0 {
				_, err := replayer.Replayer().Replay("test", "key")
				if err != nil {
					b.Fatal(err)
				}
			}
			chk, err := replayer.Step("step")
			if err != nil || !chk.Match {
				b.Fatalf("replay step mismatch: %v", err)
			}
		}
	}
}
