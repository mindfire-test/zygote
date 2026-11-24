// Package vfs provides vfs.
package vfs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

// DefaultGCGracePeriod is the default period during which recently created or accessed
// objects are spared from garbage collection to safeguard concurrent recording runs (NFR-3.4).
const DefaultGCGracePeriod = 5 * time.Minute

// GCOptions specifies configuration for a garbage collection run.
type GCOptions struct {
	// DryRun, if true, reports candidates for deletion and reclaimed bytes without mutating the store.
	DryRun bool
	// GracePeriod protects objects modified within this window from being pruned (NFR-3.4).
	// If set to 0, no grace period is applied.
	GracePeriod time.Duration
}

// GCResult reports statistics from a garbage collection run.
type GCResult struct {
	ReachableObjects int
	CandidateObjects int
	DeletedObjects   int
	ReclaimedBytes   int64
	Candidates       []ObjectInfo
	Deleted          []Hash
}

// GC executes garbage collection on the given Store (FR-1.11, NFR-3.4).
// It identifies all objects reachable from the named snapshots (keep), identifies
// unreachable orphan objects older than the configured grace period, and removes them.
func GC(ctx context.Context, store Store, keep []Snapshot, opts GCOptions) (GCResult, error) {
	// 1. Compute reachable set from keep snapshots (FR-1.11)
	reachable, err := Reachable(ctx, store, keep)
	if err != nil {
		return GCResult{}, fmt.Errorf("computing reachable objects: %w", err)
	}

	// 2. Enumerate all objects currently present in the store
	allHashes, err := store.Objects()
	if err != nil {
		return GCResult{}, fmt.Errorf("listing store objects: %w", err)
	}

	now := time.Now()
	var candidates []ObjectInfo
	var reclaimedBytes int64

	// Sort hashes for deterministic GC traversal
	sort.Slice(allHashes, func(i, j int) bool {
		for k := 0; k < 32; k++ {
			if allHashes[i][k] != allHashes[j][k] {
				return allHashes[i][k] < allHashes[j][k]
			}
		}
		return false
	})

	for _, h := range allHashes {
		select {
		case <-ctx.Done():
			return GCResult{}, ctx.Err()
		default:
		}

		if _, ok := reachable[h]; ok {
			continue // Active, reachable object
		}

		info, err := store.Stat(h)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return GCResult{}, fmt.Errorf("inspecting object %x: %w", h, err)
		}

		// Concurrency safety (NFR-3.4): do not prune objects within grace period
		if opts.GracePeriod > 0 && now.Sub(info.ModTime) < opts.GracePeriod {
			continue
		}

		candidates = append(candidates, info)
		reclaimedBytes += info.Size
	}

	result := GCResult{
		ReachableObjects: len(reachable),
		CandidateObjects: len(candidates),
		ReclaimedBytes:   reclaimedBytes,
		Candidates:       candidates,
	}

	if opts.DryRun {
		return result, nil
	}

	// 3. Perform pruning
	deleted := make([]Hash, 0, len(candidates))
	for _, c := range candidates {
		select {
		case <-ctx.Done():
			result.Deleted = deleted
			result.DeletedObjects = len(deleted)
			return result, ctx.Err()
		default:
		}

		if err := store.Delete(c.Hash); err != nil {
			result.Deleted = deleted
			result.DeletedObjects = len(deleted)
			return result, fmt.Errorf("pruning object %x: %w", c.Hash, err)
		}
		deleted = append(deleted, c.Hash)
	}

	result.Deleted = deleted
	result.DeletedObjects = len(deleted)
	return result, nil
}
