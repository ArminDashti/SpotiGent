// Package ai implements chat completion providers (OpenRouter, OpenCode Go,
// OpenAI, Mistral, Claude/Anthropic and Google Gemini) with tool-calling
// support, plus the agent loop that lets the model act on the user's
// Spotify account.
package ai

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ---- OpenAI-compatible wire types ----

type Message struct {
	Role       string      `json:"role"`
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type Tool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Tools       []Tool    `json:"tools,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

// ---- Provider configuration ----

// Provider identifies which AI gateway to call.
type Provider string

const (
	ProviderOpenRouter Provider = "openrouter"
	ProviderOpenCode   Provider = "opencode"
	ProviderOpenAI     Provider = "openai"
	ProviderMistral    Provider = "mistral"
	ProviderClaude     Provider = "claude"
	ProviderGoogle     Provider = "google"
)

// DefaultBaseURL returns the canonical base URL for a provider.
// Sources:
//   - OpenAI: servers.url https://api.openai.com/v1 in
//     https://github.com/openai/openai-openapi (openapi.yaml)
//   - Mistral: base https://api.mistral.ai/v1, POST /v1/chat/completions
//     (https://docs.mistral.ai/api/endpoint/chat)
//   - Claude/Anthropic: REST API at https://api.anthropic.com,
//     POST /v1/messages with x-api-key + anthropic-version headers
//     (https://platform.claude.com/docs/en/api/overview)
//   - Google Gemini OpenAI-compat: base
//     https://generativelanguage.googleapis.com/v1beta/openai/
//     (https://ai.google.dev/gemini-api/docs/openai)
func DefaultBaseURL(p Provider) string {
	switch p {
	case ProviderOpenCode:
		return "https://opencode.ai/zen/go/v1"
	case ProviderOpenAI:
		return "https://api.openai.com/v1"
	case ProviderMistral:
		return "https://api.mistral.ai/v1"
	case ProviderClaude:
		return "https://api.anthropic.com"
	case ProviderGoogle:
		return "https://generativelanguage.googleapis.com/v1beta/openai"
	default:
		return "https://openrouter.ai/api/v1"
	}
}

// Config describes one provider connection.
type Config struct {
	Provider Provider
	APIKey   string
	BaseURL  string
	Model    string
}

// OpenRouterDefaults returns known-good OpenRouter models for the UI.
func OpenRouterModels() []string {
	return []string{
		"openai/gpt-4.1-mini",
		"openai/gpt-4.1",
		"anthropic/claude-sonnet-4",
		"google/gemini-2.5-flash",
		"google/gemini-2.5-pro",
		"x-ai/grok-4",
		"deepseek/deepseek-chat-v3.1",
		"meta-llama/llama-4-maverick",
	}
}

// OpenCodeModels returns the OpenCode Go plan models that speak the
// OpenAI-compatible /chat/completions endpoint (docs: opencode.ai/docs/go).
// Models served only via /responses or /messages are intentionally excluded.
func OpenCodeModels() []string {
	return []string{
		"glm-5.3-flash",
		"glm-5.3",
		"glm-5.2",
		"kimi-k3",
		"kimi-k2.7-code",
		"kimi-k2.6",
		"longcat-2.0",
		"deepseek-v4.1-flash",
		"deepseek-v4-pro",
		"deepseek-v4-flash",
		"mimo-v2.6-flash",
		"mimo-v2.6-pro",
		"mimo-v2.5",
		"mimo-v2.5-pro",
		"hy4-preview",
		"hy3",
	}
}

// OpenAIModels returns known-good OpenAI chat models.
func OpenAIModels() []string {
	return []string{
		"gpt-4.1-mini",
		"gpt-4.1",
		"gpt-5-mini",
		"gpt-5",
	}
}

// MistralModels returns known-good Mistral chat models.
func MistralModels() []string {
	return []string{
		"mistral-small-latest",
		"mistral-medium-latest",
		"mistral-large-latest",
	}
}

// ClaudeModels returns known-good Anthropic Claude messages models.
func ClaudeModels() []string {
	return []string{
		"claude-haiku-4-5",
		"claude-sonnet-4-5",
		"claude-opus-4-1",
	}
}

// GoogleModels returns known-good Gemini models for the OpenAI-compatible
// endpoint (https://ai.google.dev/gemini-api/docs/openai).
func GoogleModels() []string {
	return []string{
		"gemini-2.5-flash",
		"gemini-2.5-pro",
		"gemini-3-flash-preview",
	}
}

// IsOpenCodeModel reports whether m is a valid OpenCode Go chat model.
func IsOpenCodeModel(m string) bool {
	for _, known := range OpenCodeModels() {
		if m == known {
			return true
		}
	}
	return false
}

// Complete sends a chat completion request to the configured provider.
// OpenAI, Mistral, Google (via its OpenAI-compatible base URL), OpenRouter
// and OpenCode Go all speak POST {base}/chat/completions with a Bearer key.
// Claude/Anthropic is native: POST https://api.anthropic.com/v1/messages
// with x-api-key + anthropic-version headers.
func Complete(ctx context.Context, cfg Config, req chatRequest) (*chatResponse, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("no API key configured for %s — add it in Settings", cfg.Provider)
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL(cfg.Provider)
	}
	if cfg.Provider == ProviderClaude {
		return completeAnthropic(ctx, cfg, req)
	}
	return completeOpenAICompatible(ctx, cfg, req)
}

func completeOpenAICompatible(ctx context.Context, cfg Config, req chatRequest) (*chatResponse, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "SpotiGent/1.1")
	if cfg.Provider == ProviderOpenCode {
		// OpenCode Go asks clients to identify the conversation for
		// routing/prompt-caching (docs: opencode.ai/docs/go).
		httpReq.Header.Set("X-Opencode-Session", newSessionID())
	} else {
		httpReq.Header.Set("HTTP-Referer", "http://localhost:8080")
		httpReq.Header.Set("X-Title", "SpotiGent")
	}

	client := &http.Client{Timeout: 120 * time.Second}
	res, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", cfg.Provider, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		if res.StatusCode >= 400 {
			return nil, fmt.Errorf("%s HTTP %d: %s", cfg.Provider, res.StatusCode, truncate(string(raw), 300))
		}
		return nil, fmt.Errorf("invalid %s response: %w", cfg.Provider, err)
	}
	if cr.Error != nil {
		return nil, fmt.Errorf("%s: %s", cfg.Provider, cr.Error.Message)
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("%s HTTP %d: %s", cfg.Provider, res.StatusCode, truncate(string(raw), 300))
	}
	if len(cr.Choices) == 0 {
		return nil, fmt.Errorf("%s returned no choices", cfg.Provider)
	}
	return &cr, nil
}

// ---- Anthropic Claude native API (POST /v1/messages) ----

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	System    string          `json:"system,omitempty"`
	Messages  []any           `json:"messages"`
	Tools     []anthropicTool `json:"tools,omitempty"`
}

func completeAnthropic(ctx context.Context, cfg Config, req chatRequest) (*chatResponse, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}

	var systemParts []string
	messages := make([]any, 0, len(req.Messages))
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			if strings.TrimSpace(m.Content) != "" {
				systemParts = append(systemParts, m.Content)
			}
		case "assistant":
			if len(m.ToolCalls) == 0 {
				messages = append(messages, map[string]any{"role": "assistant", "content": m.Content})
				continue
			}
			blocks := make([]any, 0, len(m.ToolCalls)+1)
			if strings.TrimSpace(m.Content) != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": m.Content})
			}
			for _, tc := range m.ToolCalls {
				var input map[string]any
				if strings.TrimSpace(tc.Function.Arguments) != "" {
					_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
				}
				if input == nil {
					input = map[string]any{}
				}
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": input,
				})
			}
			messages = append(messages, map[string]any{"role": "assistant", "content": blocks})
		case "tool":
			messages = append(messages, map[string]any{
				"role": "user",
				"content": []any{map[string]any{
					"type":        "tool_result",
					"tool_use_id": m.ToolCallID,
					"content":     m.Content,
				}},
			})
		default: // user
			messages = append(messages, map[string]any{"role": "user", "content": m.Content})
		}
	}

	tools := make([]anthropicTool, 0, len(req.Tools))
	for _, t := range req.Tools {
		schema := t.Function.Parameters
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		tools = append(tools, anthropicTool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			InputSchema: schema,
		})
	}

	areq := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		System:    strings.Join(systemParts, "\n\n"),
		Messages:  messages,
		Tools:     tools,
	}
	body, err := json.Marshal(areq)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-api-key", cfg.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "SpotiGent/1.1")

	client := &http.Client{Timeout: 120 * time.Second}
	res, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%s request failed: %w", cfg.Provider, err)
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var ar struct {
		Content []struct {
			Type  string         `json:"type"`
			Text  string         `json:"text,omitempty"`
			ID    string         `json:"id,omitempty"`
			Name  string         `json:"name,omitempty"`
			Input map[string]any `json:"input,omitempty"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Error      *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(raw, &ar); err != nil {
		if res.StatusCode >= 400 {
			return nil, fmt.Errorf("%s HTTP %d: %s", cfg.Provider, res.StatusCode, truncate(string(raw), 300))
		}
		return nil, fmt.Errorf("invalid %s response: %w", cfg.Provider, err)
	}
	if ar.Error != nil {
		return nil, fmt.Errorf("%s: %s", cfg.Provider, ar.Error.Message)
	}
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("%s HTTP %d: %s", cfg.Provider, res.StatusCode, truncate(string(raw), 300))
	}
	var text strings.Builder
	var calls []ToolCall
	for _, b := range ar.Content {
		switch b.Type {
		case "text":
			text.WriteString(b.Text)
		case "tool_use":
			args, _ := json.Marshal(b.Input)
			if string(args) == "" || string(args) == "null" {
				args = []byte("{}")
			}
			var tc ToolCall
			tc.ID = b.ID
			tc.Type = "function"
			tc.Function.Name = b.Name
			tc.Function.Arguments = string(args)
			calls = append(calls, tc)
		}
	}
	finish := "stop"
	if len(calls) > 0 {
		finish = "tool_calls"
	}
	return &chatResponse{
		Choices: []struct {
			Message      Message `json:"message"`
			FinishReason string  `json:"finish_reason"`
		}{
			{
				Message: Message{
					Role:      "assistant",
					Content:   strings.TrimSpace(text.String()),
					ToolCalls: calls,
				},
				FinishReason: finish,
			},
		},
	}, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// newSessionID returns a random hex session identifier for the
// X-Opencode-Session header (one conversation = one request here).
func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "spotigent"
	}
	return hex.EncodeToString(b[:])
}
