package domain

import "strings"

// AgentRole defines the function an agent plays within a team.
type AgentRole string

const (
	RolePlanner     AgentRole = "planner"     // Decomposes goals
	RoleExecutor    AgentRole = "executor"    // Runs tools/tasks
	RoleReviewer    AgentRole = "reviewer"    // Validates work
	RoleCoordinator AgentRole = "coordinator" // Routes work
	RoleSpecialist  AgentRole = "specialist"  // Domain expert
)

// TeamMember represents an agent bound to a team with a specific role.
type TeamMember struct {
	AgentName   string
	Role        AgentRole
	CanDelegate bool     // Can this member ask others to do work?
	CanApprove  bool     // Can this member approve work?
	Expertise   []string // Domain tags (e.g., ["python", "devops"])
}

// RelationType categorizes how two agents relate.
type RelationType string

const (
	RelationDependsOn     RelationType = "depends_on"    // A needs output from B
	RelationSupervises    RelationType = "supervises"    // A monitors/approves work by B
	RelationDelegatesTo   RelationType = "delegates_to"  // A asks B to do work
	RelationComplementary RelationType = "complementary" // A and B work in concert
	RelationConflicts     RelationType = "conflicts"     // A and B have opposite constraints
)

// Relationship captures directed edges in the agent network.
type Relationship struct {
	FromAgent string
	ToAgent   string
	Type      RelationType
	Weight    float64 // 0.0-1.0, strength of relationship
	Metadata  map[string]any
}

// TeamCoordination holds the state of a team's internal relationships and workflow routing.
// It acts as a snapshot of "who is assigned to what, and in what dependency order."
type TeamCoordination struct {
	Name          string
	Members       map[string]TeamMember // key: agent name
	Relationships []Relationship
	WorkflowOrder []string // Which agent(s) go first, second, etc.
	Created       int64
	Updated       int64
}

// NetworkNode represents a machine/instance running bernclaw agents.
type NetworkNode struct {
	ID            string   // hostname or UUID
	Endpoint      string   // ws://host:port or similar
	Role          string   // "hub", "worker", "leaf"
	Stable        bool     // Is this node reliable?
	Agents        []string // Agent names available on this node
	LastHeartbeat int64
}

// TeamGraph manages the full directed graph of agent relationships
// and enables queries like "who can execute task X" or "get approval chain".
type TeamGraph struct {
	Teams  map[string]*TeamCoordination
	Nodes  map[string]*NetworkNode
	Global []Relationship // Cross-team relationships
}

// NewTeamCoordination creates an empty team structure.
func NewTeamCoordination(name string) *TeamCoordination {
	return &TeamCoordination{
		Name:          strings.TrimSpace(name),
		Members:       make(map[string]TeamMember),
		Relationships: make([]Relationship, 0),
		WorkflowOrder: make([]string, 0),
	}
}

// NewTeamGraph initializes a graph store.
func NewTeamGraph() *TeamGraph {
	return &TeamGraph{
		Teams:  make(map[string]*TeamCoordination),
		Nodes:  make(map[string]*NetworkNode),
		Global: make([]Relationship, 0),
	}
}

// AddMember registers an agent as a team member with a role.
func (tc *TeamCoordination) AddMember(agentName string, role AgentRole) {
	tc.Members[NormalizeName(agentName)] = TeamMember{
		AgentName: agentName,
		Role:      role,
	}
}

// AddRelationship records a directed relationship between two agents.
func (tc *TeamCoordination) AddRelationship(from, to string, relType RelationType) {
	rel := Relationship{
		FromAgent: NormalizeName(from),
		ToAgent:   NormalizeName(to),
		Type:      relType,
		Weight:    1.0,
		Metadata:  make(map[string]any),
	}
	tc.Relationships = append(tc.Relationships, rel)
}

// ForRole finds all team members with a given role.
func (tc *TeamCoordination) ForRole(role AgentRole) []TeamMember {
	var result []TeamMember
	for _, member := range tc.Members {
		if member.Role == role {
			result = append(result, member)
		}
	}
	return result
}

// Upstreams returns agents that this agent depends on.
func (tc *TeamCoordination) Upstreams(agentName string) []string {
	normalized := NormalizeName(agentName)
	var result []string
	for _, rel := range tc.Relationships {
		if rel.ToAgent == normalized && rel.Type == RelationDependsOn {
			result = append(result, rel.FromAgent)
		}
	}
	return result
}

// Downstreams returns agents that depend on this agent.
func (tc *TeamCoordination) Downstreams(agentName string) []string {
	normalized := NormalizeName(agentName)
	var result []string
	for _, rel := range tc.Relationships {
		if rel.FromAgent == normalized && rel.Type == RelationDependsOn {
			result = append(result, rel.ToAgent)
		}
	}
	return result
}
