package domain

// Message types for inter-agent and inter-node communication.

// AgentMessage is the envelope for agent-to-agent communication.
type AgentMessage struct {
	ID            string
	FromAgent     string
	ToAgent       string
	FromTeam      string
	ToTeam        string
	MessageType   string // "request", "reply", "event", "heartbeat"
	Payload       any
	CorrelationID string // links to original request
	Timestamp     int64
}

// TaskRequest is sent by one agent to another requesting work.
type TaskRequest struct {
	ID          string
	Title       string
	Description string
	Context     map[string]any
	Requester   string
	Deadline    int64
	Priority    int // 1 (low) to 10 (critical)
}

// TaskResult is the response after a task completes.
type TaskResult struct {
	TaskID   string
	Status   string // "pending", "running", "completed", "failed"
	Output   any
	Error    string
	Evidence map[string]any
}

// SessionTrace represents a cross-agent execution trace.
// Used to understand how a complex task flowed through multiple agents.
type SessionTrace struct {
	ID            string
	RootAgentName string
	TeamName      string
	TraceEvents   []TraceEvent
	StartedAt     int64
	CompletedAt   int64
}

// TraceEvent is a single action or state change during execution.
type TraceEvent struct {
	Timestamp   int64
	Agent       string
	EventType   string // "started", "delegated", "tool_called", "completed"
	Description string
	Data        map[string]any
	ChildTraces []string // IDs of related traces
}

// GroupSessionContext extends a conversation to involve multiple agents.
type GroupSessionContext struct {
	SessionID    string
	TeamName     string
	Participants []string // Agent names
	History      []AgentMessage
	SharedState  map[string]any // Mutable state all agents see
	CreatedAt    int64
}
