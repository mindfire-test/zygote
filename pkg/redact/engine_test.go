package redact_test

import (
	"bytes"
	"testing"

	"github.com/mindfire/zygote/pkg/redact"
)

func TestEngineRedact(t *testing.T) {
	policy := &redact.Policy{
		Rules: []redact.PolicyRule{
			{ID: "aws-access-token", Action: redact.ActionRedact},
		},
	}
	engine, err := redact.NewEngine(policy)
	if err != nil {
		t.Fatal(err)
	}

	content := []byte("hello AKIAIOSFODNN7EXAMPLE world")
	res, err := engine.Process("test.txt", content)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(res.Content, []byte("hello *** world")) {
		t.Fatalf("Expected redacted string, got %s", res.Content)
	}
}

func TestEngineFailClosed(t *testing.T) {
	policy := &redact.Policy{} // Empty policy = fail closed
	engine, err := redact.NewEngine(policy)
	if err != nil {
		t.Fatal(err)
	}

	content := []byte("hello AKIAIOSFODNN7EXAMPLE world")
	_, err = engine.Process("test.txt", content)
	if err == nil {
		t.Fatal("Expected error due to fail closed")
	}
}

func TestEnginePathDigestOnly(t *testing.T) {
	policy := &redact.Policy{
		Rules: []redact.PolicyRule{
			{Path: "*.csv", Action: redact.ActionDigestOnly},
		},
	}
	engine, err := redact.NewEngine(policy)
	if err != nil {
		t.Fatal(err)
	}

	res, err := engine.Process("data.csv", []byte("huge data"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.DigestOnly {
		t.Fatal("Expected DigestOnly to be true")
	}
}
