package vfs

import (
	"fmt"
	"testing"
)

func BenchmarkForkLargeWorld(b *testing.B) {
	s := NewMemStore()
	w := NewWorld(s)

	for i := 0; i < 10000; i++ {
		path := fmt.Sprintf("dir/file_%d.txt", i)
		if err := w.WriteFile(path, []byte("data"), 0o644); err != nil {
			b.Fatalf("WriteFile failed: %v", err)
		}
	}

	snap := w.Snapshot()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Fork(s, snap)
	}
}
