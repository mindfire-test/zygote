// Package trace provides trace.
package trace

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mindfire/zygote/pkg/compress"
	"github.com/mindfire/zygote/vfs"
	"lukechampine.com/blake3"
)

var (
	// ErrHashMismatch indicates mismatch.
	ErrHashMismatch = errors.New("corrupt bundle: object content does not match its hash filename")
	// ErrMissingObject indicates missing object.
	ErrMissingObject = errors.New("corrupt bundle: recording relies on objects not present in the bundle")
	// ErrPathTraversal indicates a dangerous path entry in bundle.
	ErrPathTraversal = errors.New("corrupt bundle: path traversal detected")
)

// ExportOptions specifies configuration options for exporting a bundle.
type ExportOptions struct {
	PrivateKey []byte // Detached Ed25519 private signing key (FR-4.8, OD-7)
	Compress   bool   // Enable Zstandard compression for objects inside the bundle (FR-4.10)
}

// ExportFile writes a highly portable zip bundle containing the Recording manifest
// and all reachable cryptographically hashed vfs objects.
func ExportFile(filename string, store vfs.Store, rec Recording) error {
	return ExportBundle(filename, store, rec, ExportOptions{})
}

// ExportSignedFile writes a zip bundle with an optional detached Ed25519 signature
// over the canonicalized manifest (trace.json per RFC 8785) (FR-4.8, OD-7).
func ExportSignedFile(filename string, store vfs.Store, rec Recording, privKey []byte) error {
	return ExportBundle(filename, store, rec, ExportOptions{PrivateKey: privKey})
}

// ExportBundle writes a zip bundle with configurable signature and compression options (FR-4.8, FR-4.10).
func ExportBundle(filename string, store vfs.Store, rec Recording, opts ExportOptions) error {
	f, err := os.Create(filename) //nolint:gosec
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	zw := zip.NewWriter(f)

	// 1. Write canonical JSON manifest
	manifestFile, err := zw.Create("trace.json")
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if _, err := manifestFile.Write(b); err != nil {
		return err
	}

	// 2. If private key provided, sign canonical manifest and write "signature" entry
	if len(opts.PrivateKey) > 0 {
		sig, err := SignManifest(rec, opts.PrivateKey)
		if err != nil {
			return fmt.Errorf("signing manifest: %w", err)
		}
		sigFile, err := zw.Create("signature")
		if err != nil {
			return err
		}
		if _, err := sigFile.Write(sig); err != nil {
			return err
		}
	}

	// 3. Identify all reachable objects
	var snaps []vfs.Snapshot
	for _, step := range rec.Steps {
		var h vfs.Hash
		hb, err := hex.DecodeString(step.Root)
		if err != nil || len(hb) != 32 {
			continue
		}
		copy(h[:], hb)
		snaps = append(snaps, vfs.Snapshot{Root: h})
	}

	reachable, err := vfs.Reachable(context.Background(), store, snaps)
	if err != nil {
		return err
	}

	// 4. Write objects (optionally compressed with Zstandard)
	for hash := range reachable {
		if hash == vfs.EmptyTreeHash {
			continue
		}
		data, ok := store.Get(hash)
		if !ok {
			return fmt.Errorf("missing object in store: %x", hash)
		}

		payload := data
		method := uint16(zip.Deflate)

		if opts.Compress {
			compressed := compress.Compress(data)
			if len(compressed) < len(data) {
				payload = compressed
				method = zip.Store // Already compressed with zstd, store directly without redundant deflate
			}
		}

		header := &zip.FileHeader{
			Name:   "objects/" + hex.EncodeToString(hash[:]),
			Method: method,
		}
		objFile, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err := objFile.Write(payload); err != nil {
			return err
		}
	}

	return zw.Close()
}

// ImportFile reads a zip bundle, cryptographically validates every object's
// integrity, and ensures the manifest is fully loadable before returning it.
func ImportFile(filename string, store vfs.Store) (Bundle, error) {
	return ImportVerifiedFile(filename, store, nil)
}

// ImportVerifiedFile reads a zip bundle, cryptographically validates every object's
// integrity, checks reachability, and optionally verifies the detached Ed25519 signature
// against the provided public key (if pubKey is non-empty).
// If pubKey is provided and the bundle has no signature, ErrNoSignature is returned.
// If pubKey is provided and the signature does not match, ErrInvalidSignature is returned.
func ImportVerifiedFile(filename string, store vfs.Store, pubKey []byte) (Bundle, error) {
	r, err := zip.OpenReader(filename)
	if err != nil {
		return Bundle{}, err
	}
	defer r.Close() //nolint:errcheck

	var rec Recording
	var signature []byte

	// 1. Read objects and verify integrity (FR-4.3)
	for _, f := range r.File {
		if strings.Contains(f.Name, "..") || strings.HasPrefix(f.Name, "/") || strings.HasPrefix(f.Name, "\\") {
			return Bundle{}, ErrPathTraversal
		}
		if f.Name == "signature" {
			rc, err := f.Open()
			if err != nil {
				return Bundle{}, err
			}
			sigBytes, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return Bundle{}, err
			}
			signature = sigBytes
			continue
		}
		if f.Name == "trace.json" {
			rc, err := f.Open()
			if err != nil {
				return Bundle{}, err
			}
			dec := json.NewDecoder(rc)
			err = dec.Decode(&rec)
			_ = rc.Close()
			if err != nil {
				return Bundle{}, err
			}
			if rec.Version != 1 {
				return Bundle{}, fmt.Errorf("unsupported format version: %d", rec.Version)
			}
			continue
		}

		if strings.HasPrefix(f.Name, "objects/") {
			hexName := strings.TrimPrefix(f.Name, "objects/")
			expectedHash, err := hex.DecodeString(hexName)
			if err != nil || len(expectedHash) != 32 {
				continue
			}

			rc, err := f.Open()
			if err != nil {
				return Bundle{}, err
			}
			raw, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return Bundle{}, err
			}

			data := raw
			if compress.IsCompressed(raw) {
				if decompressed, err := compress.Decompress(raw); err == nil {
					actualHash := blake3.Sum256(decompressed)
					if bytes.Equal(actualHash[:], expectedHash) {
						data = decompressed
					}
				}
			}

			actualHash := blake3.Sum256(data)
			if !bytes.Equal(actualHash[:], expectedHash) {
				return Bundle{}, ErrHashMismatch
			}

			if _, err := store.Put(data); err != nil {
				return Bundle{}, err
			}
		}
	}

	// 2. Validate reachability (FR-4.4)
	for _, step := range rec.Steps {
		var h vfs.Hash
		hb, err := hex.DecodeString(step.Root)
		if err == nil && len(hb) == 32 {
			copy(h[:], hb)
			if err := vfs.Verify(context.Background(), store, vfs.Snapshot{Root: h}); err != nil {
				return Bundle{}, ErrMissingObject
			}
		}
	}

	verified := false
	if len(pubKey) > 0 {
		if len(signature) == 0 {
			return Bundle{Recording: rec, Store: store, Signature: signature}, ErrNoSignature
		}
		if !VerifyManifest(rec, signature, pubKey) {
			return Bundle{Recording: rec, Store: store, Signature: signature}, ErrInvalidSignature
		}
		verified = true
	}

	return Bundle{Recording: rec, Store: store, Signature: signature, Verified: verified}, nil
}
