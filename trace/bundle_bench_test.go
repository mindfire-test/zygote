package trace_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
)

func BenchmarkBundle_Export_Deflate(b *testing.B) {
	store := vfs.NewMemStore()
	r := trace.Record(store)
	r.Set("agent", "bench")
	r.Step("init")

	largeContent := bytes.Repeat([]byte("package main\n\nfunc Process() {\n\t// representative codebase workload\n}\n"), 200)
	for i := 0; i < 50; i++ {
		_ = r.World().WriteFile(filepath.Join("pkg", "file.go"), largeContent, 0644)
	}
	r.Step("files")
	rec := r.Recording()

	tmpDir := b.TempDir()
	out := filepath.Join(tmpDir, "deflate.zip")

	b.ResetTimer()
	for b.Loop() {
		_ = trace.ExportBundle(out, store, rec, trace.ExportOptions{Compress: false})
	}
}

func BenchmarkBundle_Export_Zstd(b *testing.B) {
	store := vfs.NewMemStore()
	r := trace.Record(store)
	r.Set("agent", "bench")
	r.Step("init")

	largeContent := bytes.Repeat([]byte("package main\n\nfunc Process() {\n\t// representative codebase workload\n}\n"), 200)
	for i := 0; i < 50; i++ {
		_ = r.World().WriteFile(filepath.Join("pkg", "file.go"), largeContent, 0644)
	}
	r.Step("files")
	rec := r.Recording()

	tmpDir := b.TempDir()
	out := filepath.Join(tmpDir, "zstd.zip")

	b.ResetTimer()
	for b.Loop() {
		_ = trace.ExportBundle(out, store, rec, trace.ExportOptions{Compress: true})
	}
}

func BenchmarkBundle_Import_Deflate(b *testing.B) {
	store := vfs.NewMemStore()
	r := trace.Record(store)
	r.Set("agent", "bench")
	r.Step("init")

	largeContent := bytes.Repeat([]byte("package main\n\nfunc Process() {\n\t// representative codebase workload\n}\n"), 200)
	_ = r.World().WriteFile("main.go", largeContent, 0644)
	r.Step("files")
	rec := r.Recording()

	tmpDir := b.TempDir()
	bundlePath := filepath.Join(tmpDir, "deflate.zip")
	_ = trace.ExportBundle(bundlePath, store, rec, trace.ExportOptions{Compress: false})

	b.ResetTimer()
	for b.Loop() {
		dstStore := vfs.NewMemStore()
		_, _ = trace.ImportFile(bundlePath, dstStore)
	}
}

func BenchmarkBundle_Import_Zstd(b *testing.B) {
	store := vfs.NewMemStore()
	r := trace.Record(store)
	r.Set("agent", "bench")
	r.Step("init")

	largeContent := bytes.Repeat([]byte("package main\n\nfunc Process() {\n\t// representative codebase workload\n}\n"), 200)
	_ = r.World().WriteFile("main.go", largeContent, 0644)
	r.Step("files")
	rec := r.Recording()

	tmpDir := b.TempDir()
	bundlePath := filepath.Join(tmpDir, "zstd.zip")
	_ = trace.ExportBundle(bundlePath, store, rec, trace.ExportOptions{Compress: true})

	b.ResetTimer()
	for b.Loop() {
		dstStore := vfs.NewMemStore()
		_, _ = trace.ImportFile(bundlePath, dstStore)
	}
}
