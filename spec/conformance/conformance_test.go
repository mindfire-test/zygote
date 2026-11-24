package conformance_test

import (
	"archive/zip"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mindfire/zygote/harness"
	"github.com/mindfire/zygote/journal"
	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
	"lukechampine.com/blake3"
)

// 1. Tree encoding conformance
type TreeFixture struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Entries     []TreeEntry `json:"entries"`
	EncodedHex  string      `json:"encoded_hex"`
	HashHex     string      `json:"hash_hex"`
}

type TreeEntry struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	Mode uint32 `json:"mode"`
	Size int64  `json:"size"`
	Hash string `json:"hash"`
}

func TestConformance_Trees(t *testing.T) {
	data, err := os.ReadFile("trees/vectors.json")
	if err != nil {
		t.Fatalf("failed to read tree vectors: %v", err)
	}

	var fixtures []TreeFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("failed to unmarshal tree fixtures: %v", err)
	}

	for _, tc := range fixtures {
		t.Run(tc.Name, func(t *testing.T) {
			var tree vfs.Tree
			for _, e := range tc.Entries {
				var h vfs.Hash
				hb, err := hex.DecodeString(e.Hash)
				if err != nil {
					t.Fatalf("invalid fixture hash hex: %v", err)
				}
				copy(h[:], hb)

				kind := vfs.KindFile
				if e.Kind == "dir" {
					kind = vfs.KindDir
				}

				tree = append(tree, vfs.Entry{
					Name: e.Name,
					Kind: kind,
					Mode: e.Mode,
					Size: e.Size,
					Hash: h,
				})
			}

			// Test encoding matches expected canonical bytes
			encoded := tree.Encode()
			gotEncodedHex := hex.EncodeToString(encoded)
			if gotEncodedHex != tc.EncodedHex {
				t.Errorf("encoded hex mismatch:\n got:  %s\n want: %s", gotEncodedHex, tc.EncodedHex)
			}

			// Test hash matches expected BLAKE3 hash
			hash := blake3.Sum256(encoded)
			gotHashHex := hex.EncodeToString(hash[:])
			if gotHashHex != tc.HashHex {
				t.Errorf("hash hex mismatch:\n got:  %s\n want: %s", gotHashHex, tc.HashHex)
			}

			// Test round-trip decoding
			decodedTree, err := vfs.DecodeTree(encoded)
			if err != nil {
				t.Fatalf("failed to decode tree bytes: %v", err)
			}

			reEncoded := decodedTree.Encode()
			if !bytes.Equal(encoded, reEncoded) {
				t.Errorf("round-trip re-encoded bytes did not match original canonical bytes")
			}
		})
	}
}

// 2. Bundle conformance
type BundleExpectation struct {
	Filename      string `json:"filename"`
	Description   string `json:"description"`
	ExpectedValid bool   `json:"expected_valid"`
	ExpectedError string `json:"expected_error,omitempty"`
}

func TestConformance_Bundles(t *testing.T) {
	manifestPath := filepath.Join("bundles", "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read bundle manifest: %v", err)
	}

	var expectations []BundleExpectation
	if err := json.Unmarshal(data, &expectations); err != nil {
		t.Fatalf("failed to unmarshal bundle expectations: %v", err)
	}

	for _, exp := range expectations {
		t.Run(exp.Filename, func(t *testing.T) {
			zipPath := filepath.Join("bundles", exp.Filename)
			store := vfs.NewMemStore()
			bundle, err := trace.ImportFile(zipPath, store)

			if exp.ExpectedValid {
				if err != nil {
					t.Fatalf("expected bundle %s to be valid, got error: %v", exp.Filename, err)
				}
				if bundle.Recording.Version != 1 {
					t.Errorf("expected version 1, got %d", bundle.Recording.Version)
				}
			} else if err == nil {
				t.Fatalf("expected bundle %s to be invalid, but ImportFile succeeded", exp.Filename)
			}
		})
	}
}

// 3. Replay & Divergence conformance
type RecordingFixture struct {
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Recording     trace.Recording `json:"recording"`
	ReplayEffects []journal.Entry `json:"replay_effects"`
	ReplayWorld   string          `json:"replay_world"` // hex hash
	ExpectedMatch bool            `json:"expected_match"`
	ExpectedClass string          `json:"expected_class,omitempty"` // divergence class
}

func TestConformance_Recordings(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("recordings", "fixtures.json"))
	if err != nil {
		t.Fatalf("failed to read recording fixtures: %v", err)
	}

	var fixtures []RecordingFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("failed to unmarshal recording fixtures: %v", err)
	}

	for _, fix := range fixtures {
		t.Run(fix.Name, func(t *testing.T) {
			replayer := journal.NewReplayer(fix.Recording.Effects)

			var divergenceErr *journal.DivergenceError
			for _, eff := range fix.ReplayEffects {
				_, err := replayer.Replay(eff.Op, eff.Key)
				if err != nil {
					if div, ok := err.(*journal.DivergenceError); ok {
						divergenceErr = div
						break
					}
					t.Fatalf("unexpected non-divergence error: %v", err)
				}
			}

			// If effects did not diverge, check world state
			if divergenceErr == nil {
				if len(fix.Recording.Steps) > 0 {
					expectedRoot := fix.Recording.Steps[0].Root
					if expectedRoot != fix.ReplayWorld {
						divergenceErr = &journal.DivergenceError{
							Class:   journal.ClassWorldMismatch,
							Message: "world root hash does not match recording",
							Step:    0,
						}
					}
				}
			}

			if fix.ExpectedMatch {
				if divergenceErr != nil {
					t.Fatalf("expected match, got divergence: %v", divergenceErr)
				}
			} else {
				if divergenceErr == nil {
					t.Fatalf("expected divergence with class %s, but run matched", fix.ExpectedClass)
				}
				if string(divergenceErr.Class) != fix.ExpectedClass {
					t.Errorf("divergence class mismatch: got %s, want %s", divergenceErr.Class, fix.ExpectedClass)
				}
			}
		})
	}
}

