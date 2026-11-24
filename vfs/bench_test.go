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
	for b.Loop() {
		_ = Fork(s, snap)
	}
}

var sampleWorkload = []byte(`
package main

import (
	"context"
	"fmt"
	"os"
)

type Config struct {
	Name    string
	Version int
	Tags    []string
}

func Process(ctx context.Context, cfg Config) error {
	if cfg.Name == "" {
		return fmt.Errorf("name is required")
	}
	for i, tag := range cfg.Tags {
		fmt.Printf("tag[%d] = %s\n", i, tag)
	}
	return nil
}
`)

func BenchmarkDiskStore_Put_Raw(b *testing.B) {
	tmpDir := b.TempDir()
	store, _ := NewDiskStore(tmpDir)

	b.SetBytes(int64(len(sampleWorkload)))
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		data := append([]byte(fmt.Sprintf("// run %d\n", i)), sampleWorkload...)
		_, _ = store.Put(data)
	}
}

func BenchmarkDiskStore_Put_Zstd(b *testing.B) {
	tmpDir := b.TempDir()
	store, _ := NewCompressedDiskStore(tmpDir)

	b.SetBytes(int64(len(sampleWorkload)))
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		data := append([]byte(fmt.Sprintf("// run %d\n", i)), sampleWorkload...)
		_, _ = store.Put(data)
	}
}

func BenchmarkDiskStore_Get_Raw(b *testing.B) {
	tmpDir := b.TempDir()
	store, _ := NewDiskStore(tmpDir)
	h, _ := store.Put(sampleWorkload)

	b.SetBytes(int64(len(sampleWorkload)))
	b.ResetTimer()
	for b.Loop() {
		_, _ = store.Get(h)
	}
}

func BenchmarkDiskStore_Get_Zstd(b *testing.B) {
	tmpDir := b.TempDir()
	store, _ := NewCompressedDiskStore(tmpDir)
	h, _ := store.Put(sampleWorkload)

	b.SetBytes(int64(len(sampleWorkload)))
	b.ResetTimer()
	for b.Loop() {
		_, _ = store.Get(h)
	}
}

