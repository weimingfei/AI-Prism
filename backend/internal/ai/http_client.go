package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ai-prism/backend/internal/config"
)

var ErrNotConfigured = errors.New("ai client is not configured")

type HTTPClient struct {
	provider    string
	baseURL     string
	apiKey      string
	model       string
	temperature float32
	client      *http.Client
}

func NewHTTPClient(cfg config.AIConfig) *HTTPClient {
	return &HTTPClient{
		provider:    strings.TrimSpace(cfg.Provider),
		baseURL:     strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/"),
		apiKey:      strings.TrimSpace(cfg.APIKey),
		model:       strings.TrimSpace(cfg.Model),
		temperature: cfg.Temperature,
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (c *HTTPClient) Chat(ctx context.Context, request ChatRequest) (ChatResponse, error) {
	runtime := c.runtime(ctx)
	if runtime == nil || runtime.baseURL == "" || runtime.model == "" {
		return ChatResponse{}, ErrNotConfigured
	}

	model := strings.TrimSpace(request.Model)
	if model == "" {
		model = runtime.model
	}
	if override, ok := providerOverrideFromContext(ctx); ok && strings.TrimSpace(override.Model) != "" {
		model = strings.TrimSpace(override.Model)
	}
	temperature := request.Temperature
	if temperature == 0 {
		temperature = runtime.temperature
	}

	switch strings.ToLower(runtime.provider) {
	case "ollama":
		return runtime.chatOllama(ctx, model, request.Messages, temperature)
	default:
		return runtime.chatOpenAICompatible(ctx, model, request.Messages, temperature)
	}
}

func (c *HTTPClient) runtime(ctx context.Context) *HTTPClient {
	if c == nil {
		return nil
	}
	override, ok := providerOverrideFromContext(ctx)
	if !ok {
		return c
	}
	runtime := *c
	if strings.TrimSpace(override.Provider) != "" {
		runtime.provider = strings.TrimSpace(override.Provider)
	}
	if strings.TrimSpace(override.BaseURL) != "" {
		runtime.baseURL = strings.TrimRight(strings.TrimSpace(override.BaseURL), "/")
	}
	if strings.TrimSpace(override.APIKey) != "" {
		runtime.apiKey = strings.TrimSpace(override.APIKey)
	}
	if strings.TrimSpace(override.Model) != "" {
		runtime.model = strings.TrimSpace(override.Model)
	}
	if override.Temperature != 0 {
		runtime.temperature = override.Temperature
	}
	runtime.client = c.client
	return &runtime
}

func (c *HTTPClient) StreamChat(ctx context.Context, request ChatRequest) (<-chan ChatResponse, <-chan error) {
	responses := make(chan ChatResponse, 1)
	errs := make(chan error, 1)
	go func() {
		defer close(responses)
		defer close(errs)
		response, err := c.Chat(ctx, request)
		if err != nil {
			errs <- err
			return
		}
		responses <- response
	}()
	return responses, errs
}

func (c *HTTPClient) chatOllama(ctx context.Context, model string, messages []Message, temperature float32) (ChatResponse, error) {
	payload := map[string]any{
		"model":    model,
		"messages": messages,
		"stream":   false,
		"options": map[string]any{
			"temperature": temperature,
		},
	}

	body, err := c.postJSON(ctx, c.baseURL+"/api/chat", payload)
	if err != nil {
		return ChatResponse{}, err
	}

	var decoded struct {
		Message Message `json:"message"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ChatResponse{}, err
	}
	return ChatResponse{
		Content: strings.TrimSpace(decoded.Message.Content),
		Raw:     body,
	}, nil
}

func (c *HTTPClient) chatOpenAICompatible(ctx context.Context, model string, messages []Message, temperature float32) (ChatResponse, error) {
	payload := map[string]any{
		"model":       model,
		"messages":    messages,
		"temperature": temperature,
	}

	body, err := c.postJSON(ctx, c.baseURL+"/chat/completions", payload)
	if err != nil {
		return ChatResponse{}, err
	}

	var decoded struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return ChatResponse{}, err
	}
	if len(decoded.Choices) == 0 {
		return ChatResponse{}, errors.New("ai response has no choices")
	}
	return ChatResponse{
		Content: strings.TrimSpace(decoded.Choices[0].Message.Content),
		Raw:     body,
	}, nil
}

func (c *HTTPClient) postJSON(ctx context.Context, url string, payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ai request failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	return body, nil
}
