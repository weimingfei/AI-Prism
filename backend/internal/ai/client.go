package ai

import (
	"context"
	"strings"

	"github.com/ai-prism/backend/internal/config"
)

type providerOverrideKey struct{}

type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature float32
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Content string
	Raw     []byte
}

type Client interface {
	Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)
	StreamChat(ctx context.Context, request ChatRequest) (<-chan ChatResponse, <-chan error)
}

func WithProviderOverride(ctx context.Context, cfg config.AIConfig) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(cfg.Provider) == "" &&
		strings.TrimSpace(cfg.BaseURL) == "" &&
		strings.TrimSpace(cfg.APIKey) == "" &&
		strings.TrimSpace(cfg.Model) == "" {
		return ctx
	}
	cfg.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.Provider = strings.TrimSpace(cfg.Provider)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.Model = strings.TrimSpace(cfg.Model)
	return context.WithValue(ctx, providerOverrideKey{}, cfg)
}

func providerOverrideFromContext(ctx context.Context) (config.AIConfig, bool) {
	if ctx == nil {
		return config.AIConfig{}, false
	}
	cfg, ok := ctx.Value(providerOverrideKey{}).(config.AIConfig)
	return cfg, ok
}
