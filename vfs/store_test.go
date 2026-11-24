package vfs_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/mindfire/zygote/pkg/compress"
	"github.com/mindfire/zygote/vfs"
	"lukechampine.com/blake3"
)

func TestCompressedDiskStore(t *testing.T) {
	ctx := context.Background()
	tmpDir := filepath.Join(t.TempDir(), "compressed_store")

	store, err := vfs.NewCompressedDiskStore(tmpDir)
	if err != nil {
		t.Fatalf("NewCompressedDiskStore failed: %v", err)
	}
	if !store.IsCompressed() {
		t.Fatalf("expected store.IsCompressed() to be true")
	}

	// 1. Store large compressible file
	largeData := bytes.Repeat([]byte("func Example() { println(\"testing zstd compressed storage in zygote\") }\n"), 100)
	expectedHash := blake3.Sum256(largeData)

	h, err := store.Put(largeData)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if h != vfs.Hash(expectedHash) {
		t.Fatalf("hash mismatch: expected %x, got %x", expectedHash, h)
	}

	// 2. Check on-disk file: must be compressed with zstd
	hexHash := hex.EncodeToString(h[:])
	onDiskPath := filepath.Join(tmpDir, hexHash[:2], hexHash[2:])
	diskBytes, err := os.ReadFile(onDiskPath)
	if err != nil {
		t.Fatalf("reading disk file: %v", err)
	}

	if !compress.IsCompressed(diskBytes) {
		t.Fatalf("expected on-disk file to be zstd-compressed")
	}
	if len(diskBytes) >= len(largeData) {
		t.Fatalf("expected compressed size (%d) < raw size (%d)", len(diskBytes), len(largeData))
	}

	// 3. Transparent read via Get
	gotData, ok := store.Get(h)
	if !ok {
		t.Fatalf("Get returned ok=false")
	}
	if !bytes.Equal(largeData, gotData) {
		t.Fatalf("decompressed data does not match original")
	}

	// 4. Cross-store compatibility:
	// An uncompressed store instance must be able to read the compressed object transparently
	uncompressedStore, err := vfs.NewDiskStore(tmpDir)
	if err != nil {
		t.Fatalf("NewDiskStore failed: %v", err)
	}
	crossData, ok := uncompressedStore.Get(h)
	if !ok {
		t.Fatalf("uncompressedStore.Get failed to read compressed object")
	}
	if !bytes.Equal(largeData, crossData) {
		t.Fatalf("cross-store data mismatch")
	}

	// 5. Store an uncompressed object in uncompressedStore, then read with compressedStore
	uncompressedStore.SetCompression(false)
	rawSmallData := []byte("small text that does not need compression")
	hRaw, err := uncompressedStore.Put(rawSmallData)
	if err != nil {
		t.Fatalf("uncompressedStore.Put failed: %v", err)
	}
	gotFromComp, ok := store.Get(hRaw)
	if !ok {
		t.Fatalf("compressed store failed to read uncompressed object")
	}
	if !bytes.Equal(rawSmallData, gotFromComp) {
		t.Fatalf("data mismatch reading uncompressed object from compressed store")
	}

	// 6. World operations and verification
	world := vfs.NewWorld(store)
	if err := world.WriteFile("code.go", largeData, 0644); err != nil {
		t.Fatalf("world.WriteFile failed: %v", err)
	}
	snap := world.Snapshot()
	if err := vfs.Verify(ctx, store, snap); err != nil {
		t.Fatalf("vfs.Verify failed on compressed world snapshot: %v", err)
	}
}
