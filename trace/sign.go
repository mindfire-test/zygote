package trace

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
)

var (
	// ErrInvalidSignature indicates signature mismatch or corrupted signature.
	ErrInvalidSignature = errors.New("invalid signature: manifest signature verification failed")
	// ErrNoSignature indicates the bundle does not have a signature when one was expected.
	ErrNoSignature = errors.New("no signature found in bundle")
)

// CanonicalizeManifest serializes the given Recording to canonical JSON according to RFC 8785.
// In canonical JSON:
// 1. Whitespace between tokens is eliminated.
// 2. Object keys are sorted lexicographically by UTF-16 code units (for ASCII/standard strings this is standard byte sort).
func CanonicalizeManifest(rec Recording) ([]byte, error) {
	raw, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	var val any
	if err := json.Unmarshal(raw, &val); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := encodeCanonicalJSON(&buf, val); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeCanonicalJSON(buf *bytes.Buffer, v any) error {
	switch val := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(b)
	case float64:
		// JSON numbers: format cleanly per RFC 8785
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(b)
	case []any:
		buf.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encodeCanonicalJSON(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]any:
		buf.WriteByte('{')
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(kb)
			buf.WriteByte(':')
			if err := encodeCanonicalJSON(buf, val[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(b)
	}
	return nil
}

// GenerateKeyPair generates a new Ed25519 public and private key pair.
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// SignManifest signs the RFC 8785 canonical representation of the Recording using the Ed25519 private key.
func SignManifest(rec Recording, privKey ed25519.PrivateKey) ([]byte, error) {
	canon, err := CanonicalizeManifest(rec)
	if err != nil {
		return nil, fmt.Errorf("canonicalizing manifest: %w", err)
	}
	return ed25519.Sign(privKey, canon), nil
}

// VerifyManifest verifies an Ed25519 signature over the canonicalized manifest.
func VerifyManifest(rec Recording, sig []byte, pubKey ed25519.PublicKey) bool {
	canon, err := CanonicalizeManifest(rec)
	if err != nil {
		return false
	}
	return ed25519.Verify(pubKey, canon, sig)
}

// LoadPrivateKey loads an Ed25519 private key from a file.
// The file may contain either raw 64-byte private key bytes or a 32-byte seed.
func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, err
	}
	if len(data) == ed25519.PrivateKeySize {
		return ed25519.PrivateKey(data), nil
	}
	if len(data) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(data), nil
	}
	return nil, fmt.Errorf("invalid ed25519 private key size: expected %d or %d bytes, got %d", ed25519.PrivateKeySize, ed25519.SeedSize, len(data))
}

// LoadPublicKey loads an Ed25519 public key from a file (raw 32 bytes).
func LoadPublicKey(path string) (ed25519.PublicKey, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, err
	}
	if len(data) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid ed25519 public key size: expected %d bytes, got %d", ed25519.PublicKeySize, len(data))
	}
	return ed25519.PublicKey(data), nil
}
