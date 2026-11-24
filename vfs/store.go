// Package vfs provides vfs.
package vfs

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mindfire/zygote/pkg/compress"
	"lukechampine.com/blake3"
)

type memObject struct {
	data    []byte
	modTime time.Time
}

// MemStore is an in-memory implementation of Store.
type MemStore struct {
	mu      sync.RWMutex
	objects map[Hash]memObject
}

// NewMemStore creates a new MemStore.
func NewMemStore() *MemStore {
	return &MemStore{
		objects: make(map[Hash]memObject),
	}
}

// Put stores a blob and returns its BLAKE3 hash.
func (s *MemStore) Put(b []byte) (Hash, error) {
	h := blake3.Sum256(b)
	hash := Hash(h)

	s.mu.Lock()
	defer s.mu.Unlock()

	if obj, exists := s.objects[hash]; exists {
		obj.modTime = time.Now()
		s.objects[hash] = obj
		return hash, nil
	}

	clone := make([]byte, len(b))
	copy(clone, b)
	s.objects[hash] = memObject{
		data:    clone,
		modTime: time.Now(),
	}
	return hash, nil
}

// Get retrieves a blob by its hash.
func (s *MemStore) Get(h Hash) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	obj, ok := s.objects[h]
	if !ok {
		return nil, false
	}

	clone := make([]byte, len(obj.data))
	copy(clone, obj.data)
	return clone, true
}

// Delete removes an object by its hash.
func (s *MemStore) Delete(h Hash) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.objects, h)
	return nil
}

// Objects returns all object hashes stored in the MemStore.
func (s *MemStore) Objects() ([]Hash, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hashes := make([]Hash, 0, len(s.objects))
	for h := range s.objects {
		hashes = append(hashes, h)
	}
	return hashes, nil
}

// Stat retrieves metadata about an object in MemStore.
func (s *MemStore) Stat(h Hash) (ObjectInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	obj, ok := s.objects[h]
	if !ok {
		return ObjectInfo{}, os.ErrNotExist
	}

	return ObjectInfo{
		Hash:    h,
		Size:    int64(len(obj.data)),
		ModTime: obj.modTime,
	}, nil
}

// DiskStore is an on-disk implementation of Store.
type DiskStore struct {
	mu       sync.RWMutex
	root     string
	compress bool
}

// NewDiskStore creates a new DiskStore, creating the root directory if necessary.
func NewDiskStore(root string) (*DiskStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil { //nolint:gosec
		return nil, err
	}
	return &DiskStore{root: root}, nil
}

// NewCompressedDiskStore creates a new DiskStore with Zstandard compression enabled (FR-4.10).
func NewCompressedDiskStore(root string) (*DiskStore, error) {
	s, err := NewDiskStore(root)
	if err != nil {
		return nil, err
	}
	s.compress = true
	return s, nil
}

// SetCompression enables or disables transparent Zstandard compression for newly written objects.
func (s *DiskStore) SetCompression(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compress = enabled
}

// IsCompressed reports whether compression is enabled on the DiskStore.
func (s *DiskStore) IsCompressed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.compress
}

// Root returns the root directory path of the DiskStore.
func (s *DiskStore) Root() string {
	return s.root
}

// Put stores a blob to disk using a hash-prefixed fan-out.
func (s *DiskStore) Put(b []byte) (Hash, error) {
	h := blake3.Sum256(b)
	hash := Hash(h)
	hexHash := hex.EncodeToString(hash[:])

	dir := filepath.Join(s.root, hexHash[:2])
	path := filepath.Join(dir, hexHash[2:])

	s.mu.RLock()
	// Check if exists
	if _, err := os.Stat(path); err == nil {
		s.mu.RUnlock()
		now := time.Now()
		_ = os.Chtimes(path, now, now)
		return hash, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Recheck under write lock
	if _, err := os.Stat(path); err == nil {
		now := time.Now()
		_ = os.Chtimes(path, now, now)
		return hash, nil
	}

	if err := os.MkdirAll(dir, 0o755); err != nil { //nolint:gosec
		return hash, err
	}

	// temp-file-and-rename for atomicity
	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	tmpPath := path + ".tmp" + hex.EncodeToString(rnd[:])

	payload := b
	if s.compress {
		compressed := compress.Compress(b)
		if len(compressed) < len(b) {
			payload = compressed
		}
	}

	if err := os.WriteFile(tmpPath, payload, 0o444); err != nil { //nolint:gosec
		return hash, err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath) //nolint:errcheck,gosec
		return hash, err
	}

	return hash, nil
}

// Get retrieves a blob from disk by its hash, automatically decompressing if zstd-compressed.
func (s *DiskStore) Get(h Hash) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hexHash := hex.EncodeToString(h[:])
	path := filepath.Join(s.root, hexHash[:2], hexHash[2:])

	b, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, false
	}

	if compress.IsCompressed(b) {
		if decompressed, err := compress.Decompress(b); err == nil {
			if Hash(blake3.Sum256(decompressed)) == h {
				return decompressed, true
			}
		}
	}

	if Hash(blake3.Sum256(b)) == h {
		return b, true
	}

	return nil, false
}

// Delete removes an object file from disk by its hash.
func (s *DiskStore) Delete(h Hash) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	hexHash := hex.EncodeToString(h[:])
	dir := filepath.Join(s.root, hexHash[:2])
	path := filepath.Join(dir, hexHash[2:])

	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// Attempt to remove empty fan-out directory (ignored if non-empty)
	_ = os.Remove(dir)
	return nil
}

// Objects returns all valid object hashes stored in the DiskStore.
func (s *DiskStore) Objects() ([]Hash, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var hashes []Hash
	for _, entry := range entries {
		if !entry.IsDir() || len(entry.Name()) != 2 {
			continue
		}
		prefix := entry.Name()
		subEntries, err := os.ReadDir(filepath.Join(s.root, prefix))
		if err != nil {
			continue
		}
		for _, subEntry := range subEntries {
			if subEntry.IsDir() {
				continue
			}
			name := subEntry.Name()
			if len(name) != 62 {
				continue
			}
			fullHex := prefix + name
			hb, err := hex.DecodeString(fullHex)
			if err != nil || len(hb) != 32 {
				continue
			}
			var h Hash
			copy(h[:], hb)
			hashes = append(hashes, h)
		}
	}
	return hashes, nil
}

// Stat retrieves metadata about an object on disk.
func (s *DiskStore) Stat(h Hash) (ObjectInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hexHash := hex.EncodeToString(h[:])
	path := filepath.Join(s.root, hexHash[:2], hexHash[2:])

	fi, err := os.Stat(path)
	if err != nil {
		return ObjectInfo{}, err
	}

	return ObjectInfo{
		Hash:    h,
		Size:    fi.Size(),
		ModTime: fi.ModTime(),
	}, nil
}
