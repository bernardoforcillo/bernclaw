package domain

import "strings"

const (
	ConnectorProviderGeminiAI     = "gemini-ai"
	ConnectorProviderGeminiAIAlt  = "google-ai-studio"
	ConnectorProviderGeminiVertex = "gemini-vertex"
	ConnectorProviderOpenAICompat = "openai-compatible"
)

// Connector holds the credentials and configuration for an LLM provider.
type Connector struct {
	Name     string
	Provider string
	APIKey   string
	BaseURL  string
}

// IsSupportedProvider reports whether the given provider string is one of the
// recognised connector providers.
func IsSupportedProvider(provider string) bool {
	switch strings.TrimSpace(provider) {
	case ConnectorProviderGeminiAI, ConnectorProviderGeminiAIAlt, ConnectorProviderGeminiVertex, ConnectorProviderOpenAICompat:
		return true
	default:
		return false
	}
}
