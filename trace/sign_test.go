package trace_test

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
)

func TestCanonicalizeManifest(t *testing.T) {
	rec := trace.Recording{
		Version:  1,
		Producer: "zygote-test/1.0",
		Meta: map[string]string{
			"zebra": "stripes",
			"apple": "fruit",
			"beta":  "second",
		},
	}

	canon1, err := trace.CanonicalizeManifest(rec)
	if err != nil {
		t.Fatalf("CanonicalizeManifest failed: %v", err)
	}

	canon2, err := trace.CanonicalizeManifest(rec)
	if err != nil {
		t.Fatalf("CanonicalizeManifest failed: %v", err)
	}

	if !bytes.Equal(canon1, canon2) {
		t.Fatalf("canonicalization must be deterministic")
	}

	// Verify no arbitrary whitespace exists in output
	if bytes.Contains(canon1, []byte(" : ")) || bytes.Contains(canon1, []byte("\n")) {
		t.Fatalf("canonical JSON must not contain extra formatting whitespace")
	}
}

func TestSignAndVerifyManifest(t *testing.T) {
	pub, priv, err := trace.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	rec := trace.Recording{
		Version:  1,
		Producer: "test-agent",
		Steps: []trace.Step{
			{N: 0, Name: "init", Root: "0000000000000000000000000000000000000000000000000000000000000000"},
		},
	}

	sig, err := trace.SignManifest(rec, priv)
	if err != nil {
		t.Fatalf("SignManifest failed: %v", err)
	}
	if len(sig) != ed25519.SignatureSize {
		t.Fatalf("expected signature size %d, got %d", ed25519.SignatureSize, len(sig))
	}

	// Verify valid signature
	if !trace.VerifyManifest(rec, sig, pub) {
		t.Fatalf("VerifyManifest failed on valid signature")
	}

	// Tamper with recording
	tampered := rec
	tampered.Producer = "malicious-agent"
	if trace.VerifyManifest(tampered, sig, pub) {
		t.Fatalf("VerifyManifest should fail on tampered manifest")
	}

	// Corrupt signature
	badSig := make([]byte, len(sig))
	copy(badSig, sig)
	badSig[0] ^= 0xff
	if trace.VerifyManifest(rec, badSig, pub) {
		t.Fatalf("VerifyManifest should fail on corrupt signature")
	}

	// Wrong public key
	otherPub, _, _ := trace.GenerateKeyPair()
	if trace.VerifyManifest(rec, sig, otherPub) {
		t.Fatalf("VerifyManifest should fail with wrong public key")
	}
}

func TestLoadKeys(t *testing.T) {
	tmpDir := t.TempDir()
	pub, priv, err := trace.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	pubPath := filepath.Join(tmpDir, "key.pub")
	privPath := filepath.Join(tmpDir, "key.priv")
	seedPath := filepath.Join(tmpDir, "seed.bin")

	if err := os.WriteFile(pubPath, pub, 0600); err != nil {
		t.Fatalf("writing pubkey: %v", err)
	}
	if err := os.WriteFile(privPath, priv, 0600); err != nil {
		t.Fatalf("writing privkey: %v", err)
	}
	if err := os.WriteFile(seedPath, priv.Seed(), 0600); err != nil {
		t.Fatalf("writing seed: %v", err)
	}

	loadedPub, err := trace.LoadPublicKey(pubPath)
	if err != nil {
		t.Fatalf("LoadPublicKey failed: %v", err)
	}
	if !bytes.Equal(pub, loadedPub) {
		t.Fatalf("loaded pubkey does not match original")
	}

	loadedPriv, err := trace.LoadPrivateKey(privPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey failed: %v", err)
	}
	if !bytes.Equal(priv, loadedPriv) {
		t.Fatalf("loaded privkey does not match original")
	}

	loadedPrivFromSeed, err := trace.LoadPrivateKey(seedPath)
	if err != nil {
		t.Fatalf("LoadPrivateKey from seed failed: %v", err)
	}
	if !bytes.Equal(priv, loadedPrivFromSeed) {
		t.Fatalf("loaded privkey from seed does not match original")
	}

	// Test invalid sizes
	badKeyPath := filepath.Join(tmpDir, "bad.key")
	if err := os.WriteFile(badKeyPath, []byte("short"), 0600); err != nil {
		t.Fatalf("writing bad key: %v", err)
	}
	if _, err := trace.LoadPublicKey(badKeyPath); err == nil {
		t.Fatalf("expected error for invalid public key size")
	}
	if _, err := trace.LoadPrivateKey(badKeyPath); err == nil {
		t.Fatalf("expected error for invalid private key size")
	}
}

