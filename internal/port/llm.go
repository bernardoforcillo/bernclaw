package port

import (
	"context"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
)

// GenerateRequest is the model-agnostic request sent to an LLM.
type GenerateRequest struct {
	ModelName string
	Messages  []domain.Message
}

// LLMClient is implemented by every LLM provider adapter.
// Generate returns the assistant reply as a domain.Message.
type LLMClient interface {
	Generate(ctx context.Context, req GenerateRequest) (domain.Message, error)
}

// LLMClientFactory builds an LLMClient from a resolved connector.
// Adapters register a factory so the app layer can create clients on demand
// without knowing the concrete type.
type LLMClientFactory func(connector domain.Connector) (LLMClient, error)
