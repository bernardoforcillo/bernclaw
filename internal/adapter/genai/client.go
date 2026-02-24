// Package genai provides port.LLMClient implementations for Google AI Studio
// and Google Vertex AI using the google.golang.org/genai SDK.
package genai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gai "google.golang.org/genai"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

const defaultModel = "gemini-3-flash-preview"

// ── AI Studio ────────────────────────────────────────────────────────────────

// AIStudioClient is a port.LLMClient backed by Google AI Studio
// (https://generativelanguage.googleapis.com) authenticated with an API key.
type AIStudioClient struct {
	client    *gai.Client
	modelName string
}

// NewAIStudioClient creates an AIStudioClient authenticated with apiKey.
// modelName defaults to "gemini-3.0-flash-preview" if empty.
func NewAIStudioClient(ctx context.Context, apiKey, modelName string) (*AIStudioClient, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("genai/ai-studio: api key is required")
	}
	if strings.TrimSpace(modelName) == "" {
		modelName = defaultModel
	}

	c, err := gai.NewClient(ctx, &gai.ClientConfig{
		APIKey:  apiKey,
		Backend: gai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("genai/ai-studio: create client: %w", err)
	}

	return &AIStudioClient{client: c, modelName: strings.TrimSpace(modelName)}, nil
}

// Generate implements port.LLMClient. It sends the full message history to the
// Gemini model and returns the assistant reply.
func (c *AIStudioClient) Generate(ctx context.Context, req port.GenerateRequest) (domain.Message, error) {
	if c == nil {
		return domain.Message{}, errors.New("genai/ai-studio: client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if len(req.Messages) == 0 {
		return domain.Message{}, errors.New("genai/ai-studio: at least one message is required")
	}

	model := strings.TrimSpace(req.ModelName)
	if model == "" {
		model = c.modelName
	}

	// Convert domain.Message slice → []*gai.Content
	contents := make([]*gai.Content, 0, len(req.Messages))
	for _, msg := range req.Messages {
		role := "user"
		if strings.EqualFold(msg.Role, "assistant") || strings.EqualFold(msg.Role, "model") {
			role = "model"
		}
		contents = append(contents, &gai.Content{
			Role:  role,
			Parts: []*gai.Part{{Text: msg.Content}},
		})
	}

	resp, err := c.client.Models.GenerateContent(ctx, model, contents, nil)
	if err != nil {
		return domain.Message{}, fmt.Errorf("genai/ai-studio: generate: %w", err)
	}
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return domain.Message{}, errors.New("genai/ai-studio: empty response from API")
	}

	var sb strings.Builder
	for _, p := range resp.Candidates[0].Content.Parts {
		if p.Text != "" {
			sb.WriteString(p.Text)
		}
	}

	return domain.Message{Role: "assistant", Content: strings.TrimSpace(sb.String())}, nil
}

// NewAIStudioClientFromConnector creates an AIStudioClient from a domain.Connector
// whose Provider is ConnectorProviderGeminiAI.
func NewAIStudioClientFromConnector(ctx context.Context, connector domain.Connector, modelName string) (port.LLMClient, error) {
	if connector.Provider != domain.ConnectorProviderGeminiAI {
		return nil, fmt.Errorf("genai/ai-studio: unsupported provider %q, expected %q",
			connector.Provider, domain.ConnectorProviderGeminiAI)
	}
	return NewAIStudioClient(ctx, connector.APIKey, modelName)
}

// ── Vertex AI (legacy stub, kept for future use) ─────────────────────────────

// VertexConfig holds settings for Google Vertex AI.
type VertexConfig struct {
	ProjectID string
	Region    string
	APIKey    string
}

// VertexClient is a port.LLMClient placeholder for Vertex AI.
// Use AIStudioClient for Google AI Studio; this stub exists for future Vertex integration.
type VertexClient struct {
	projectID string
	region    string
	modelName string
}

// NewVertexClient returns a VertexClient (placeholder – not yet wired to the real API).
func NewVertexClient(cfg VertexConfig) (*VertexClient, error) {
	projectID := strings.TrimSpace(cfg.ProjectID)
	if projectID == "" {
		return nil, errors.New("genai/vertex: project id is required")
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-central1"
	}
	return &VertexClient{projectID: projectID, region: region, modelName: defaultModel}, nil
}

// Generate implements port.LLMClient (placeholder).
func (c *VertexClient) Generate(_ context.Context, req port.GenerateRequest) (domain.Message, error) {
	return domain.Message{}, fmt.Errorf("genai/vertex: not yet implemented (project=%s region=%s model=%s)",
		c.projectID, c.region, req.ModelName)
}
