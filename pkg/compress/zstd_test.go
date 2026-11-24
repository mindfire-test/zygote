package compress_test

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/mindfire/zygote/pkg/compress"
)

func TestZstdRoundTrip(t *testing.T) {
	data := bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. Zstandard compression test in Zygote CAS."), 50)

	compressed := compress.Compress(data)
	if !compress.IsCompressed(compressed) {
		t.Fatalf("expected IsCompressed to return true for compressed payload")
	}

	if len(compressed) >= len(data) {
		t.Fatalf("expected compressed size (%d) to be smaller than original (%d)", len(compressed), len(data))
	}

	decompressed, err := compress.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if !bytes.Equal(data, decompressed) {
		t.Fatalf("decompressed payload does not match original")
	}
}

func TestIsCompressed_NonZstd(t *testing.T) {
	if compress.IsCompressed([]byte{}) {
		t.Fatalf("empty slice should not be recognized as compressed")
	}
	if compress.IsCompressed([]byte{1, 2, 3}) {
		t.Fatalf("short slice should not be recognized as compressed")
	}
	if compress.IsCompressed([]byte("regular text file content")) {
		t.Fatalf("regular text should not be recognized as compressed")
	}
}

func BenchmarkCompress(b *testing.B) {
	data := make([]byte, 64*1024) // 64KB representative blob
	_, _ = rand.Read(data)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for b.Loop() {
		_ = compress.Compress(data)
	}
}

func BenchmarkDecompress(b *testing.B) {
	data := bytes.Repeat([]byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"benchmark\")\n}\n"), 500)
	compressed := compress.Compress(data)

	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for b.Loop() {
		_, err := compress.Decompress(compressed)
		if err != nil {
			b.Fatalf("Decompress failed: %v", err)
		}
	}
}
