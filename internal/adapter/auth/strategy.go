// Package auth provides composable authentication strategies for LLM connectors.
// This allows different providers to use different auth mechanisms (API Key, Service Account, OAuth, etc.)
// without tight coupling to specific implementations.
package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Strategy defines the interface for authentication implementations.
// Each LLM provider can implement their own strategy based on their requirements.
type Strategy interface {
	// Name returns the strategy type identifier (e.g., "api-key", "service-account", "oauth")
	Name() string

	// Validate checks if the strategy has required credentials set.
	Validate() error

	// GetCredential retrieves a credential value by key.
	// Used to construct authentication headers or clients.
	GetCredential(key string) (string, error)

	// Apply applies the strategy to prepare a context or configuration.
	// Implementation-specific behavior for setting up auth.
	Apply(ctx context.Context) (context.Context, error)
}

// APIKeyStrategy authenticates using an API key.
// Suitable for providers like OpenAI, Anthropic, etc.
type APIKeyStrategy struct {
	APIKey string
	Header string // Header name for the API key (default: "Authorization")
}

// Name implements Strategy.
func (s *APIKeyStrategy) Name() string {
	return "api-key"
}

// Validate checks if APIKey is set.
func (s *APIKeyStrategy) Validate() error {
	if strings.TrimSpace(s.APIKey) == "" {
		return errors.New("api-key strategy: api key is required")
	}
	return nil
}

// GetCredential retrieves the API key or header name.
func (s *APIKeyStrategy) GetCredential(key string) (string, error) {
	switch strings.ToLower(key) {
	case "api-key", "key":
		return s.APIKey, nil
	case "header":
		if s.Header == "" {
			return "Authorization", nil
		}
		return s.Header, nil
	default:
		return "", fmt.Errorf("api-key strategy: unknown credential key '%s'", key)
	}
}

// Apply adds the API key to the context as a value.
func (s *APIKeyStrategy) Apply(ctx context.Context) (context.Context, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return context.WithValue(ctx, "x-api-key", s.APIKey), nil
}

// ServiceAccountStrategy authenticates using a service account (Google Cloud, etc.).
// Suitable for providers like Google Vertex AI.
type ServiceAccountStrategy struct {
	ProjectID   string // GCP Project ID
	Region      string // GCP Region
	Credentials string // Path to service account JSON or JSON string
}

// Name implements Strategy.
func (s *ServiceAccountStrategy) Name() string {
	return "service-account"
}

// Validate checks if required fields are set.
func (s *ServiceAccountStrategy) Validate() error {
	if strings.TrimSpace(s.ProjectID) == "" {
		return errors.New("service-account strategy: project id is required")
	}
	if strings.TrimSpace(s.Credentials) == "" {
		return errors.New("service-account strategy: credentials are required")
	}
	return nil
}

// GetCredential retrieves service account credentials or metadata.
func (s *ServiceAccountStrategy) GetCredential(key string) (string, error) {
	switch strings.ToLower(key) {
	case "project-id", "projectid":
		return s.ProjectID, nil
	case "region":
		if s.Region == "" {
			return "us-central1", nil // default
		}
		return s.Region, nil
	case "credentials":
		return s.Credentials, nil
	default:
		return "", fmt.Errorf("service-account strategy: unknown credential key '%s'", key)
	}
}

// Apply prepares the context with service account credentials.
// In a real implementation, this would initialize a Google Cloud client.
func (s *ServiceAccountStrategy) Apply(ctx context.Context) (context.Context, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, "gcp-project-id", s.ProjectID)
	ctx = context.WithValue(ctx, "gcp-region", s.Region)
	ctx = context.WithValue(ctx, "gcp-credentials", s.Credentials)
	return ctx, nil
}

// OAuthStrategy authenticates using OAuth2 tokens.
// Suitable for providers supporting OAuth2 flows.
type OAuthStrategy struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	AccessToken  string // Current access token
}

// Name implements Strategy.
func (s *OAuthStrategy) Name() string {
	return "oauth"
}

// Validate checks if required OAuth fields are set.
func (s *OAuthStrategy) Validate() error {
	if strings.TrimSpace(s.ClientID) == "" {
		return errors.New("oauth strategy: client id is required")
	}
	if strings.TrimSpace(s.ClientSecret) == "" {
		return errors.New("oauth strategy: client secret is required")
	}
	if strings.TrimSpace(s.TokenURL) == "" {
		return errors.New("oauth strategy: token url is required")
	}
	return nil
}

// GetCredential retrieves OAuth credentials or tokens.
func (s *OAuthStrategy) GetCredential(key string) (string, error) {
	switch strings.ToLower(key) {
	case "client-id", "clientid":
		return s.ClientID, nil
	case "client-secret", "clientsecret":
		return s.ClientSecret, nil
	case "token-url", "tokenurl":
		return s.TokenURL, nil
	case "access-token", "accesstoken":
		if s.AccessToken == "" {
			return "", errors.New("oauth strategy: no current access token")
		}
		return s.AccessToken, nil
	default:
		return "", fmt.Errorf("oauth strategy: unknown credential key '%s'", key)
	}
}

// Apply prepares the context with OAuth credentials.
// In a real implementation, this would perform token exchange if needed.
func (s *OAuthStrategy) Apply(ctx context.Context) (context.Context, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, "oauth-client-id", s.ClientID)
	ctx = context.WithValue(ctx, "oauth-access-token", s.AccessToken)
	return ctx, nil
}

// StrategyBuilder helps construct authentication strategies from configuration.
type StrategyBuilder struct {
	strategyType string
	config       map[string]string
}

// NewStrategyBuilder creates a new builder for the given strategy type.
func NewStrategyBuilder(strategyType string) *StrategyBuilder {
	return &StrategyBuilder{
		strategyType: strings.ToLower(strategyType),
		config:       make(map[string]string),
	}
}

// WithConfig sets configuration key-value pairs.
func (b *StrategyBuilder) WithConfig(key, value string) *StrategyBuilder {
	b.config[key] = value
	return b
}

// Build constructs the appropriate Strategy based on configuration.
func (b *StrategyBuilder) Build() (Strategy, error) {
	switch b.strategyType {
	case "api-key":
		return &APIKeyStrategy{
			APIKey: b.config["api-key"],
			Header: b.config["header"],
		}, nil

	case "service-account", "gcp":
		return &ServiceAccountStrategy{
			ProjectID:   b.config["project-id"],
			Region:      b.config["region"],
			Credentials: b.config["credentials"],
		}, nil

	case "oauth":
		return &OAuthStrategy{
			ClientID:     b.config["client-id"],
			ClientSecret: b.config["client-secret"],
			TokenURL:     b.config["token-url"],
			AccessToken:  b.config["access-token"],
		}, nil

	default:
		return nil, fmt.Errorf("unknown strategy type: %s", b.strategyType)
	}
}
