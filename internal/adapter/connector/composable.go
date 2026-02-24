// Package connector provides a composable connector implementation that supports
// different authentication strategies and provider-specific configurations.
package connector

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bernardoforcillo/bernclaw/internal/adapter/auth"
	genaiAdapter "github.com/bernardoforcillo/bernclaw/internal/adapter/genai"
	"github.com/bernardoforcillo/bernclaw/internal/adapter/openaicompat"
	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// ComposableConnector wraps an authentication strategy with provider-specific behavior.
// This allows different LLM providers to be configured with different auth mechanisms
// without duplicating code.
type ComposableConnector struct {
	Provider   string            // Provider name (openai-compat, vertex-ai, etc.)
	AuthMethod string            // Authentication method (api-key, service-account, oauth)
	Strategy   auth.Strategy     // The actual authentication strategy
	Config     map[string]string // Provider-specific configuration
	Client     port.LLMClient    // The underlying LLM client (lazy-initialized)
}

// NewComposableConnector creates a new composable connector with the specified strategy.
func NewComposableConnector(provider, authMethod string, strategy auth.Strategy) *ComposableConnector {
	return &ComposableConnector{
		Provider:   provider,
		AuthMethod: authMethod,
		Strategy:   strategy,
		Config:     make(map[string]string),
	}
}

// WithConfig sets provider-specific configuration.
func (c *ComposableConnector) WithConfig(key, value string) *ComposableConnector {
	c.Config[key] = value
	return c
}

// Validate checks if the connector is properly configured.
func (c *ComposableConnector) Validate() error {
	if strings.TrimSpace(c.Provider) == "" {
		return errors.New("composable connector: provider is required")
	}
	if c.Strategy == nil {
		return errors.New("composable connector: authentication strategy is required")
	}
	if err := c.Strategy.Validate(); err != nil {
		return fmt.Errorf("composable connector: %w", err)
	}
	return nil
}

// GetLLMClient returns the underlying LLM client, initializing it if necessary.
func (c *ComposableConnector) GetLLMClient(ctx context.Context) (port.LLMClient, error) {
	if c.Client != nil {
		return c.Client, nil
	}

	if err := c.Validate(); err != nil {
		return nil, err
	}

	// Apply strategy to context
	strCtx, err := c.Strategy.Apply(ctx)
	if err != nil {
		return nil, fmt.Errorf("composable connector: failed to apply strategy: %w", err)
	}

	// Create provider-specific client based on provider and auth method
	var client port.LLMClient
	switch strings.ToLower(c.Provider) {
	case "openai-compatible", "openai":
		client, err = c.createOpenAIClient(strCtx)
	case "vertex-ai", "google-vertex", "gemini-vertex":
		client, err = c.createVertexClient(strCtx)
	case "ai-studio", "google-ai-studio":
		client, err = c.createAIStudioClient(strCtx)
	default:
		return nil, fmt.Errorf("composable connector: unsupported provider '%s'", c.Provider)
	}

	if err != nil {
		return nil, err
	}

	c.Client = client
	return c.Client, nil
}

// createOpenAIClient creates an OpenAI-compatible client.
func (c *ComposableConnector) createOpenAIClient(_ context.Context) (port.LLMClient, error) {
	apiKey, err := c.Strategy.GetCredential("api-key")
	if err != nil {
		return nil, fmt.Errorf("composable connector: failed to get api key: %w", err)
	}

	baseURL := c.Config["base-url"]
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return openaicompat.NewClient(openaicompat.Config{
		APIKey:  apiKey,
		BaseURL: baseURL,
	})
}

// createVertexClient creates a Google Vertex AI client using Service Account auth.
func (c *ComposableConnector) createVertexClient(ctx context.Context) (port.LLMClient, error) {
	projectID, err := c.Strategy.GetCredential("project-id")
	if err != nil {
		return nil, fmt.Errorf("composable connector: failed to get project id: %w", err)
	}

	region, err := c.Strategy.GetCredential("region")
	if err != nil {
		return nil, fmt.Errorf("composable connector: failed to get region: %w", err)
	}

	credentials, err := c.Strategy.GetCredential("credentials")
	if err != nil {
		return nil, fmt.Errorf("composable connector: failed to get credentials: %w", err)
	}

	// Create Vertex AI client with service account
	// This would use the genai package to initialize with service account credentials
	_ = projectID
	_ = region
	_ = credentials

	return nil, errors.New("composable connector: vertex ai client creation not yet implemented in this stub")
}

