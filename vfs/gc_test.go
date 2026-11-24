package vfs_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mindfire/zygote/vfs"
)

func TestGC_MemStore(t *testing.T) {
	ctx := context.Background()
	store := vfs.NewMemStore()
	world := vfs.NewWorld(store)

	// Step 1: create initial state
	if err := world.WriteFile("a.txt", []byte("file a content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := world.WriteFile("b.txt", []byte("file b content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	snap1 := world.Snapshot()

	// Step 2: mutate world, overwriting a.txt and adding c.txt
	if err := world.WriteFile("a.txt", []byte("updated a content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := world.WriteFile("c.txt", []byte("file c content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	snap2 := world.Snapshot()

	// Step 3: create random orphan blob
	orphanHash, err := store.Put([]byte("unreferenced orphan blob"))
	if err != nil {
		t.Fatalf("Put orphan failed: %v", err)
	}

	// Verify all objects exist before GC
	if _, ok := store.Get(orphanHash); !ok {
		t.Fatalf("orphan blob missing before GC")
	}

	// Step 4: Run GC keeping only snap2 (with 0 grace period)
	res, err := vfs.GC(ctx, store, []vfs.Snapshot{snap2}, vfs.GCOptions{DryRun: false, GracePeriod: 0})
	if err != nil {
		t.Fatalf("GC failed: %v", err)
	}

	if res.DeletedObjects == 0 {
		t.Fatalf("expected deleted objects > 0, got %d", res.DeletedObjects)
	}

	// Verify orphan was deleted
	if _, ok := store.Get(orphanHash); ok {
		t.Fatalf("expected orphan blob to be deleted")
	}

	// Verify snap2 is completely intact
	if err := vfs.Verify(ctx, store, snap2); err != nil {
		t.Fatalf("snap2 verification failed after GC: %v", err)
	}

	// Verify snap1 now has missing blocks (since old a.txt and old root were pruned)
	if err := vfs.Verify(ctx, store, snap1); err == nil {
		t.Fatalf("expected snap1 to fail verification after pruning")
	}
}

func TestGC_DiskStore(t *testing.T) {
	ctx := context.Background()
	tmpDir := filepath.Join(t.TempDir(), "store")

	store, err := vfs.NewDiskStore(tmpDir)
	if err != nil {
		t.Fatalf("NewDiskStore failed: %v", err)
	}

	world := vfs.NewWorld(store)
	if err := world.WriteFile("hello.txt", []byte("hello disk world"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	snap := world.Snapshot()

	// Put an orphan blob
	orphanData := []byte("temporary orphan on disk")
	orphanHash, err := store.Put(orphanData)
	if err != nil {
		t.Fatalf("Put orphan failed: %v", err)
	}

	// 1. Dry run test
	dryRes, err := vfs.GC(ctx, store, []vfs.Snapshot{snap}, vfs.GCOptions{DryRun: true, GracePeriod: 0})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}
	// Candidates are: initial empty tree created by NewWorld + orphan blob = 2
	if dryRes.CandidateObjects != 2 {
		t.Fatalf("expected 2 candidates, got %d", dryRes.CandidateObjects)
	}
	expectedBytes := int64(4 + len(orphanData)) // 4 bytes for empty tree + orphan
	if dryRes.ReclaimedBytes != expectedBytes {
		t.Fatalf("expected reclaimed bytes %d, got %d", expectedBytes, dryRes.ReclaimedBytes)
	}
	// Verify orphan still exists on disk after dry run
	if _, ok := store.Get(orphanHash); !ok {
		t.Fatalf("dry run must not delete objects")
	}

	// 2. Real GC test
	realRes, err := vfs.GC(ctx, store, []vfs.Snapshot{snap}, vfs.GCOptions{DryRun: false, GracePeriod: 0})
	if err != nil {
		t.Fatalf("real GC failed: %v", err)
	}
	if realRes.DeletedObjects != 2 {
		t.Fatalf("expected 2 deleted objects, got %d", realRes.DeletedObjects)
	}
	if _, ok := store.Get(orphanHash); ok {
		t.Fatalf("expected orphan to be deleted after real GC")
	}

	// Verify active snapshot is intact
	if err := vfs.Verify(ctx, store, snap); err != nil {
		t.Fatalf("active snapshot verification failed: %v", err)
	}
}

func TestGC_GracePeriod_NFR3_4(t *testing.T) {
	ctx := context.Background()
	tmpDir := filepath.Join(t.TempDir(), "store")
	store, err := vfs.NewDiskStore(tmpDir)
	if err != nil {
		t.Fatalf("NewDiskStore failed: %v", err)
	}

	// Put an orphan object now
	orphanHash, err := store.Put([]byte("newly written orphan"))
	if err != nil {
		t.Fatalf("Put orphan failed: %v", err)
	}

	// Run GC with a 5-minute grace period
	res, err := vfs.GC(ctx, store, nil, vfs.GCOptions{
		DryRun:      false,
		GracePeriod: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("GC with grace period failed: %v", err)
	}
	if res.DeletedObjects != 0 {
		t.Fatalf("expected 0 deletions within grace period, got %d", res.DeletedObjects)
	}

	// Object must still exist
	if _, ok := store.Get(orphanHash); !ok {
		t.Fatalf("object within grace period was unexpectedly deleted")
	}

	// Now age the file artificially to 1 hour ago
	hexHash := "0123456789abcdef" // dummy, compute actual
	info, err := store.Stat(orphanHash)
	if err != nil {
		t.Fatalf("Stat failed: %v", err)
	}
	_ = hexHash
	_ = info

	// In DiskStore, update mtime on disk:
	fullHex := ""
	for _, b := range orphanHash {
		fullHex += string("0123456789abcdef"[b>>4]) + string("0123456789abcdef"[b&0xf])
	}
	filePath := filepath.Join(tmpDir, fullHex[:2], fullHex[2:])
	pastTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(filePath, pastTime, pastTime); err != nil {
		t.Fatalf("Chtimes failed: %v", err)
	}

	// Run GC again with 5m grace period
	res2, err := vfs.GC(ctx, store, nil, vfs.GCOptions{
		DryRun:      false,
		GracePeriod: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("GC after aging failed: %v", err)
	}
	if res2.DeletedObjects != 1 {
		t.Fatalf("expected 1 deletion after aging beyond grace period, got %d", res2.DeletedObjects)
	}
	if _, ok := store.Get(orphanHash); ok {
		t.Fatalf("expected aged orphan to be deleted")
	}
}

func TestGC_ConcurrencySafety(t *testing.T) {
	ctx := context.Background()
	tmpDir := filepath.Join(t.TempDir(), "store")
	store, err := vfs.NewDiskStore(tmpDir)
	if err != nil {
		t.Fatalf("NewDiskStore failed: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Writer goroutine simulating active recording run (NFR-3.4)
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				data := []byte{byte(i), byte(i >> 8), 0xAA, 0xBB}
				_, _ = store.Put(data)
				i++
				time.Sleep(time.Millisecond)
			}
		}
	}()

	// Reader / GC goroutines
	for g := 0; g < 3; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 5; i++ {
				_, _ = vfs.GC(ctx, store, nil, vfs.GCOptions{
					DryRun:      false,
					GracePeriod: 100 * time.Millisecond,
				})
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	// Let concurrency run
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestGC_MissingBlockFailsafe(t *testing.T) {
	ctx := context.Background()
	store := vfs.NewMemStore()

	// Put an orphan object
	orphanHash, err := store.Put([]byte("valid orphan"))
	if err != nil {
		t.Fatalf("Put orphan failed: %v", err)
	}

	// Create a snapshot reference to a non-existent root
	var nonExistentRoot vfs.Hash
	nonExistentRoot[0] = 0xDE
	nonExistentRoot[1] = 0xAD

	badSnap := vfs.Snapshot{Root: nonExistentRoot}

	// GC should fail-closed and delete nothing
	_, err = vfs.GC(ctx, store, []vfs.Snapshot{badSnap}, vfs.GCOptions{DryRun: false, GracePeriod: 0})
	if err == nil {
		t.Fatalf("expected error when keep snapshot contains missing block")
	}

	// Orphan must still be in the store
	if _, ok := store.Get(orphanHash); !ok {
		t.Fatalf("orphan was deleted despite keep snapshot validation failure")
	}
}
