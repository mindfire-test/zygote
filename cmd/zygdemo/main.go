// Package main provides the end-to-end Zygote M0 demonstration.
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/mindfire/zygote/journal"
	"github.com/mindfire/zygote/source/localdir"
	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
)

func main() {
	fmt.Println("=== Zygote M0 End-to-End Demonstration ===")

	// 1. Setup Physical Workspace
	workspace, err := os.MkdirTemp("", "zygote-demo-*")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(workspace) }()

	_ = os.WriteFile(filepath.Join(workspace, "main.go"), []byte("package main\n\nfunc main() {\n\t// TODO\n}\n"), 0644) //nolint:gosec
	fmt.Printf("[1] Created physical workspace at %s\n", workspace)

	// 2. Ingest into Zygote VFS
	ctx := context.Background()
	store := vfs.NewMemStore()
	r := trace.Record(store)
	r.Set("agent", "zygdemo-bot")

	adapter := localdir.New(workspace)
	if err := adapter.Load(ctx, r.World()); err != nil {
		panic(err)
	}
	r.Step("init")
	rSnap := r.World().Snapshot().Root
	fmt.Printf("[2] Loaded workspace into cryptographic VFS (Root Hash: %x)\n", rSnap[:4])

	// 3. Simulate Agent Execution (Record Mode)
	// Agent asks LLM for code
	llmResponse, _ := journal.ModelRecord(r.Recorder(), "prompt-1", func() (string, error) {
		return "fmt.Println(\"Hello World\")", nil
	})

	// Agent edits file in VFS
	b, _ := fs.ReadFile(r.World(), "main.go")
	newCode := string(b) + "\n// Agent says: " + llmResponse
	_ = r.World().WriteFile("main.go", []byte(newCode), 0644) //nolint:gosec

	r.Step("agent-edit")
	rSnap2 := r.World().Snapshot().Root
	fmt.Printf("[3] Agent executed. Mock LLM called. Step captured (Root Hash: %x)\n", rSnap2[:4])

	// 4. Branching (Forks)
	forkA := vfs.Fork(store, r.World().Snapshot())
	forkB := vfs.Fork(store, r.World().Snapshot())

	_ = forkA.WriteFile("a.txt", []byte("fork A"), 0644) //nolint:gosec
	_ = forkB.WriteFile("b.txt", []byte("fork B"), 0644) //nolint:gosec
	snapA := forkA.Snapshot().Root
	snapB := forkB.Snapshot().Root
	fmt.Printf("[4] Forked VFS in memory. Fork A root: %x..., Fork B root: %x...\n", snapA[:4], snapB[:4])

	// Apply winner back to disk (we choose the main world, not a fork, just for simplicity, or we could apply forkA)
	if err := adapter.Apply(ctx, forkA); err != nil {
		panic(err)
	}
	fmt.Printf("    Applied Fork A back to physical disk securely.\n")

	// 5. Bundle to Zip
	bundlePath := filepath.Join(workspace, "run.zip")
	if err := trace.ExportFile(bundlePath, store, r.Recording()); err != nil {
		panic(err)
	}
	info, _ := os.Stat(bundlePath)
	fmt.Printf("[5] Exported portable bundle to run.zip (%d bytes)\n", info.Size())

	// 6. Air-gapped Replay
	fmt.Println("\n--- Transporting bundle to simulated new machine ---")
	newStore := vfs.NewMemStore()
	bundle, err := trace.ImportFile(bundlePath, newStore)
	if err != nil {
		panic(err)
	}
	fmt.Println("[6] Bundle imported and cryptographically verified.")

	replayer := journal.NewReplayer(bundle.Recording.Effects)

	hBytes, _ := hex.DecodeString(bundle.Recording.Steps[0].Root)
	var h vfs.Hash
	copy(h[:], hBytes)
	replayWorld := vfs.Fork(newStore, vfs.Snapshot{Root: h})

	fmt.Println("    Replaying Agent...")
	// Replay LLM
	ans, err := journal.ModelReplay(replayer, "prompt-1")
	if err != nil {
		panic(err)
	}
	fmt.Printf("    Intercepted LLM call during replay without network: %s\n", ans)

	// Replay Agent VFS Edit
	b2, _ := fs.ReadFile(replayWorld, "main.go")
	newCode2 := string(b2) + "\n// Agent says: " + ans
	_ = replayWorld.WriteFile("main.go", []byte(newCode2), 0644) //nolint:gosec

	// 7. Divergence Detection
	// Intentionally diverge!
	_ = replayWorld.WriteFile("rogue.txt", []byte("I am a bad agent"), 0644) //nolint:gosec
	fmt.Println("[7] Agent intentionally diverged during replay by writing rogue.txt")

	replaySnap := replayWorld.Snapshot()
	expectedHex := bundle.Recording.Steps[1].Root
	actualHex := fmt.Sprintf("%x", replaySnap.Root)

	if expectedHex != actualHex {
		fmt.Printf("    [!] DIVERGENCE DETECTED! Expected root: %s, Actual root: %s\n", expectedHex[:8], actualHex[:8])
	}

	fmt.Println("\n=== M0 Demonstration Complete ===")
}
