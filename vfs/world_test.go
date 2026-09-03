package vfs

import (
	"testing"
	"testing/fstest"
)

func TestWorld_FS(t *testing.T) {
	s := NewMemStore()
	w := NewWorld(s)

	err := w.WriteFile("a/b/c.txt", []byte("hello world"), 0o644)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	w.Commit()

	err = fstest.TestFS(w, "a/b/c.txt")
	if err != nil {
		t.Fatalf("TestFS failed: %v", err)
	}
}

func TestSnapshotIsContentAddressed(t *testing.T) {
	s := NewMemStore()

	w1 := NewWorld(s)
	w1.WriteFile("file1.txt", []byte("1"), 0o644)
	w1.WriteFile("file2.txt", []byte("2"), 0o644)

	w2 := NewWorld(s)
	w2.WriteFile("file2.txt", []byte("2"), 0o644)
	w2.WriteFile("file1.txt", []byte("1"), 0o644)

	snap1, _ := w1.Commit()
	snap2, _ := w2.Commit()

	if snap1.Root != snap2.Root {
		t.Fatalf("snapshots should be identical regardless of write order")
	}
}

func TestDiff(t *testing.T) {
	s := NewMemStore()

	w1 := NewWorld(s)
	w1.WriteFile("a/b/c.txt", []byte("1"), 0o644)
	w1.WriteFile("a/b/d.txt", []byte("2"), 0o644)
	snap1, _ := w1.Commit()

	w2 := Fork(s, snap1)
	w2.WriteFile("a/b/c.txt", []byte("3"), 0o644) // modify
	w2.Remove("a/b/d.txt")                        // delete
	w2.WriteFile("a/e.txt", []byte("4"), 0o644)   // add
	snap2, _ := w2.Commit()

	changes, err := Diff(s, snap1, snap2)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	if len(changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(changes))
	}
}
