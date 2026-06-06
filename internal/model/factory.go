package model

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// New builds a tool-calling chat model for the backend named in cfg.Backend,
// mirroring the Deployer factory shape: one interface, a config-chosen impl.
func New(ctx context.Context, cfg Config) (model.ToolCallingChatModel, error) {
	switch cfg.Backend {
	case "ollama":
		return ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
			BaseURL: cfg.BaseURL,
			Model:   cfg.Model,
		})
	case "openai":
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{
			APIKey:    cfg.APIKey,
			BaseURL:   cfg.BaseURL,
			Model:     cfg.Model,
			MaxTokens: &cfg.MaxTokens,
		})
	case "anthropic":
		return claude.NewChatModel(ctx, &claude.Config{
			APIKey:    cfg.APIKey,
			Model:     cfg.Model,
			MaxTokens: cfg.MaxTokens,
		})
	default:
		return nil, fmt.Errorf("unknown model backend %q (want ollama|openai|anthropic)", cfg.Backend)
	}
}
