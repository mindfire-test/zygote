package harness_test

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mindfire/zygote/harness"
	"github.com/mindfire/zygote/trace"
	"github.com/mindfire/zygote/vfs"
)

func TestTypescriptClientParity(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not found")
	}

	cwd, _ := os.Getwd()
	tsClientDir := filepath.Join(cwd, "..", "clients", "typescript")

	// Ensure the typescript package is built
	if _, err := os.Stat(filepath.Join(tsClientDir, "dist", "src", "index.js")); err != nil {
		t.Skip("TypeScript package not built. Run 'npm run build' first.")
	}

	script := `
const { ZygoteClient } = require(process.env.TS_CLIENT_DIR + '/dist/src/index.js');

console.log("Starting TS agent...");

const client = new ZygoteClient();

async function run() {
    await client.handshake(1);

    const ans = await client.effect("model", "q1", Buffer.from("hello ts"));

    console.log("Got answer:", ans.toString());

    await client.step("step-1");
    client.close();
}

run().catch(e => {
    console.error(e);
    process.exit(1);
});
`

	store := vfs.NewMemStore()
	r := trace.Record(store)
	r.Set("agent", "ts-tester")
	r.Step("init")

	cmd := exec.CommandContext(context.Background(), "node", "-e", script)
	cmd.Env = append(os.Environ(), "TS_CLIENT_DIR="+tsClientDir)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	srv := harness.NewServer(stdoutPipe, stdinPipe, harness.NewRecordHandler(r))

	errc := make(chan error, 1)
	go func() {
		errc <- srv.Serve()
	}()

	cmdErr := cmd.Wait()
	srvErr := <-errc

	if srvErr != nil && srvErr != io.EOF {
		t.Fatalf("Harness server error: %v", srvErr)
	}
	if cmdErr != nil {
		t.Fatalf("Agent error: %v", cmdErr)
	}

	rec := r.Recording()
	if len(rec.Steps) != 2 {
		t.Fatalf("Expected 2 steps, got %d", len(rec.Steps))
	}
	if rec.Steps[1].Name != "step-1" {
		t.Fatalf("Expected step-1")
	}

	if len(rec.Effects) != 1 {
		t.Fatalf("Expected 1 effect, got %d", len(rec.Effects))
	}

	if rec.Effects[0].Op != "model" || rec.Effects[0].Key != "q1" {
		t.Fatalf("Effect mismatch: %v", rec.Effects[0])
	}

	if string(rec.Effects[0].Value) != "hello ts" {
		t.Fatalf("Expected effect value 'hello ts', got '%s'", string(rec.Effects[0].Value))
	}
}
