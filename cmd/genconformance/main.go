package main

import (
	"archive/zip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mindfire/zygote/journal"
	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
	"lukechampine.com/blake3"
)

type TreeFixture struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Entries     []TreeEntry `json:"entries"`
	EncodedHex  string      `json:"encoded_hex"`
	HashHex     string      `json:"hash_hex"`
}

type TreeEntry struct {
	Name string `json:"name"`
	Kind string `json:"kind"` // "file" or "dir"
	Mode uint32 `json:"mode"`
	Size int64  `json:"size"`
	Hash string `json:"hash"` // hex string
}

func mustHash(data []byte) vfs.Hash {
	return vfs.Hash(blake3.Sum256(data))
}

func main() {
	if err := generateTreeFixtures(); err != nil {
		fmt.Fprintf(os.Stderr, "tree fixtures error: %v\n", err)
		os.Exit(1)
	}
	if err := generateBundleFixtures(); err != nil {
		fmt.Fprintf(os.Stderr, "bundle fixtures error: %v\n", err)
		os.Exit(1)
	}
	if err := generateRecordingFixtures(); err != nil {
		fmt.Fprintf(os.Stderr, "recording fixtures error: %v\n", err)
		os.Exit(1)
	}
	if err := generateTranscriptFixtures(); err != nil {
		fmt.Fprintf(os.Stderr, "transcript fixtures error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("All fixtures generated successfully.")
}

func generateTreeFixtures() error {
	fixtures := []TreeFixture{}

	// 1. Empty tree
	emptyTree := vfs.Tree{}
	emptyEncoded := emptyTree.Encode()
	emptyHash := blake3.Sum256(emptyEncoded)
	fixtures = append(fixtures, TreeFixture{
		Name:        "empty_tree",
		Description: "The empty tree with zero entries",
		Entries:     []TreeEntry{},
		EncodedHex:  hex.EncodeToString(emptyEncoded),
		HashHex:     hex.EncodeToString(emptyHash[:]),
	})

	// 2. Single file
	hHello := mustHash([]byte("hello world\n"))
	singleTree := vfs.Tree{
		{
			Name: "hello.txt",
			Kind: vfs.KindFile,
			Mode: 0o644,
			Size: 12,
			Hash: hHello,
		},
	}
	singleEnc := singleTree.Encode()
	singleHash := blake3.Sum256(singleEnc)
	fixtures = append(fixtures, TreeFixture{
		Name:        "single_file",
		Description: "Tree with a single hello.txt file",
		Entries: []TreeEntry{
			{Name: "hello.txt", Kind: "file", Mode: 0o644, Size: 12, Hash: hex.EncodeToString(hHello[:])},
		},
		EncodedHex: hex.EncodeToString(singleEnc),
		HashHex:    hex.EncodeToString(singleHash[:]),
	})

	// 3. Multi-entry sorted tree
	hMain := mustHash([]byte("package main\n\nfunc main() {}\n"))
	hSubTree := mustHash([]byte{0, 0, 0, 0}) // pseudo tree hash
	multiTree := vfs.Tree{
		{
			Name: "README.md",
			Kind: vfs.KindFile,
			Mode: 0o644,
			Size: 12,
			Hash: hHello,
		},
		{
			Name: "main.go",
			Kind: vfs.KindFile,
			Mode: 0o644,
			Size: int64(len("package main\n\nfunc main() {}\n")),
			Hash: hMain,
		},
		{
			Name: "subpkg",
			Kind: vfs.KindDir,
			Mode: 0o755,
			Size: 0,
			Hash: hSubTree,
		},
	}
	multiEnc := multiTree.Encode()
	multiHash := blake3.Sum256(multiEnc)
	fixtures = append(fixtures, TreeFixture{
		Name:        "multi_sorted_entries",
		Description: "Tree with multiple entries in lexicographical sort order",
		Entries: []TreeEntry{
			{Name: "README.md", Kind: "file", Mode: 0o644, Size: 12, Hash: hex.EncodeToString(hHello[:])},
			{Name: "main.go", Kind: "file", Mode: 0o644, Size: int64(len("package main\n\nfunc main() {}\n")), Hash: hex.EncodeToString(hMain[:])},
			{Name: "subpkg", Kind: "dir", Mode: 0o755, Size: 0, Hash: hex.EncodeToString(hSubTree[:])},
		},
		EncodedHex: hex.EncodeToString(multiEnc),
		HashHex:    hex.EncodeToString(multiHash[:]),
	})

	out, err := json.MarshalIndent(fixtures, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("spec/conformance/trees/vectors.json", out, 0o600)
}

func generateBundleFixtures() error {
	dir := "spec/conformance/bundles"
	manifestPath := filepath.Join(dir, "manifest.json")

	type BundleExpectation struct {
		Filename      string `json:"filename"`
		Description   string `json:"description"`
		ExpectedValid bool   `json:"expected_valid"`
		ExpectedError string `json:"expected_error,omitempty"`
	}

	expectations := []BundleExpectation{}

	// 1. Valid bundle
	validZipPath := filepath.Join(dir, "bundle_valid.zip")
	store := vfs.NewMemStore()
	w := vfs.NewWorld(store)
	if err := w.WriteFile("test.txt", []byte("hello zygote conformance"), 0o644); err != nil {
		return err
	}
	snap := w.Snapshot()
	rec := trace.Recording{
		Version:       1,
		HashAlgorithm: "blake3",
		Producer:      "zygote/conformance-gen",
		HasRawPrompts: false,
		Steps: []trace.Step{
			{
				N:       0,
				Name:    "init",
				Root:    hex.EncodeToString(snap.Root[:]),
				Effects: 1,
			},
		},
		Effects: []journal.Entry{
			{
				Seq:   0,
				Op:    "model",
				Key:   "test-key",
				Value: []byte("model-output"),
			},
		},
	}
	if err := trace.ExportFile(validZipPath, store, rec); err != nil {
		return err
	}
	expectations = append(expectations, BundleExpectation{
		Filename:      "bundle_valid.zip",
		Description:   "Valid bundle with full reachability and intact hashes",
		ExpectedValid: true,
	})

	// 2. Corrupt object hash mismatch
	corruptZipPath := filepath.Join(dir, "bundle_corrupt_hash.zip")
	if err := createZipWithTamperedObject(validZipPath, corruptZipPath); err != nil {
		return err
	}
	expectations = append(expectations, BundleExpectation{
		Filename:      "bundle_corrupt_hash.zip",
		Description:   "Bundle where an object in objects/ does not match its BLAKE3 hash filename",
		ExpectedValid: false,
		ExpectedError: "corrupt bundle: object content does not match its hash filename",
	})

	// 3. Truncated / missing object
	truncatedZipPath := filepath.Join(dir, "bundle_missing_object.zip")
	if err := createZipMissingObject(validZipPath, truncatedZipPath); err != nil {
		return err
	}
	expectations = append(expectations, BundleExpectation{
		Filename:      "bundle_missing_object.zip",
		Description:   "Bundle missing an object required by step reachability",
		ExpectedValid: false,
		ExpectedError: "corrupt bundle: recording relies on objects not present in the bundle",
	})

	// 4. Traversal attack
	traversalZipPath := filepath.Join(dir, "bundle_traversal_attack.zip")
	if err := createZipTraversalAttack(traversalZipPath); err != nil {
		return err
	}
	expectations = append(expectations, BundleExpectation{
		Filename:      "bundle_traversal_attack.zip",
		Description:   "Bundle containing a path traversal attack (e.g. ../../etc/passwd)",
		ExpectedValid: false,
		ExpectedError: "path traversal detected",
	})

	out, err := json.MarshalIndent(expectations, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, out, 0o600)
}

func createZipWithTamperedObject(srcZip, dstZip string) error {
	r, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer r.Close()

	dst, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer dst.Close()

	zw := zip.NewWriter(dst)
	defer zw.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data := make([]byte, f.UncompressedSize64)
		_, _ = rc.Read(data)
		_ = rc.Close()

		if len(f.Name) > 8 && f.Name[:8] == "objects/" {
			// tamper with payload
			data = []byte("tampered data which does not match filename hash")
		}

		w, err := zw.Create(f.Name)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func createZipMissingObject(srcZip, dstZip string) error {
	r, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer r.Close()

	dst, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer dst.Close()

	zw := zip.NewWriter(dst)
	defer zw.Close()

	for _, f := range r.File {
		if len(f.Name) > 8 && f.Name[:8] == "objects/" {
			// skip an object!
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}
		data := make([]byte, f.UncompressedSize64)
		_, _ = rc.Read(data)
		_ = rc.Close()

		w, err := zw.Create(f.Name)
		if err != nil {
			return err
		}
		if _, err := w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func createZipTraversalAttack(dstZip string) error {
	dst, err := os.Create(dstZip)
	if err != nil {
		return err
	}
	defer dst.Close()

	zw := zip.NewWriter(dst)
	defer zw.Close()

	// 1. trace.json
	rec := trace.Recording{
		Version:       1,
		HashAlgorithm: "blake3",
		Producer:      "traversal-test",
	}
	recBytes, _ := json.Marshal(rec)
	w, err := zw.Create("trace.json")
	if err != nil {
		return err
	}
	_, _ = w.Write(recBytes)

	// 2. Traversal entry
	tw, err := zw.Create("../../evil.txt")
	if err != nil {
		return err
	}
	_, _ = tw.Write([]byte("malicious content"))

	return nil
}

func generateRecordingFixtures() error {
	type RecordingFixture struct {
		Name          string          `json:"name"`
		Description   string          `json:"description"`
		Recording     trace.Recording `json:"recording"`
		ReplayEffects []journal.Entry `json:"replay_effects"`
		ReplayWorld   string          `json:"replay_world"` // hex hash
		ExpectedMatch bool            `json:"expected_match"`
		ExpectedClass string          `json:"expected_class,omitempty"` // divergence class
	}

	recWorldHash := "ec2bd03bf86b935fa34d71ad7ebb049f1f10f87d343e521511d8f9e6625620cd"
	diffWorldHash := "1111111111111111111111111111111111111111111111111111111111111111"

	baseRec := trace.Recording{
		Version:       1,
		HashAlgorithm: "blake3",
		Producer:      "zygote/conformance-gen",
		Steps: []trace.Step{
			{
				N:       0,
				Name:    "step0",
				Root:    recWorldHash,
				Effects: 1,
			},
		},
		Effects: []journal.Entry{
			{
				Seq:   0,
				Op:    "model",
				Key:   "prompt-1",
				Value: []byte("response-1"),
			},
		},
	}

	fixtures := []RecordingFixture{
		{
			Name:        "match_exact",
			Description: "Replay matching recorded effect and world hash",
			Recording:   baseRec,
			ReplayEffects: []journal.Entry{
				{Seq: 0, Op: "model", Key: "prompt-1", Value: []byte("response-1")},
			},
			ReplayWorld:   recWorldHash,
			ExpectedMatch: true,
		},
		{
			Name:        "effect_mismatch",
			Description: "Replay with changed prompt or tool op/key",
			Recording:   baseRec,
			ReplayEffects: []journal.Entry{
				{Seq: 0, Op: "model", Key: "different-prompt", Value: []byte("response-1")},
			},
			ReplayWorld:   recWorldHash,
			ExpectedMatch: false,
			ExpectedClass: "effect-mismatch",
		},
		{
			Name:        "world_mismatch",
			Description: "Replay where effects match but the resulting world hash diverges",
			Recording:   baseRec,
			ReplayEffects: []journal.Entry{
				{Seq: 0, Op: "model", Key: "prompt-1", Value: []byte("response-1")},
			},
			ReplayWorld:   diffWorldHash,
			ExpectedMatch: false,
			ExpectedClass: "world-mismatch",
		},
		{
			Name:        "exhausted",
			Description: "Replay where agent requests more effects than recorded",
			Recording:   baseRec,
			ReplayEffects: []journal.Entry{
				{Seq: 0, Op: "model", Key: "prompt-1", Value: []byte("response-1")},
				{Seq: 1, Op: "model", Key: "prompt-2", Value: []byte("response-2")},
			},
			ReplayWorld:   recWorldHash,
			ExpectedMatch: false,
			ExpectedClass: "exhausted",
		},
	}

	out, err := json.MarshalIndent(fixtures, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("spec/conformance/recordings/fixtures.json", out, 0o600)
}

func generateTranscriptFixtures() error {
	type TranscriptMessage struct {
		Direction string          `json:"direction"` // "agent->zyg" or "zyg->agent"
		Raw       json.RawMessage `json:"raw"`
	}

	type TranscriptFixture struct {
		Name        string              `json:"name"`
		Description string              `json:"description"`
		Messages    []TranscriptMessage `json:"messages"`
	}

	fixtures := []TranscriptFixture{
		{
			Name:        "handshake_and_step",
			Description: "Standard handshake followed by step checkpoint",
			Messages: []TranscriptMessage{
				{
					Direction: "agent->zyg",
					Raw:       json.RawMessage(`{"jsonrpc":"2.0","id":1,"method":"handshake","params":{"version":1}}`),
				},
				{
					Direction: "zyg->agent",
					Raw:       json.RawMessage(`{"jsonrpc":"2.0","id":1,"result":{"version":1}}`),
				},
				{
					Direction: "agent->zyg",
					Raw:       json.RawMessage(`{"jsonrpc":"2.0","id":2,"method":"step","params":{"name":"init"}}`),
				},
				{
					Direction: "zyg->agent",
					Raw:       json.RawMessage(`{"jsonrpc":"2.0","id":2,"result":{"match":true,"expected":"ec2bd03bf86b935fa34d71ad7ebb049f1f10f87d343e521511d8f9e6625620cd","actual":"ec2bd03bf86b935fa34d71ad7ebb049f1f10f87d343e521511d8f9e6625620cd"}}`),
				},
			},
		},
		{
			Name:        "effect_record",
			Description: "Recording a model effect over harness protocol",
			Messages: []TranscriptMessage{
				{
					Direction: "agent->zyg",
					Raw:       json.RawMessage(`{"jsonrpc":"2.0","id":3,"method":"effect","params":{"op":"model","key":"plan-fix","value":"eyJwbGFuIjoidXBkYXRlIn0="}}`),
				},
				{
					Direction: "zyg->agent",
					Raw:       json.RawMessage(`{"jsonrpc":"2.0","id":3,"result":{"value":"eyJwbGFuIjoidXBkYXRlIn0="}}`),
				},
			},
		},
	}

	out, err := json.MarshalIndent(fixtures, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("spec/conformance/transcripts/transcripts.json", out, 0o600)
}
