// Package genai provides a port.LLMClient implementation for Google Vertex AI
// using the google.golang.org/genai package for generative models.
// This adapter currently provides a template; full genai integration requires
// the actual genai API. For now, demonstrates the integration point.
package genai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// Config holds the credentials and settings for Google Vertex AI.
type Config struct {
	ProjectID string // Google Cloud project ID
	Region    string // GCP region (e.g., "us-central1")
	APIKey    string // Google API Key (optional, uses Application Default Credentials if empty)
}

// Client is a port.LLMClient backed by Google Vertex AI generative models.
type Client struct {
	projectID string
	region    string
	apiKey    string
	modelName string
}

// NewClient validates the config and returns a new Client.
func NewClient(cfg Config) (*Client, error) {
	projectID := strings.TrimSpace(cfg.ProjectID)
	if projectID == "" {
		return nil, errors.New("genai: project id is required")
	}

	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-central1" // default region
	}

	return &Client{
		projectID: projectID,
		region:    region,
		apiKey:    strings.TrimSpace(cfg.APIKey),
		modelName: "gemini-1.5-pro", // default model
	}, nil
}

// Generate implements port.LLMClient. It sends the request to Google Vertex AI
// and returns the assembled reply.
// Note: This is a template implementation. Actual implementation requires
// google.golang.org/genai client initialization based on the final genai API.
func (c *Client) Generate(ctx context.Context, req port.GenerateRequest) (domain.Message, error) {
	if c == nil {
		return domain.Message{}, errors.New("genai: client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if len(req.Messages) == 0 {
		return domain.Message{}, errors.New("genai: at least one message is required")
	}

	modelName := strings.TrimSpace(req.ModelName)
	if modelName == "" {
		modelName = c.modelName // use default if not specified
	}

	// Extract the last user message or context
	var userInput string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if strings.EqualFold(req.Messages[i].Role, "user") {
			userInput = req.Messages[i].Content
			break
		}
	}

	if userInput == "" {
		return domain.Message{}, errors.New("genai: no user message found in request")
	}

	// TODO: Initialize google.golang.org/genai client and call API
	// Placeholder response while genai integration is completed
	generatedText := fmt.Sprintf("[genai/%s@%s:%s] Placeholder response for: %s",
		modelName, c.projectID, c.region, userInput)

	return domain.Message{Role: "assistant", Content: generatedText}, nil
}

// NewClientFromConnector creates a genai.Client from a domain.Connector.
// The connector should have Provider = "gemini-vertex" and APIKey set.
func NewClientFromConnector(connector domain.Connector) (port.LLMClient, error) {
	if connector.Provider != domain.ConnectorProviderGeminiVertex {
		return nil, fmt.Errorf("genai: unsupported provider '%s', expected '%s'",
			connector.Provider, domain.ConnectorProviderGeminiVertex)
	}

	// Extract project and region from connector if available (BaseURL format: project:region)
	projectID := "default-project"
	region := "us-central1"
	if connector.BaseURL != "" {
		parts := strings.Split(connector.BaseURL, ":")
		if len(parts) >= 1 {
			projectID = parts[0]
		}
		if len(parts) >= 2 {
			region = parts[1]
		}
	}

	cfg := Config{
		ProjectID: projectID,
		Region:    region,
		APIKey:    connector.APIKey,
	}

	return NewClient(cfg)
}
