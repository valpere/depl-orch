// Package model builds a backend-agnostic tool-calling chat model for the
// agentic recovery steps. Selection and configuration are env-driven so the
// binary needs no rebuild to change provider (requirements §2.5, §4).
package model

import (
	"fmt"
	"os"
	"strconv"
)

// Config selects and configures the recovery model backend.
type Config struct {
	Backend   string // ollama | openai | anthropic
	Model     string // model id
	BaseURL   string // endpoint (ollama / openai-compatible)
	APIKey    string // for openai / anthropic
	MaxTokens int    // response cap
}

// Load reads the model configuration from the environment.
//
//	MODEL_BACKEND     ollama | openai | anthropic   (default "ollama")
//	MODEL_ID          model id                       (required)
//	MODEL_BASE_URL    endpoint                       (default Ollama local)
//	MODEL_API_KEY     api key (openai / anthropic)
//	MODEL_MAX_TOKENS  response cap                   (default 4096)
func Load() (Config, error) {
	c := Config{
		Backend:   getenv("MODEL_BACKEND", "ollama"),
		Model:     os.Getenv("MODEL_ID"),
		BaseURL:   getenv("MODEL_BASE_URL", "http://localhost:11434"),
		APIKey:    os.Getenv("MODEL_API_KEY"),
		MaxTokens: getenvInt("MODEL_MAX_TOKENS", 4096),
	}
	if c.Model == "" {
		return Config{}, fmt.Errorf("MODEL_ID is required")
	}
	return c, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
