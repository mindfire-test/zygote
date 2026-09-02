package vfs

import (
	"encoding/binary"
	"errors"
	"sort"

	"lukechampine.com/blake3"
)

var (
	// ErrInvalidTree is returned when decoding a malformed tree byte slice.
	ErrInvalidTree = errors.New("invalid tree encoding")
)

// Tree represents a directory containing a list of entries.
type Tree []Entry

// Encode serializes the tree into a canonical binary format.
// The tree must be sorted by Name before encoding.
func (t Tree) Encode() []byte {
	size := 4
	for _, e := range t {
		size += 2 + len(e.Name) + 1 + 4 + 8 + 32
	}

	b := make([]byte, size)
	binary.BigEndian.PutUint32(b[0:4], uint32(len(t)))

	offset := 4
	for _, e := range t {
		nameLen := uint16(len(e.Name))
		binary.BigEndian.PutUint16(b[offset:offset+2], nameLen)
		offset += 2

		copy(b[offset:offset+int(nameLen)], e.Name)
		offset += int(nameLen)

		b[offset] = byte(e.Kind)
		offset += 1

		binary.BigEndian.PutUint32(b[offset:offset+4], e.Mode)
		offset += 4

		binary.BigEndian.PutUint64(b[offset:offset+8], uint64(e.Size))
		offset += 8

		copy(b[offset:offset+32], e.Hash[:])
		offset += 32
	}
	return b
}

// DecodeTree deserializes the canonical binary format into a Tree.
func DecodeTree(b []byte) (Tree, error) {
	if len(b) < 4 {
		return nil, ErrInvalidTree
	}

	count := binary.BigEndian.Uint32(b[0:4])
	var t Tree
	if count > 0 {
		t = make(Tree, 0, count)
	}

	offset := 4
	for i := uint32(0); i < count; i++ {
		if offset+2 > len(b) {
			return nil, ErrInvalidTree
		}

		nameLen := binary.BigEndian.Uint16(b[offset : offset+2])
		offset += 2

		if offset+int(nameLen)+1+4+8+32 > len(b) {
			return nil, ErrInvalidTree
		}

		name := string(b[offset : offset+int(nameLen)])
		offset += int(nameLen)

		kind := Kind(b[offset])
		offset += 1

		mode := binary.BigEndian.Uint32(b[offset : offset+4])
		offset += 4

		size := binary.BigEndian.Uint64(b[offset : offset+8])
		offset += 8

		var hash Hash
		copy(hash[:], b[offset:offset+32])
		offset += 32

		t = append(t, Entry{
			Name: name,
			Kind: kind,
			Mode: mode,
			Size: int64(size),
			Hash: hash,
		})
	}
	return t, nil
}

// EmptyTreeHash is the blake3 hash of an empty tree (0 entries).
var EmptyTreeHash = func() Hash {
	t := Tree{}
	b := t.Encode()
	h := blake3.Sum256(b)
	return Hash(h)
}()

// Sort sorts the tree entries by name.
func (t Tree) Sort() {
	sort.Slice(t, func(i, j int) bool {
		return t[i].Name < t[j].Name
	})
}
