package vfs

import (
	"errors"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
)

var (
	ErrInvalidPath = errors.New("invalid path")
	ErrNotFound    = fs.ErrNotExist
	ErrNotDir      = errors.New("not a directory")
	ErrIsDir       = errors.New("is a directory")
)

// World represents a mutable view of a content-addressed filesystem state.
type World struct {
	store Store
	root  Hash
}

// NewWorld creates a new world starting from an empty tree.
func NewWorld(s Store) *World {
	empty := Tree{}
	b := empty.Encode()
	h, _ := s.Put(b)
	return &World{store: s, root: h}
}

// Fork creates a new World from an existing snapshot in O(1).
func Fork(s Store, snap Snapshot) *World {
	return &World{store: s, root: snap.Root}
}

// Snapshot returns the current world state hash in O(1).
func (w *World) Snapshot() Snapshot {
	return Snapshot{Root: w.root}
}

// isValidPath checks for strict path rules: no absolute paths, no ., no .., no empty components.
func isValidPath(p string) bool {
	if !fs.ValidPath(p) {
		return false
	}
	if p == "" || p[0] == '/' {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func (w *World) loadTree(h Hash) (Tree, error) {
	if h == EmptyTreeHash {
		return Tree{}, nil
	}
	b, ok := w.store.Get(h)
	if !ok {
		return nil, ErrNotFound
	}
	return DecodeTree(b)
}

func (w *World) saveTree(t Tree) (Hash, error) {
	if len(t) == 0 {
		return EmptyTreeHash, nil
	}
	t.Sort()
	b := t.Encode()
	return w.store.Put(b)
}

// WriteFile writes a file to the world, creating intermediate directories if needed.
func (w *World) WriteFile(p string, data []byte, mode uint32) error {
	if !isValidPath(p) {
		return ErrInvalidPath
	}

	blobHash, err := w.store.Put(data)
	if err != nil {
		return err
	}

	entry := Entry{
		Name: path.Base(p),
		Kind: KindFile,
		Hash: blobHash,
		Mode: mode,
		Size: int64(len(data)),
	}

	parts := strings.Split(p, "/")
	newRoot, err := w.putEntry(w.root, parts[:len(parts)-1], entry)
	if err != nil {
		return err
	}
	w.root = newRoot
	return nil
}

func (w *World) putEntry(dirHash Hash, dirs []string, entry Entry) (Hash, error) {
	t, err := w.loadTree(dirHash)
	if err != nil {
		return dirHash, err
	}

	if len(dirs) == 0 {
		// Replace or add entry
		idx := -1
		for i, e := range t {
			if e.Name == entry.Name {
				idx = i
				break
			}
		}
		if idx >= 0 {
			t[idx] = entry
		} else {
			t = append(t, entry)
		}
		return w.saveTree(t)
	}

	nextDir := dirs[0]
	var nextHash Hash = EmptyTreeHash
	idx := -1
	for i, e := range t {
		if e.Name == nextDir {
			if e.Kind != KindDir {
				return dirHash, ErrNotDir
			}
			nextHash = e.Hash
			idx = i
			break
		}
	}

	newNextHash, err := w.putEntry(nextHash, dirs[1:], entry)
	if err != nil {
		return dirHash, err
	}

	dirEntry := Entry{
		Name: nextDir,
		Kind: KindDir,
		Hash: newNextHash,
		Mode: 0o755,
		Size: 0,
	}

	if idx >= 0 {
		t[idx] = dirEntry
	} else {
		t = append(t, dirEntry)
	}
	return w.saveTree(t)
}

// ReadFile reads the content of a file from the world.
func (w *World) ReadFile(p string) ([]byte, error) {
	if !isValidPath(p) {
		return nil, ErrInvalidPath
	}

	entry, err := w.getEntry(p)
	if err != nil {
		return nil, err
	}
	if entry.Kind != KindFile {
		return nil, ErrIsDir
	}

	b, ok := w.store.Get(entry.Hash)
	if !ok {
		return nil, ErrNotFound
	}
	return b, nil
}

func (w *World) getEntry(p string) (Entry, error) {
	if p == "." {
		return Entry{Name: ".", Kind: KindDir, Hash: w.root, Mode: 0o755}, nil
	}
	parts := strings.Split(p, "/")
	curr := w.root
	var lastEntry Entry

	for i, part := range parts {
		t, err := w.loadTree(curr)
		if err != nil {
			return Entry{}, err
		}
		found := false
		for _, e := range t {
			if e.Name == part {
				lastEntry = e
				curr = e.Hash
				found = true
				break
			}
		}
		if !found {
			return Entry{}, ErrNotFound
		}
		if i < len(parts)-1 && lastEntry.Kind != KindDir {
			return Entry{}, ErrNotDir
		}
	}
	return lastEntry, nil
}

// Remove removes a file or directory from the world.
func (w *World) Remove(p string) error {
	if !isValidPath(p) {
		return ErrInvalidPath
	}
	parts := strings.Split(p, "/")
	newRoot, err := w.removeEntry(w.root, parts)
	if err != nil {
		return err
	}
	w.root = newRoot
	return nil
}

func (w *World) removeEntry(dirHash Hash, parts []string) (Hash, error) {
	t, err := w.loadTree(dirHash)
	if err != nil {
		return dirHash, err
	}

	target := parts[0]
	idx := -1
	for i, e := range t {
		if e.Name == target {
			idx = i
			break
		}
	}
	if idx < 0 {
		return dirHash, ErrNotFound
	}

	if len(parts) == 1 {
		// Remove it
		t = append(t[:idx], t[idx+1:]...)
		return w.saveTree(t)
	}

	if t[idx].Kind != KindDir {
		return dirHash, ErrNotDir
	}

	newHash, err := w.removeEntry(t[idx].Hash, parts[1:])
	if err != nil {
		return dirHash, err
	}

	t[idx].Hash = newHash
	return w.saveTree(t)
}

// fs.FS implementation

func (w *World) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	entry, err := w.getEntry(name)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	}
	if entry.Kind == KindDir {
		return &dirFile{w: w, entry: entry, path: name}, nil
	}
	b, ok := w.store.Get(entry.Hash)
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: ErrNotFound}
	}
	return &regFile{entry: entry, data: b, path: name}, nil
}

