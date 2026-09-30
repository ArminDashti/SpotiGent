package config

import (
	"os"
	"strconv"
)

// Config holds server-level settings. User-level settings (API keys,
// provider, theme, model) live in data/settings.json and are editable
// from the web UI, so nothing sensitive is baked into source code.
type Config struct {
	Host string
	Port string

	// Optional environment overrides for the AI provider API keys.
	// Keys entered in the web UI take precedence over these.
	OpenRouterAPIKey string
	OpenCodeAPIKey   string
	OpenAIAPIKey     string
	MistralAPIKey    string
	ClaudeAPIKey     string
	GoogleAPIKey     string

	// Base URLs (overridable for self-hosted or proxied deployments).
	OpenRouterBaseURL string
	OpenCodeBaseURL   string // OpenCode Go plan endpoint
	OpenAIBaseURL     string
	MistralBaseURL    string
	ClaudeBaseURL     string // Anthropic REST root, e.g. https://api.anthropic.com
	GoogleBaseURL     string // Gemini OpenAI-compatible base
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Load reads configuration from the environment with sensible defaults.
func Load() Config {
	return Config{
		Host:              getEnv("SPOTIGENT_HOST", "127.0.0.1"),
		Port:              getEnv("SPOTIGENT_PORT", "8080"),
		OpenRouterAPIKey:  os.Getenv("OPENROUTER_API_KEY"),
		OpenCodeAPIKey:    os.Getenv("OPENCODE_API_KEY"),
		OpenAIAPIKey:      firstEnv("OPENAI_API_KEY"),
		MistralAPIKey:     firstEnv("MISTRAL_API_KEY"),
		ClaudeAPIKey:      firstEnv("ANTHROPIC_API_KEY", "CLAUDE_API_KEY"),
		GoogleAPIKey:      firstEnv("GOOGLE_API_KEY", "GEMINI_API_KEY", "GOOGLE_GEMINI_API_KEY"),
		OpenRouterBaseURL: getEnv("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		OpenCodeBaseURL:   getEnv("OPENCODE_BASE_URL", "https://opencode.ai/zen/go/v1"),
		OpenAIBaseURL:     getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		MistralBaseURL:    getEnv("MISTRAL_BASE_URL", "https://api.mistral.ai/v1"),
		ClaudeBaseURL:     getEnv("ANTHROPIC_BASE_URL", "https://api.anthropic.com"),
		GoogleBaseURL:     getEnv("GOOGLE_BASE_URL", "https://generativelanguage.googleapis.com/v1beta/openai"),
	}
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// Addr returns the host:port listen address.
func (c Config) Addr() string {
	// Validate the port is numeric, fall back otherwise.
	if _, err := strconv.Atoi(c.Port); err != nil {
		return c.Host + ":8080"
	}
	return c.Host + ":" + c.Port
}
