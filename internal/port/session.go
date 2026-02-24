// Package port defines interfaces for session and context management.
package port

import (
	"context"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
)

// SessionManager handles session lifecycle and state
type SessionManager interface {
	// ResolveOrCreate returns the session for a given context, creating if necessary
	ResolveOrCreate(ctx context.Context, agentName, channel, identifier, userID string) (Session, error)

	// GetSession retrieves an existing session by ID
	GetSession(sessionID string) (Session, error)

	// ListSessions returns all active sessions for an agent
	ListSessions(agentName string) []Session

	// SaveSession persists session state
	SaveSession(session Session) error

	// CloseSession ends a session
	CloseSession(sessionID string) error
}

// Session represents an active conversation context
type Session interface {
	// Identity
	GetID() string
	GetType() string // "main", "dm", "group"
	GetAgentName() string
	GetChannel() string
	GetIdentifier() string
	GetUserID() string

	// History
	AddMessage(msg domain.Message)
	GetHistory(limit int) []domain.Message

	// Permissions & Security
	HasPermission(permission string) bool
	CanUseTool(toolName string) bool
	SetAllowedTools(tools []string)

	// Context
	SetContextValue(key string, value any)
	GetContextValue(key string) (any, bool)

	// Lifecycle
	IsActive() bool
	SetActive(bool)
}

// ContextAssembler builds execution context from session, memory, and workspace
type ContextAssembler interface {
	// AssembleSystemPrompt builds the system prompt from workspace files and context
	AssembleSystemPrompt(ctx context.Context, session Session, workspace Workspace) (string, error)

	// AssembleExecutionContext prepares the full context for model invocation
	AssembleExecutionContext(ctx context.Context, session Session, workspace Workspace, memoryStore MemoryStore) (ExecutionContext, error)

	// InjectSkills selectively includes relevant skills in the context
	InjectSkills(ctx context.Context, skills []Skill, constraints *SecurityConstraints) ([]Skill, error)
}

// ExecutionContext contains all information needed for a model invocation
type ExecutionContext struct {
	// Core
	SessionID   string
	SessionType string
	AgentName   string
	UserID      string

	// Prompts
	SystemPrompt string
	Messages     []domain.Message

	// Tools & Skills
	AvailableTools []Tool
	InjectedSkills []string

	// Security
	Sandbox     bool
	Permissions map[string]bool

	// Memory context
	RelevantMemories []MemoryEntry
}

// Workspace defines access to agent workspace configuration
type Workspace interface {
	// File access
	ReadFile(relativePath string) (string, error)
	WriteFile(relativePath string, content string) error

	// Configuration
	GetAgentsByRole(role string) []domain.Spec
	GetTools() []Tool
	GetSkills() map[string]Skill
}

// MemoryStore provides semantic and keyword search over agent memory
type MemoryStore interface {
	// Semantic search with embeddings
	SearchSemantic(ctx context.Context, query string, limit int) ([]MemoryEntry, error)

	// Keyword search
	SearchKeywords(ctx context.Context, keywords []string, limit int) ([]MemoryEntry, error)

	// Hybrid search (semantic + keyword)
	Search(ctx context.Context, query string, keywords []string, limit int) ([]MemoryEntry, error)

	// Store a memory entry
	Store(ctx context.Context, entry MemoryEntry) error

	// Get memory entry by ID
	Get(ctx context.Context, id string) (MemoryEntry, error)
}

// MemoryEntry represents a single memory item with embedding
type MemoryEntry struct {
	ID        string
	Content   string
	Embedding []float32 // Vector embedding
	Keywords  []string
	Source    string // "user_message", "tool_result", "memory.md", etc.
	Timestamp int64
	SessionID string
}

// Skill represents a playbook or capability
type Skill struct {
	Name         string
	Description  string
	Tools        []string // which tools this skill uses
	SystemPrompt string   // skill-specific prompt injection
	Cost         int      // token cost estimate
	TrustLevel   string   // "public", "user", "main"
}

// Tool represents a tool available to the agent
type Tool interface {
	GetName() string
	GetDescription() string
	GetParameters() map[string]any
	Execute(ctx context.Context, params map[string]any) (any, error)
}

// SecurityConstraints define sandboxing and tool policies for a session
type SecurityConstraints struct {
	SessionType    string
	Sandboxed      bool
	AllowedTools   []string
	MaxTokens      int
	TimeoutSeconds int
	NetworkAccess  bool
	FileAccess     bool
}

// ToolPolicy evaluates whether a tool can be executed in a session
type ToolPolicy interface {
	CanExecute(ctx context.Context, session Session, toolName string) (bool, error)
	GetExecutionEnvironment(toolName string) ExecutionEnvironment
}

// ExecutionEnvironment describes where a tool runs
type ExecutionEnvironment struct {
	Type           string // "host", "docker", "wasm", "native"
	Sandboxed      bool
	NetworkAccess  bool
	FileAccess     bool
	MaxMemoryMB    int
	TimeoutSeconds int
}