func TestBundleSignAndVerify(t *testing.T) {
	tmpDir := t.TempDir()
	bundlePath := filepath.Join(tmpDir, "signed_run.zip")

	pub, priv, err := trace.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	store := vfs.NewMemStore()
	r := trace.Record(store)
	r.Step("init")
	if err := r.World().WriteFile("test.txt", []byte("hello signed world"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	r.Step("signed_step")
	rec := r.Recording()
	rec.Producer = "signed-test"

	// Export signed file
	if err := trace.ExportSignedFile(bundlePath, store, rec, priv); err != nil {
		t.Fatalf("ExportSignedFile failed: %v", err)
	}

	// 1. Verify bundle with matching public key
	importStore := vfs.NewMemStore()
	bundle, err := trace.ImportVerifiedFile(bundlePath, importStore, pub)
	if err != nil {
		t.Fatalf("ImportVerifiedFile failed: %v", err)
	}
	if !bundle.Verified {
		t.Fatalf("expected bundle to be marked Verified")
	}
	if len(bundle.Signature) != ed25519.SignatureSize {
		t.Fatalf("expected signature of length %d, got %d", ed25519.SignatureSize, len(bundle.Signature))
	}

	// 2. Import without public key (unverified but valid integrity)
	bundleUnverified, err := trace.ImportFile(bundlePath, vfs.NewMemStore())
	if err != nil {
		t.Fatalf("ImportFile failed: %v", err)
	}
	if bundleUnverified.Verified {
		t.Fatalf("expected bundle without verify key to NOT be marked Verified")
	}
	if len(bundleUnverified.Signature) != ed25519.SignatureSize {
		t.Fatalf("expected signature preserved even when unverified")
	}

	// 3. Verify with wrong public key
	wrongPub, _, _ := trace.GenerateKeyPair()
	_, err = trace.ImportVerifiedFile(bundlePath, vfs.NewMemStore(), wrongPub)
	if err != trace.ErrInvalidSignature {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}

	// 4. Verify unsigned bundle with public key returns ErrNoSignature
	unsignedPath := filepath.Join(tmpDir, "unsigned_run.zip")
	if err := trace.ExportFile(unsignedPath, store, rec); err != nil {
		t.Fatalf("ExportFile failed: %v", err)
	}
	_, err = trace.ImportVerifiedFile(unsignedPath, vfs.NewMemStore(), pub)
	if err != trace.ErrNoSignature {
		t.Fatalf("expected ErrNoSignature, got %v", err)
	}

	// 5. Tampered bundle: modify trace.json in zip
	tamperedPath := filepath.Join(tmpDir, "tampered_run.zip")
	tamperZip(t, bundlePath, tamperedPath, "trace.json", func(data []byte) []byte {
		return bytes.Replace(data, []byte("signed-test"), []byte("hacked-test"), 1)
	})

	_, err = trace.ImportVerifiedFile(tamperedPath, vfs.NewMemStore(), pub)
	if err != trace.ErrInvalidSignature {
		t.Fatalf("expected ErrInvalidSignature on tampered manifest, got %v", err)
	}
}

func tamperZip(t *testing.T, src, dst, targetEntry string, mutate func([]byte) []byte) {
	r, err := zip.OpenReader(src)
	if err != nil {
		t.Fatalf("open src zip: %v", err)
	}
	defer r.Close()

	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("create dst zip: %v", err)
	}
	defer out.Close()

	zw := zip.NewWriter(out)
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read entry %s: %v", f.Name, err)
		}

		if f.Name == targetEntry {
			data = mutate(data)
		}

		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create entry %s in dst: %v", f.Name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write entry %s in dst: %v", f.Name, err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatalf("close dst zip: %v", err)
	}
}
