package app

import (
	"context"
	"fmt"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// TeamService orchestrates multi-agent work within a team.
// It uses the graph store to route tasks, find the best agent for work,
// and manage cross-agent communication and approval chains.
type TeamService struct {
	graphStore port.GraphStore
	messenger  port.InterAgentMessenger // optional: for inter-agent comms
	tracer     port.SessionTracer       // optional: for execution tracing
}

// NewTeamService creates a team orchestration service.
func NewTeamService(graphStore port.GraphStore) *TeamService {
	return &TeamService{
		graphStore: graphStore,
	}
}

// SetMessenger registers an optional messenger for inter-agent communication.
func (ts *TeamService) SetMessenger(messenger port.InterAgentMessenger) {
	ts.messenger = messenger
}

// SetTracer registers an optional tracer for execution tracking.
func (ts *TeamService) SetTracer(tracer port.SessionTracer) {
	ts.tracer = tracer
}

// DispatchTask analyzes a task and determines which agent(s) should handle it.
// Strategy: Use role-matching and expertise matching to find the best fit.
func (ts *TeamService) DispatchTask(ctx context.Context, teamName string, task domain.TaskRequest) ([]string, error) {
	if teamName == "" || task.ID == "" {
		return nil, fmt.Errorf("team name and task ID are required")
	}

	team, err := ts.graphStore.GetTeamCoordination(ctx, teamName)
	if err != nil {
		return nil, err
	}

	// Start with all executors
	executors := team.ForRole(domain.RoleExecutor)
	if len(executors) == 0 {
		return nil, fmt.Errorf("no executor agents in team %s", teamName)
	}

	// If we have expertise tags in the task context, prefer agents with matching skills
	if skills, ok := task.Context["skills"].([]string); ok && len(skills) > 0 {
		specialists, _ := ts.graphStore.FindByExpertise(ctx, teamName, skills)
		if len(specialists) > 0 {
			// Return the first specialist
			return []string{specialists[0].AgentName}, nil
		}
	}

	// Default: return the first executor
	return []string{executors[0].AgentName}, nil
}

// GetApprovalChain returns the list of agents who must approve a task,
// based on the team's relationship graph. Typically starts with reviewers.
func (ts *TeamService) GetApprovalChain(ctx context.Context, teamName string, task domain.TaskRequest) ([]string, error) {
	team, err := ts.graphStore.GetTeamCoordination(ctx, teamName)
	if err != nil {
		return nil, err
	}

	// Get all reviewers
	reviewers := team.ForRole(domain.RoleReviewer)
	result := make([]string, 0, len(reviewers))
	for _, r := range reviewers {
		result = append(result, r.AgentName)
	}

	return result, nil
}

// FindAgent searches for an agent matching role and expertise.
func (ts *TeamService) FindAgent(ctx context.Context, teamName string, role domain.AgentRole, skills []string) (string, error) {
	team, err := ts.graphStore.GetTeamCoordination(ctx, teamName)
	if err != nil {
		return "", err
	}

	// Get members with the role
	candidates := team.ForRole(role)
	if len(candidates) == 0 {
		return "", fmt.Errorf("no agents with role %s in team %s", role, teamName)
	}

	// If skills provided, match them
	if len(skills) > 0 {
		for _, candidate := range candidates {
			skillSet := make(map[string]bool)
			for _, s := range candidate.Expertise {
				skillSet[s] = true
			}
			for _, need := range skills {
				if skillSet[need] {
					return candidate.AgentName, nil
				}
			}
		}
	}

	// Fallback: return first candidate
	return candidates[0].AgentName, nil
}

// ExecuteWorkflow runs a named workflow by dispatching tasks in order
// across the agents in the team. This is a simplified sequential execution.
// For complex workflows, use external orchestration or state machines.
func (ts *TeamService) ExecuteWorkflow(ctx context.Context, teamName string, workflowName string, input any) (any, error) {
	team, err := ts.graphStore.GetTeamCoordination(ctx, teamName)
	if err != nil {
		return nil, err
	}

	if len(team.WorkflowOrder) == 0 {
		return nil, fmt.Errorf("no workflow order defined for team %s", teamName)
	}

	// Execute agents in order
	result := input
	for _, agentName := range team.WorkflowOrder {
		// In a real implementation, this would:
		// 1. Invoke the agent with the input
		// 2. Collect the output
		// 3. Pass it to the next agent
		//
		// For now, we'll just trace the workflow structure.
		fmt.Printf("Workflow %s: would execute agent %s\n", workflowName, agentName)
	}

	return result, nil
}

// CheckCanDelegate verifies that an agent has delegation permissions.
func (ts *TeamService) CheckCanDelegate(ctx context.Context, teamName string, agentName string) (bool, error) {
	team, err := ts.graphStore.GetTeamCoordination(ctx, teamName)
	if err != nil {
		return false, err
	}

	key := domain.NormalizeName(agentName)
	member, ok := team.Members[key]
	if !ok {
		return false, fmt.Errorf("agent not found in team: %s", agentName)
	}

	return member.CanDelegate, nil
}

// CheckCanApprove verifies that an agent has approval permissions.
func (ts *TeamService) CheckCanApprove(ctx context.Context, teamName string, agentName string) (bool, error) {
	team, err := ts.graphStore.GetTeamCoordination(ctx, teamName)
	if err != nil {
		return false, err
	}

	key := domain.NormalizeName(agentName)
	member, ok := team.Members[key]
	if !ok {
		return false, fmt.Errorf("agent not found in team: %s", agentName)
	}

	return member.CanApprove, nil
}
