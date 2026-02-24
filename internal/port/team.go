package port

import (
	"context"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
)

// GraphStore persists and queries team structures and agent relationships.
type GraphStore interface {
	// Team CRUD
	SaveTeamCoordination(ctx context.Context, team *domain.TeamCoordination) error
	GetTeamCoordination(ctx context.Context, teamName string) (*domain.TeamCoordination, error)
	ListTeams(ctx context.Context) ([]string, error)

	// Relationship queries
	FindRelated(ctx context.Context, agentName string, relType domain.RelationType) ([]string, error)
	FindPath(ctx context.Context, fromAgent, toAgent string) ([]string, error) // BFS shortest path
	FindByRole(ctx context.Context, teamName string, role domain.AgentRole) ([]domain.TeamMember, error)
	FindByExpertise(ctx context.Context, teamName string, skills []string) ([]domain.TeamMember, error)

	// Relationship mutations
	AddRelationship(ctx context.Context, teamName string, rel domain.Relationship) error
	RemoveRelationship(ctx context.Context, teamName string, rel domain.Relationship) error

	// Network node tracking
	RegisterNode(ctx context.Context, node domain.NetworkNode) error
	HeartbeatNode(ctx context.Context, nodeID string) error
	FindNodesForAgent(ctx context.Context, agentName string) ([]domain.NetworkNode, error)
}

// TeamOrchestrator routes work across agents within a team.
// It uses graph queries to determine the best path to accomplish work.
type TeamOrchestrator interface {
	// Dispatch a task; router returns which agent(s) should handle it
	DispatchTask(ctx context.Context, teamName string, task domain.TaskRequest) ([]string, error)

	// Get the approval chain for a task
	GetApprovalChain(ctx context.Context, teamName string, task domain.TaskRequest) ([]string, error)

	// Find the best agent for a type of work based on role + expertise
	FindAgent(ctx context.Context, teamName string, role domain.AgentRole, skills []string) (string, error)

	// Execute work across a sequence of agents
	ExecuteWorkflow(ctx context.Context, teamName string, workflowName string, input any) (any, error)
}

// InterAgentMessenger handles agent-to-agent and team-to-team communication.
type InterAgentMessenger interface {
	Send(ctx context.Context, msg domain.AgentMessage) error
	Receive(ctx context.Context, toAgent string) (domain.AgentMessage, error)
	Broadcast(ctx context.Context, teamName string, msg domain.AgentMessage) error
}

// NetworkConnector enables federation and communication with remote agent nodes.
type NetworkConnector interface {
	// Discover and register nodes (via heartbeat, DNS-SD, etc.)
	RegisterLocalNode(ctx context.Context, node domain.NetworkNode) error
	DiscoverRemoteNodes(ctx context.Context) ([]domain.NetworkNode, error)

	// Invoke an agent on a remote node
	InvokeRemoteAgent(ctx context.Context, nodeID string, agentName string, input any) (any, error)

	// Forward a message to another node's agent
	ForwardMessage(ctx context.Context, nodeID string, msg domain.AgentMessage) error
}

// SessionTracer records execution traces for complex multi-agent workflows.
type SessionTracer interface {
	StartTrace(ctx context.Context, rootAgent string, teamName string) (string, error)
	LogEvent(ctx context.Context, traceID string, event domain.TraceEvent) error
	GetTrace(ctx context.Context, traceID string) (*domain.SessionTrace, error)
}
