// Package session provides sophisticated session management following OpenClaw patterns.
// Sessions are security boundaries with different trust levels and capabilities.
package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
)

// SessionType categorizes different session trust levels and capabilities
type SessionType string

const (
	SessionTypeMain  SessionType = "main"  // Operator's full-access session
	SessionTypeDM    SessionType = "dm"    // Direct message (sandboxed)
	SessionTypeGroup SessionType = "group" // Group chat (sandboxed)
)

// SessionID uniquely identifies a session with trust-level encoding
// Format: agent:<agentId>:<sessionType>:<channel>:<identifier>
// Example: agent:main:main
//
//	agent:main:dm:whatsapp:+1234567890
//	agent:main:group:discord:123456789@g.us
type SessionID string

// Session represents an active conversation context with security and state management
type Session struct {
	ID         SessionID
	Type       SessionType
	AgentName  string
	Channel    string // whatsapp, telegram, discord, etc.
	Identifier string // phone, username, group ID, etc.
	UserID     string

	// Execution context
	History     []domain.Message // Conversation history
	ContextData map[string]any   // Arbitrary context (user preferences, etc.)

	// Timestamps
	CreatedAt     time.Time
	LastMessageAt time.Time

	// Security
	Sandboxed    bool            // Run tools in isolated container?
	AllowedTools []string        // Tool allowlist (empty = all)
	Permissions  map[string]bool // Fine-grained permissions

	// State
	mu sync.RWMutex
}

// NewSession creates a new session with appropriate defaults based on type
func NewSession(agentName, channel, identifier, userID string, sessionType SessionType) *Session {
	sessionID := SessionID(fmt.Sprintf("agent:%s:%s:%s:%s", agentName, sessionType, channel, identifier))

	// Default security settings by session type
	sandboxed := sessionType != SessionTypeMain
	now := time.Now()

	return &Session{
		ID:            sessionID,
		Type:          sessionType,
		AgentName:     agentName,
		Channel:       channel,
		Identifier:    identifier,
		UserID:        userID,
		History:       make([]domain.Message, 0),
		ContextData:   make(map[string]any),
		CreatedAt:     now,
		LastMessageAt: now,
		Sandboxed:     sandboxed,
		AllowedTools:  make([]string, 0), // Empty means all tools allowed (further restricted by policy)
		Permissions:   defaultPermissions(sessionType),
	}
}

// defaultPermissions returns baseline permissions for a session type
func defaultPermissions(sessionType SessionType) map[string]bool {
	perms := map[string]bool{
		"read_memory":      true,
		"invoke_tools":     true,
		"read_files":       true,
		"write_files":      false,
		"access_network":   false,
		"schedule_actions": false,
	}

	if sessionType == SessionTypeMain {
		// Main session: full operator capabilities
		perms["write_files"] = true
		perms["access_network"] = true
		perms["schedule_actions"] = true
	}

	return perms
}

// AddMessage appends a message to the session history
func (s *Session) AddMessage(msg domain.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.History = append(s.History, msg)
	s.LastMessageAt = time.Now()
}

// GetHistory returns the recent message history (optionally limited)
func (s *Session) GetHistory(limit int) []domain.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit >= len(s.History) {
		return append([]domain.Message{}, s.History...)
	}

	start := len(s.History) - limit
	return append([]domain.Message{}, s.History[start:]...)
}

// HasPermission checks if this session has a specific permission
func (s *Session) HasPermission(permission string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	allowed, exists := s.Permissions[permission]
	return exists && allowed
}

// CanUseTool checks if this session can use a specific tool
func (s *Session) CanUseTool(toolName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.Permissions["invoke_tools"] {
		return false
	}

	// If allowlist is empty, all tools are allowed (further policy checks apply)
	if len(s.AllowedTools) == 0 {
		return true
	}

	// Check allowlist
	for _, allowed := range s.AllowedTools {
		if allowed == toolName || allowed == "*" {
			return true
		}
	}

	return false
}

// SetContextValue sets an arbitrary context value for this session
func (s *Session) SetContextValue(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ContextData[key] = value
}

// GetContextValue retrieves an arbitrary context value
func (s *Session) GetContextValue(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, exists := s.ContextData[key]
	return val, exists
}

// Store manages sessions in memory
type Store struct {
	sessions map[SessionID]*Session
	mu       sync.RWMutex
}

// NewStore creates an empty session store
func NewStore() *Store {
	return &Store{
		sessions: make(map[SessionID]*Session),
	}
}

// Create or update a session
func (s *Store) Save(session *Session) error {
	if session == nil {
		return fmt.Errorf("cannot save nil session")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session

	return nil
}

// Get retrieves a session by ID
func (s *Store) Get(sessionID SessionID) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// List returns all sessions for an agent
func (s *Store) List(agentName string) []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Session
	prefix := fmt.Sprintf("agent:%s:", agentName)

	for id, session := range s.sessions {
		if strings.HasPrefix(string(id), prefix) {
			result = append(result, session)
		}
	}

	return result
}

// Delete removes a session
func (s *Store) Delete(sessionID SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sessions[sessionID]; !exists {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	delete(s.sessions, sessionID)
	return nil
}

// ResolveOrCreate returns an existing session or creates a new one
// This follows OpenClaw's session resolution pattern
func (s *Store) ResolveOrCreate(ctx context.Context, agentName, channel, identifier, userID string) (*Session, error) {
	// Determine session type based on context
	sessionType := SessionTypeDM

	// If identifier matches a known user (for this example, just "main"), it's the main session
	if identifier == "main" || identifier == userID {
		sessionType = SessionTypeMain
	} else if strings.HasSuffix(identifier, "@g.us") || strings.Contains(identifier, "group") {
		sessionType = SessionTypeGroup
	}

	sessionID := SessionID(fmt.Sprintf("agent:%s:%s:%s:%s", agentName, sessionType, channel, identifier))

	// Try to get existing session
	if session, err := s.Get(sessionID); err == nil {
		return session, nil
	}

	// Create new session
	session := NewSession(agentName, channel, identifier, userID, sessionType)
	if err := s.Save(session); err != nil {
		return nil, err
	}

	return session, nil
}

// Compact summarizes old messages to save tokens
// This implements OpenClaw's session compaction pattern
func (s *Session) Compact(summaryThreshold int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.History) < summaryThreshold {
		return nil // Nothing to compact
	}

	// Keep recent messages, summarize older ones
	cutoff := len(s.History) - summaryThreshold
	oldMessages := s.History[:cutoff]
	recentMessages := s.History[cutoff:]

	// Create a summary message
	if len(oldMessages) > 0 {
		summaryContent := fmt.Sprintf("[Session summary: %d previous messages compacted at %s]",
			len(oldMessages), time.Now().Format(time.RFC3339))

		summaryMsg := domain.Message{
			Role:    "system",
			Content: summaryContent,
		}

		// Replace old messages with summary
		s.History = append([]domain.Message{summaryMsg}, recentMessages...)
	}

	return nil
}

// IsActive returns whether the session is still active
func (s *Session) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return true // Sessions are active until explicitly closed
}

// SetActive sets the active state (no-op for now)
func (s *Session) SetActive(active bool) {
	// No-op
}

// SetAllowedTools updates the allowed tools for this session
func (s *Session) SetAllowedTools(tools []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AllowedTools = tools
}
