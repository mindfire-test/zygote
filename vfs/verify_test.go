package vfs_test

import (
	"context"
	"testing"

	"github.com/mindfire/zygote/vfs"
)

func TestVerifyAndReachable(t *testing.T) {
	store := vfs.NewMemStore()
	world := vfs.NewWorld(store)

	// Create some files
	err := world.WriteFile("a/b/c.txt", []byte("hello"), 0o644)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	err = world.WriteFile("a/b/d.txt", []byte("world"), 0o644)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	snap := world.Snapshot()
	ctx := context.Background()

	// 1. Verify successful
	if err := vfs.Verify(ctx, store, snap); err != nil {
		t.Fatalf("expected verify to pass, got: %v", err)
	}

	// 2. Reachable blocks
	reachable, err := vfs.Reachable(ctx, store, []vfs.Snapshot{snap})
	if err != nil {
		t.Fatalf("failed to compute reachable: %v", err)
	}
	// Root, a, b, c.txt, d.txt => 5 blocks
	if len(reachable) != 5 {
		t.Errorf("expected 5 reachable blocks, got %d", len(reachable))
	}

	// 3. Test missing block
	// We will manually construct a bad store that is missing a block
	badStore := vfs.NewMemStore()
	_, _ = badStore.Put(vfs.Tree{}.Encode()) // empty tree
	_ = vfs.NewWorld(badStore)

	// Create a valid snapshot in the real store
	hashC, _ := store.Put([]byte("hello"))
	treeB := vfs.Tree{{Name: "c.txt", Kind: vfs.KindFile, Hash: hashC, Mode: 0o644, Size: 5}}
	hashB, _ := store.Put(treeB.Encode())

	// But in badStore we only put treeB, leaving hashC missing
	badStore.Put(treeB.Encode())
	badSnap := vfs.Snapshot{Root: hashB}

	if err := vfs.Verify(ctx, badStore, badSnap); err != vfs.ErrMissingBlock {
		t.Errorf("expected ErrMissingBlock, got %v", err)
	}
}
