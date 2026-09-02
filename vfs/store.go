package vfs

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"

	"lukechampine.com/blake3"
)

// MemStore is an in-memory implementation of Store.
type MemStore struct {
	mu      sync.RWMutex
	objects map[Hash][]byte
}

// NewMemStore creates a new MemStore.
func NewMemStore() *MemStore {
	return &MemStore{
		objects: make(map[Hash][]byte),
	}
}

// Put stores a blob and returns its BLAKE3 hash.
func (s *MemStore) Put(b []byte) (Hash, error) {
	h := blake3.Sum256(b)
	hash := Hash(h)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.objects[hash]; !exists {
		clone := make([]byte, len(b))
		copy(clone, b)
		s.objects[hash] = clone
	}
	return hash, nil
}

// Get retrieves a blob by its hash.
func (s *MemStore) Get(h Hash) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	b, ok := s.objects[h]
	if !ok {
		return nil, false
	}

	clone := make([]byte, len(b))
	copy(clone, b)
	return clone, true
}

// DiskStore is an on-disk implementation of Store.
type DiskStore struct {
	root string
}

// NewDiskStore creates a new DiskStore, creating the root directory if necessary.
func NewDiskStore(root string) (*DiskStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &DiskStore{root: root}, nil
}

// Put stores a blob to disk using a hash-prefixed fan-out.
func (s *DiskStore) Put(b []byte) (Hash, error) {
	h := blake3.Sum256(b)
	hash := Hash(h)
	hexHash := hex.EncodeToString(hash[:])

	dir := filepath.Join(s.root, hexHash[:2])
	path := filepath.Join(dir, hexHash[2:])

	// Check if exists
	if _, err := os.Stat(path); err == nil {
		return hash, nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return hash, err
	}

	// temp-file-and-rename for atomicity
	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	tmpPath := path + ".tmp" + hex.EncodeToString(rnd[:])

	if err := os.WriteFile(tmpPath, b, 0o444); err != nil {
		return hash, err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return hash, err
	}

	return hash, nil
}

// Get retrieves a blob from disk by its hash.
func (s *DiskStore) Get(h Hash) ([]byte, bool) {
	hexHash := hex.EncodeToString(h[:])
	path := filepath.Join(s.root, hexHash[:2], hexHash[2:])

	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return b, true
}
