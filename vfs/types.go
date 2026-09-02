package vfs

// Hash is a 32-byte BLAKE3 root hash.
type Hash [32]byte

// Snapshot represents a world state at a point in time.
type Snapshot struct {
	Root Hash
}

// Store is the content-addressed object store interface.
type Store interface {
	Put(b []byte) (Hash, error)
	Get(h Hash) (b []byte, ok bool)
}

// Kind represents the type of a filesystem entry.
type Kind uint8

const (
	KindFile Kind = 0
	KindDir  Kind = 1
)

// Entry represents a single file or directory in a tree.
type Entry struct {
	Name string
	Kind Kind
	Hash Hash
	Mode uint32
	Size int64
}

// ChangeKind represents the type of change between two worlds.
type ChangeKind uint8

const (
	Added ChangeKind = iota
	Modified
	Deleted
)

// Change represents a change between two worlds.
type Change struct {
	Path string
	Kind ChangeKind
	From Hash
	To   Hash
}