// 4. Harness Protocol Transcript conformance
type TranscriptMessage struct {
	Direction string          `json:"direction"` // "agent->zyg" or "zyg->agent"
	Raw       json.RawMessage `json:"raw"`
}

type TranscriptFixture struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Messages    []TranscriptMessage `json:"messages"`
}

type mockHarnessHandler struct{}

func (m *mockHarnessHandler) Handshake(_ int) (int, error) {
	return 1, nil
}

func (m *mockHarnessHandler) Effect(_, _ string, value []byte) ([]byte, error) {
	return value, nil
}

func (m *mockHarnessHandler) Step(_ string) (*harness.StepResult, error) {
	h := "ec2bd03bf86b935fa34d71ad7ebb049f1f10f87d343e521511d8f9e6625620cd"
	return &harness.StepResult{
		Match:    true,
		Expected: h,
		Actual:   h,
	}, nil
}

func TestConformance_Transcripts(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("transcripts", "transcripts.json"))
	if err != nil {
		t.Fatalf("failed to read transcripts fixture: %v", err)
	}

	var fixtures []TranscriptFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("failed to unmarshal transcript fixtures: %v", err)
	}

	for _, fix := range fixtures {
		t.Run(fix.Name, func(t *testing.T) {
			var inBuf bytes.Buffer
			for _, msg := range fix.Messages {
				if msg.Direction == "agent->zyg" {
					var compact bytes.Buffer
					if err := json.Compact(&compact, msg.Raw); err != nil {
						t.Fatalf("failed to compact JSON: %v", err)
					}
					inBuf.Write(compact.Bytes())
					inBuf.WriteByte('\n')
				}
			}

			var outBuf bytes.Buffer
			handler := &mockHarnessHandler{}
			srv := harness.NewServer(&inBuf, &outBuf, handler)

			if err := srv.Serve(); err != nil {
				t.Fatalf("harness server returned error: %v", err)
			}

			// Validate outgoing messages
			outLines := bytes.Split(bytes.TrimSpace(outBuf.Bytes()), []byte("\n"))
			idx := 0
			for _, msg := range fix.Messages {
				if msg.Direction == "zyg->agent" {
					if idx >= len(outLines) {
						t.Fatalf("missing response for message %d", idx)
					}
					var expectedResp, actualResp harness.Response
					if err := json.Unmarshal(msg.Raw, &expectedResp); err != nil {
						t.Fatalf("invalid expected response JSON: %v", err)
					}
					if err := json.Unmarshal(outLines[idx], &actualResp); err != nil {
						t.Fatalf("invalid actual response JSON: %v", err)
					}

					if expectedResp.JSONRPC != actualResp.JSONRPC {
						t.Errorf("jsonrpc version mismatch: got %s, want %s", actualResp.JSONRPC, expectedResp.JSONRPC)
					}
					idx++
				}
			}
		})
	}
}

// 5. Test Bundle Reader standalone implementation (M4 Exit Criterion)
// A third party implementing a bundle reader from spec alone passes the bundle reader test.
func TestConformance_StandaloneBundleReader(t *testing.T) {
	validBundlePath := filepath.Join("bundles", "bundle_valid.zip")

	// Read bundle using standard archive/zip only (pure reader according to spec)
	zr, err := zip.OpenReader(validBundlePath)
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer zr.Close()

	foundTrace := false
	objectCount := 0

	for _, file := range zr.File {
		if file.Name == "trace.json" {
			foundTrace = true
			rc, err := file.Open()
			if err != nil {
				t.Fatalf("failed to read trace.json: %v", err)
			}
			var rec trace.Recording
			if err := json.NewDecoder(rc).Decode(&rec); err != nil {
				_ = rc.Close()
				t.Fatalf("failed to decode trace.json: %v", err)
			}
			_ = rc.Close()

			if rec.Version != 1 {
				t.Errorf("expected version 1, got %d", rec.Version)
			}
			if rec.HashAlgorithm != "blake3" {
				t.Errorf("expected hash algorithm blake3, got %s", rec.HashAlgorithm)
			}
		} else if len(file.Name) > 8 && file.Name[:8] == "objects/" {
			objectCount++
			rc, err := file.Open()
			if err != nil {
				t.Fatalf("failed to read object %s: %v", file.Name, err)
			}
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(rc)
			_ = rc.Close()

			expectedHash := file.Name[8:]
			actualHash := blake3.Sum256(buf.Bytes())
			actualHex := hex.EncodeToString(actualHash[:])
			if actualHex != expectedHash {
				t.Errorf("object %s hash mismatch: got %s", file.Name, actualHex)
			}
		}
	}

	if !foundTrace {
		t.Errorf("trace.json not found in valid bundle")
	}
	if objectCount == 0 {
		t.Errorf("no objects found in valid bundle")
	}
}
