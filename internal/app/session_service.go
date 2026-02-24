// Package app provides application services for session management and context assembly
package app

import (
	"context"
	"fmt"

	"github.com/bernardoforcillo/bernclaw/internal/adapter/session"
	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// SessionService implements port.SessionManager
type SessionService struct {
	store *session.Store
}

// NewSessionService creates a new session management service
func NewSessionService() *SessionService {
	return &SessionService{
		store: session.NewStore(),
	}
}

// ResolveOrCreate returns an existing or new session
func (s *SessionService) ResolveOrCreate(ctx context.Context, agentName, channel, identifier, userID string) (port.Session, error) {
	sess, err := s.store.ResolveOrCreate(ctx, agentName, channel, identifier, userID)
	if err != nil {
		return nil, err
	}
	return &sessionAdapter{session: sess}, nil
}

// GetSession retrieves a session by ID
func (s *SessionService) GetSession(sessionID string) (port.Session, error) {
	sess, err := s.store.Get(session.SessionID(sessionID))
	if err != nil {
		return nil, err
	}
	return &sessionAdapter{session: sess}, nil
}

// ListSessions returns all active sessions for an agent
func (s *SessionService) ListSessions(agentName string) []port.Session {
	sessions := s.store.List(agentName)

	// Convert to port.Session interface
	var result []port.Session
	for _, sess := range sessions {
		result = append(result, &sessionAdapter{session: sess})
	}

	return result
}

// SaveSession persists session state
func (s *SessionService) SaveSession(sess port.Session) error {
	// Extract the underlying session from the adapter
	if adapter, ok := sess.(*sessionAdapter); ok {
		return s.store.Save(adapter.session)
	}

	return fmt.Errorf("unknown session type")
}

// CloseSession ends a session
func (s *SessionService) CloseSession(sessionID string) error {
	return s.store.Delete(session.SessionID(sessionID))
}

// sessionAdapter wraps session.Session to implement port.Session interface
type sessionAdapter struct {
	session *session.Session
}

func (a *sessionAdapter) GetID() string {
	return string(a.session.ID)
}

func (a *sessionAdapter) GetType() string {
	return string(a.session.Type)
}

func (a *sessionAdapter) GetAgentName() string {
	return a.session.AgentName
}

func (a *sessionAdapter) GetChannel() string {
	return a.session.Channel
}

func (a *sessionAdapter) GetIdentifier() string {
	return a.session.Identifier
}

func (a *sessionAdapter) GetUserID() string {
	return a.session.UserID
}

func (a *sessionAdapter) AddMessage(msg domain.Message) {
	a.session.AddMessage(msg)
}

func (a *sessionAdapter) GetHistory(limit int) []domain.Message {
	return a.session.GetHistory(limit)
}

func (a *sessionAdapter) HasPermission(permission string) bool {
	return a.session.HasPermission(permission)
}

func (a *sessionAdapter) CanUseTool(toolName string) bool {
	return a.session.CanUseTool(toolName)
}

func (a *sessionAdapter) SetAllowedTools(tools []string) {
	a.session.AllowedTools = tools
}

func (a *sessionAdapter) SetContextValue(key string, value interface{}) {
	a.session.SetContextValue(key, value)
}

func (a *sessionAdapter) GetContextValue(key string) (interface{}, bool) {
	return a.session.GetContextValue(key)
}

func (a *sessionAdapter) IsActive() bool {
	return true // Sessions are always active until explicitly closed
}

func (a *sessionAdapter) SetActive(bool) {
	// No-op for now
}

// ExecutionFlow represents the complete message processing pipeline
type ExecutionFlow struct {
	sessionService *SessionService
	// Additional dependencies would be injected here
	// - contextAssembler
	// - memoryStore
	// - toolPolicy
	// - modelClient
}

// NewExecutionFlow creates a new execution flow processor
func NewExecutionFlow(sessionService *SessionService) *ExecutionFlow {
	return &ExecutionFlow{
		sessionService: sessionService,
	}
}

// ProcessIncomingMessage handles a 6-phase message execution
// Phase 1: MessageInput (already received)
// Phase 2: SessionResolution (resolve or create session)
// Phase 3: ContextAssembly (build execution context)
// Phase 4: SkillInjection (select relevant skills)
// Phase 5: ToolExecution (execute tools as needed)
// Phase 6: StreamCompletion (stream model response)
func (ef *ExecutionFlow) ProcessIncomingMessage(
	ctx context.Context,
	agentName, channel, identifier, userID, userMessage string,
) (port.Session, error) {
	// Phase 2: Resolve or create session
	sess, err := ef.sessionService.ResolveOrCreate(ctx, agentName, channel, identifier, userID)
	if err != nil {
		return nil, fmt.Errorf("session resolution failed: %w", err)
	}

	// Add user message to history
	sess.AddMessage(domain.Message{
		Role:    "user",
		Content: userMessage,
	})

	// Phase 3-6 would continue with context assembly, skill injection, etc.
	// For now, just return the resolved session

	return sess, nil
}
