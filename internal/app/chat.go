package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// ChatService orchestrates sending a conversation turn to the appropriate LLM.
// It resolves the connector by name, builds the provider-specific client via
// the injected factory, and delegates generation to the resulting LLMClient.
type ChatService struct {
	connectors port.ConnectorRepository
	factory    port.LLMClientFactory
}

// NewChatService creates a ChatService with the given connector repository and
// client factory.  The factory is called once per Send to create a fresh
// LLMClient for the resolved connector.
func NewChatService(connectors port.ConnectorRepository, factory port.LLMClientFactory) *ChatService {
	return &ChatService{connectors: connectors, factory: factory}
}

// Send resolves the named connector, creates the LLM client, and generates a
// reply for the given message history.  It returns the assistant's reply as a
// domain.Message.
func (s *ChatService) Send(ctx context.Context, connectorName string, modelName string, messages []domain.Message) (domain.Message, error) {
	connectorName = strings.TrimSpace(connectorName)
	if connectorName == "" {
		return domain.Message{}, fmt.Errorf("connector name is required")
	}
	if strings.TrimSpace(modelName) == "" {
		return domain.Message{}, fmt.Errorf("model name is required")
	}
	if len(messages) == 0 {
		return domain.Message{}, fmt.Errorf("at least one message is required")
	}

	connector, err := s.connectors.GetConnector(connectorName)
	if err != nil {
		return domain.Message{}, fmt.Errorf("resolve connector %q: %w", connectorName, err)
	}

	if strings.TrimSpace(connector.APIKey) == "" {
		return domain.Message{}, fmt.Errorf("connector %q has empty apiKey", connectorName)
	}

	client, err := s.factory(connector)
	if err != nil {
		return domain.Message{}, fmt.Errorf("create llm client for connector %q: %w", connectorName, err)
	}

	reply, err := client.Generate(ctx, port.GenerateRequest{
		ModelName: modelName,
		Messages:  messages,
	})
	if err != nil {
		return domain.Message{}, err
	}

	return reply, nil
}
