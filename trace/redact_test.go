package trace

import (
	"encoding/hex"
	"testing"

	"github.com/mindfire/zygote/pkg/redact"
	"github.com/mindfire/zygote/vfs"
)

func TestRedactBundle(t *testing.T) {
	// Create a fake bundle with a secret in effects and a secret in VFS
	store := vfs.NewMemStore()
	world := vfs.NewWorld(store)
	_ = world.WriteFile("secret.txt", []byte("My token is AKIAIOSFODNN7EXAMPLE !"), 0644)
	r := Record(store)
	// r.world needs the file, wait r.world is a DIFFERENT world!
	// We can just use r.World()
	_ = r.World().WriteFile("secret.txt", []byte("My token is AKIAIOSFODNN7EXAMPLE !"), 0644)
	r.Step("init")

	_, _ = r.Recorder().Record("model", "q1", func() ([]byte, error) {
		return []byte("I leaked AKIAIOSFODNN7EXAMPLE !!"), nil
	})
	r.Step("step1")

	inBundle := &Bundle{
		Recording: r.Recording(),
		Store:     store,
	}

	policy := &redact.Policy{
		Rules: []redact.PolicyRule{
			{ID: "aws-access-token", Action: redact.ActionRedact},
		},
	}

	cleanBundle, err := RedactBundle(inBundle, policy)
	if err != nil {
		t.Fatal(err)
	}

	if len(cleanBundle.Recording.Effects) != 1 {
		t.Fatal("expected 1 effect")
	}

	effValue := string(cleanBundle.Recording.Effects[0].Value)
	if effValue != "I leaked *** !!" {
		t.Fatalf("effect not redacted correctly: %s", effValue)
	}

	// Verify VFS hash changed
	oldRoot := inBundle.Recording.Steps[1].Root
	newRoot := cleanBundle.Recording.Steps[1].Root
	if oldRoot == newRoot {
		t.Fatal("expected root hash to change after redaction")
	}

	// Verify the hash is valid
	newRootHash, _ := hex.DecodeString(newRoot)
	var h vfs.Hash
	copy(h[:], newRootHash)

	_, ok := cleanBundle.Store.Get(h)
	if !ok {
		t.Fatal("new root object not found in store")
	}
}