// createAIStudioClient creates a Google AI Studio client using API Key auth.
func (c *ComposableConnector) createAIStudioClient(ctx context.Context) (port.LLMClient, error) {
	apiKey, err := c.Strategy.GetCredential("api-key")
	if err != nil {
		return nil, fmt.Errorf("composable connector: failed to get api key: %w", err)
	}

	modelName := c.Config["model"]
	if modelName == "" {
		modelName = "gemini-2.0-flash-preview"
	}

	return genaiAdapter.NewAIStudioClient(ctx, apiKey, modelName)
}

// FromDomainConnector converts a domain.Connector to a ComposableConnector.
// This bridges the existing domain model with the new composable pattern.
func FromDomainConnector(dc domain.Connector) (*ComposableConnector, error) {
	var strategy auth.Strategy
	var authMethod string

	// Helper functions for parsing BaseURL
	extractProjectID := func(baseURL string) string {
		parts := strings.Split(baseURL, ":")
		if len(parts) > 0 {
			return parts[0]
		}
		return "default-project"
	}

	extractRegion := func(baseURL string) string {
		parts := strings.Split(baseURL, ":")
		if len(parts) > 1 {
			return parts[1]
		}
		return "us-central1"
	}

	// Determine auth strategy based on provider and available credentials
	switch dc.Provider {
	case domain.ConnectorProviderGeminiVertex:
		authMethod = "service-account"
		strategy = &auth.ServiceAccountStrategy{
			ProjectID:   extractProjectID(dc.BaseURL),
			Region:      extractRegion(dc.BaseURL),
			Credentials: dc.APIKey, // For now, assume raw credentials
		}

	case domain.ConnectorProviderOpenAICompat:
		authMethod = "api-key"
		strategy = &auth.APIKeyStrategy{
			APIKey: dc.APIKey,
		}

	default:
		return nil, fmt.Errorf("unsupported provider: %s", dc.Provider)
	}

	cc := NewComposableConnector(dc.Provider, authMethod, strategy)
	cc.WithConfig("base-url", dc.BaseURL)
	cc.WithConfig("name", dc.Name)

	return cc, nil
}

// extractProjectID extracts project ID from the BaseURL (format: "project:region")
func (c *ComposableConnector) extractProjectID(dc domain.Connector) string {
	parts := strings.Split(dc.BaseURL, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return "default-project"
}

// extractRegion extracts region from the BaseURL (format: "project:region")
func (c *ComposableConnector) extractRegion(dc domain.Connector) string {
	parts := strings.Split(dc.BaseURL, ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return "us-central1"
}

// FactoryFromConnector creates a ComposableConnector factory that works with domain.Connector.
func FactoryFromConnector() func(domain.Connector) (*ComposableConnector, error) {
	return func(dc domain.Connector) (*ComposableConnector, error) {
		return FromDomainConnector(dc)
	}
}

// Example configurations for different provider/auth combinations

// NewVertexAIWithServiceAccount creates a Vertex AI connector using Service Account auth.
func NewVertexAIWithServiceAccount(projectID, region, credentialsJSON string) *ComposableConnector {
	strategy := &auth.ServiceAccountStrategy{
		ProjectID:   projectID,
		Region:      region,
		Credentials: credentialsJSON,
	}
	return NewComposableConnector("vertex-ai", "service-account", strategy).
		WithConfig("project-id", projectID).
		WithConfig("region", region)
}

// NewVertexAIWithAPIKey creates a Vertex AI connector using API Key auth (for development).
func NewVertexAIWithAPIKey(projectID, region, apiKey string) *ComposableConnector {
	strategy := &auth.APIKeyStrategy{
		APIKey: apiKey,
	}
	return NewComposableConnector("vertex-ai", "api-key", strategy).
		WithConfig("project-id", projectID).
		WithConfig("region", region)
}

// NewAIStudioWithAPIKey creates a Google AI Studio connector using API Key auth.
func NewAIStudioWithAPIKey(apiKey string) *ComposableConnector {
	strategy := &auth.APIKeyStrategy{
		APIKey: apiKey,
	}
	return NewComposableConnector("ai-studio", "api-key", strategy)
}

// NewOpenAIWithAPIKey creates an OpenAI-compatible connector using API Key auth.
func NewOpenAIWithAPIKey(apiKey, baseURL string) *ComposableConnector {
	strategy := &auth.APIKeyStrategy{
		APIKey: apiKey,
	}
	return NewComposableConnector("openai-compatible", "api-key", strategy).
		WithConfig("base-url", baseURL)
}