func (w *World) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	entry, err := w.getEntry(name)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}
	if entry.Kind != KindDir {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: ErrNotDir}
	}
	t, err := w.loadTree(entry.Hash)
	if err != nil {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: err}
	}

	var res []fs.DirEntry
	for _, e := range t {
		res = append(res, &dirEntry{entry: e})
	}
	return res, nil
}

func (w *World) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	entry, err := w.getEntry(name)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: err}
	}
	return &fileInfo{entry: entry}, nil
}

// File structures for fs.FS

type regFile struct {
	entry Entry
	data  []byte
	path  string
	off   int64
}

func (f *regFile) Stat() (fs.FileInfo, error) { return &fileInfo{entry: f.entry}, nil }
func (f *regFile) Read(b []byte) (int, error) {
	if f.off >= int64(len(f.data)) {
		return 0, io.EOF
	}
	n := copy(b, f.data[f.off:])
	f.off += int64(n)
	return n, nil
}
func (f *regFile) Close() error { return nil }

type dirFile struct {
	w     *World
	entry Entry
	path  string
	off   int
}

func (d *dirFile) Stat() (fs.FileInfo, error) { return &fileInfo{entry: d.entry}, nil }
func (d *dirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: ErrIsDir}
}
func (d *dirFile) Close() error { return nil }
func (d *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	t, err := d.w.loadTree(d.entry.Hash)
	if err != nil {
		return nil, err
	}
	if d.off >= len(t) {
		if n <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}

	end := len(t)
	if n > 0 && d.off+n < end {
		end = d.off + n
	}
	var res []fs.DirEntry
	for _, e := range t[d.off:end] {
		res = append(res, &dirEntry{entry: e})
	}
	d.off = end
	return res, nil
}

type fileInfo struct {
	entry Entry
}

func (fi *fileInfo) Name() string { return fi.entry.Name }
func (fi *fileInfo) Size() int64  { return fi.entry.Size }
func (fi *fileInfo) Mode() fs.FileMode {
	m := fs.FileMode(fi.entry.Mode)
	if fi.entry.Kind == KindDir {
		m |= fs.ModeDir
	}
	return m
}
func (fi *fileInfo) ModTime() time.Time { return time.Time{} } // No timestamps
func (fi *fileInfo) IsDir() bool        { return fi.entry.Kind == KindDir }
func (fi *fileInfo) Sys() any           { return nil }

type dirEntry struct {
	entry Entry
}

func (de *dirEntry) Name() string { return de.entry.Name }
func (de *dirEntry) IsDir() bool  { return de.entry.Kind == KindDir }
func (de *dirEntry) Type() fs.FileMode {
	if de.entry.Kind == KindDir {
		return fs.ModeDir
	}
	return 0
}
func (de *dirEntry) Info() (fs.FileInfo, error) { return &fileInfo{entry: de.entry}, nil }
