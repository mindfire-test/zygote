package vfs_test

import (
	"fmt"
	"testing"

	"github.com/mindfire/zygote/vfs"
)

func TestJournalDeferredHashing(t *testing.T) {
	s := vfs.NewMemStore()
	w := vfs.NewWorld(s)

	// Simulate a batch of writes
	for i := 0; i < 100; i++ {
		path := fmt.Sprintf("src/pkg/file_%d.go", i)
		err := w.WriteFile(path, []byte("package pkg"), 0o644)
		if err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	// At this point, the root hash should still be the empty tree
	// because hashing is deferred!
	if w.Snapshot().Root != vfs.EmptyTreeHash {
		t.Fatalf("Expected root to remain EmptyTreeHash before Commit()")
	}

	// Now we commit, which flushes the journal and computes the tree
	snap, err := w.Commit()
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if snap.Root == vfs.EmptyTreeHash {
		t.Fatalf("Expected root to change after Commit()")
	}

	// Verify we can read the files back
	err = w.WriteFile("src/pkg/file_0.go", []byte("package main"), 0o644) // uncommitted write
	w.Remove("src/pkg/file_1.go") // uncommitted remove

	// Committing again computes the new state
	snap2, err := w.Commit()
	if err != nil {
		t.Fatalf("Commit 2: %v", err)
	}

	if snap.Root == snap2.Root {
		t.Fatalf("Expected roots to differ after modifications")
	}
}
