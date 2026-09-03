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

	snap, _ := w.Commit()

	b.ResetTimer()
	for b.Loop() {
		_ = Fork(s, snap)
	}
}
