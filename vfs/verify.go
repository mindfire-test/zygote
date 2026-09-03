package vfs

import (
	"context"
	"errors"

	"lukechampine.com/blake3"
)

var (
	// ErrMissingBlock is returned when a hash reference cannot be resolved in the store.
	ErrMissingBlock = errors.New("missing block in store")
	// ErrCorruptBlock is returned when a retrieved block's content does not match its hash.
	ErrCorruptBlock = errors.New("corrupt block in store (hash mismatch)")
)

// Verify checks the cryptographic integrity of a snapshot.
// It recursively ensures that all blocks reachable from the snapshot exist in the store
// and that their contents precisely match their expected BLAKE3 hash.
func Verify(ctx context.Context, store Store, snap Snapshot) error {
	visited := make(map[Hash]struct{})
	return verifyNode(ctx, store, snap.Root, KindDir, visited)
}

func verifyNode(ctx context.Context, store Store, h Hash, kind Kind, visited map[Hash]struct{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, ok := visited[h]; ok {
		return nil // already verified
	}
	visited[h] = struct{}{}

	b, ok := store.Get(h)
	if !ok {
		return ErrMissingBlock
	}

	// Verify cryptographic integrity
	actualHash := Hash(blake3.Sum256(b))
	if actualHash != h {
		return ErrCorruptBlock
	}

	if kind == KindDir {
		tree, err := DecodeTree(b)
		if err != nil {
			return err
		}

		for _, entry := range tree {
			if err := verifyNode(ctx, store, entry.Hash, entry.Kind, visited); err != nil {
				return err
			}
		}
	}

	return nil
}

// Reachable computes the set of all Hash blocks reachable from the given snapshots.
// This is the foundational algorithm for VFS garbage collection.
func Reachable(ctx context.Context, store Store, snaps []Snapshot) (map[Hash]struct{}, error) {
	reachable := make(map[Hash]struct{})

	for _, snap := range snaps {
		if err := collectReachable(ctx, store, snap.Root, KindDir, reachable); err != nil {
			return nil, err
		}
	}

	return reachable, nil
}

func collectReachable(ctx context.Context, store Store, h Hash, kind Kind, reachable map[Hash]struct{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if _, ok := reachable[h]; ok {
		return nil // already visited
	}
	reachable[h] = struct{}{}

	b, ok := store.Get(h)
	if !ok {
		return ErrMissingBlock
	}

	if kind == KindDir {
		tree, err := DecodeTree(b)
		if err != nil {
			return err
		}

		for _, entry := range tree {
			if err := collectReachable(ctx, store, entry.Hash, entry.Kind, reachable); err != nil {
				return err
			}
		}
	}

	return nil
}
