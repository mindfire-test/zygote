package main_test

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
)

func buildZygBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "zyg")
	cmd := exec.Command("go", "build", "-o", binPath, "github.com/mindfire/zygote/cmd/zyg")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build zyg binary: %v\noutput: %s", err, string(out))
	}
	return binPath
}

func TestCLISigningAndVerification(t *testing.T) {
	zyg := buildZygBinary(t)
	tmpDir := t.TempDir()

	// 1. Generate keys
	pub, priv, err := trace.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	pubKeyPath := filepath.Join(tmpDir, "agent.pub")
	privKeyPath := filepath.Join(tmpDir, "agent.key")
	if err := os.WriteFile(pubKeyPath, pub, 0600); err != nil {
		t.Fatalf("writing pub key: %v", err)
	}
	if err := os.WriteFile(privKeyPath, priv, 0600); err != nil {
		t.Fatalf("writing priv key: %v", err)
	}

	// 2. Create a test bundle to work with
	bundlePath := filepath.Join(tmpDir, "unsigned.zip")
	store := vfs.NewMemStore()
	r := trace.Record(store)
	r.Set("agent", "cli-test-agent")
	r.Step("init")
	if err := r.World().WriteFile("main.go", []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	r.Step("step1")
	rec := r.Recording()
	if err := trace.ExportFile(bundlePath, store, rec); err != nil {
		t.Fatalf("ExportFile failed: %v", err)
	}

	// 3. Test `zyg redact --sign --key` creates a signed bundle
	signedBundlePath := filepath.Join(tmpDir, "signed.zip")
	// create an empty redact.yaml
	policyPath := filepath.Join(tmpDir, "redact.yaml")
	if err := os.WriteFile(policyPath, []byte("rules: []\n"), 0600); err != nil {
		t.Fatalf("writing redact policy: %v", err)
	}

	redactCmd := exec.Command(zyg, "redact", bundlePath, "--policy", policyPath, "--out", signedBundlePath, "--sign", "--key", privKeyPath)
	if out, err := redactCmd.CombinedOutput(); err != nil {
		t.Fatalf("zyg redact failed: %v\noutput: %s", err, string(out))
	}

	// 4. Test `zyg inspect` on signed bundle
	// 4a. Inspect without verify key
	inspectCmd1 := exec.Command(zyg, "inspect", signedBundlePath)
	out, err := inspectCmd1.CombinedOutput()
	if err != nil {
		t.Fatalf("zyg inspect failed: %v\noutput: %s", err, string(out))
	}
	if !bytes.Contains(out, []byte("Signature: present (unverified)")) {
		t.Fatalf("expected 'Signature: present (unverified)', got: %s", string(out))
	}

	// 4b. Inspect with valid verify key
	inspectCmd2 := exec.Command(zyg, "inspect", signedBundlePath, "--verify-key", pubKeyPath)
	out, err = inspectCmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("zyg inspect with valid key failed: %v\noutput: %s", err, string(out))
	}
	if !bytes.Contains(out, []byte("Signature: valid")) {
		t.Fatalf("expected 'Signature: valid', got: %s", string(out))
	}

	// 4c. Inspect with wrong verify key (generate another keypair)
	wrongPub, _, _ := trace.GenerateKeyPair()
	wrongPubKeyPath := filepath.Join(tmpDir, "wrong.pub")
	if err := os.WriteFile(wrongPubKeyPath, wrongPub, 0600); err != nil {
		t.Fatalf("writing wrong pub key: %v", err)
	}

	inspectCmd3 := exec.Command(zyg, "inspect", signedBundlePath, "--verify-key", wrongPubKeyPath)
	out, err = inspectCmd3.CombinedOutput()
	if err == nil {
		t.Fatalf("expected zyg inspect with wrong key to fail, but it succeeded: %s", string(out))
	}
	if inspectCmd3.ProcessState.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %d", inspectCmd3.ProcessState.ExitCode())
	}

	// 5. Test `zyg verify`
	// 5a. Verify with valid key
	verifyCmd1 := exec.Command(zyg, "verify", signedBundlePath, "--verify-key", pubKeyPath)
	out, err = verifyCmd1.CombinedOutput()
	if err != nil {
		t.Fatalf("zyg verify with valid key failed: %v\noutput: %s", err, string(out))
	}
	if !bytes.Contains(out, []byte("Bundle verified successfully")) {
		t.Fatalf("expected success message, got: %s", string(out))
	}

	// 5b. Verify with wrong key
	verifyCmd2 := exec.Command(zyg, "verify", signedBundlePath, "--verify-key", wrongPubKeyPath)
	out, err = verifyCmd2.CombinedOutput()
	if err == nil {
		t.Fatalf("expected zyg verify with wrong key to fail, but it succeeded: %s", string(out))
	}
	if verifyCmd2.ProcessState.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %d", verifyCmd2.ProcessState.ExitCode())
	}

	// 5c. Verify unsigned bundle with verify-key -> should fail with exit code 2
	verifyCmd3 := exec.Command(zyg, "verify", bundlePath, "--verify-key", pubKeyPath)
	out, err = verifyCmd3.CombinedOutput()
	if err == nil {
		t.Fatalf("expected zyg verify unsigned bundle with key to fail, but it succeeded: %s", string(out))
	}
	if verifyCmd3.ProcessState.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %d", verifyCmd3.ProcessState.ExitCode())
	}

	// 5d. Tampered bundle
	tamperedSignedBundle := filepath.Join(tmpDir, "tampered_signed.zip")
	tamperZip(t, signedBundlePath, tamperedSignedBundle, "trace.json", func(b []byte) []byte {
		return bytes.Replace(b, []byte("cli-test-agent"), []byte("hacker-agent"), 1)
	})

	verifyCmd4 := exec.Command(zyg, "verify", tamperedSignedBundle, "--verify-key", pubKeyPath)
	out, err = verifyCmd4.CombinedOutput()
	if err == nil {
		t.Fatalf("expected zyg verify on tampered bundle to fail, but it succeeded: %s", string(out))
	}
	if verifyCmd4.ProcessState.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %d", verifyCmd4.ProcessState.ExitCode())
	}
}

func tamperZip(t *testing.T, src, dst, targetEntry string, mutate func([]byte) []byte) {
	t.Helper()
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
