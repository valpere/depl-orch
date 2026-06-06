package model

import (
	"context"
	"testing"
)

func TestNew_UnknownBackend(t *testing.T) {
	if _, err := New(context.Background(), Config{Backend: "bogus", Model: "m"}); err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestLoad_RequiresModelID(t *testing.T) {
	t.Setenv("MODEL_ID", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when MODEL_ID is empty")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("MODEL_ID", "qwen3-coder-next:cloud")
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Backend != "ollama" {
		t.Errorf("default backend = %q, want ollama", c.Backend)
	}
	if c.Model != "qwen3-coder-next:cloud" {
		t.Errorf("model = %q", c.Model)
	}
	if c.MaxTokens != 4096 {
		t.Errorf("default MaxTokens = %d, want 4096", c.MaxTokens)
	}
}
