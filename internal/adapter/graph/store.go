// Package graph provides in-memory implementations of graph and team coordination
// interfaces. This is suitable for single-process deployments. For larger
// deployments, Neo4j or similar could replace this.
package graph

import (
	"context"
	"fmt"
	"sync"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// Store implements port.GraphStore using in-memory maps.
// All operations are goroutine-safe via RWMutex.
type Store struct {
	mu    sync.RWMutex
	teams map[string]*domain.TeamCoordination
	nodes map[string]*domain.NetworkNode
}

// NewStore creates a new in-memory graph store.
func NewStore() port.GraphStore {
	return &Store{
		teams: make(map[string]*domain.TeamCoordination),
		nodes: make(map[string]*domain.NetworkNode),
	}
}

// SaveTeamCoordination persists a team structure.
func (s *Store) SaveTeamCoordination(ctx context.Context, team *domain.TeamCoordination) error {
	if team == nil {
		return fmt.Errorf("team is nil")
	}
	if team.Name == "" {
		return fmt.Errorf("team name is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := domain.NormalizeName(team.Name)
	s.teams[key] = team
	return nil
}

// GetTeamCoordination retrieves a team by name.
func (s *Store) GetTeamCoordination(ctx context.Context, teamName string) (*domain.TeamCoordination, error) {
	if teamName == "" {
		return nil, fmt.Errorf("team name is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	key := domain.NormalizeName(teamName)
	team, ok := s.teams[key]
	if !ok {
		return nil, fmt.Errorf("team not found: %s", teamName)
	}
	return team, nil
}

// ListTeams returns all team names.
func (s *Store) ListTeams(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.teams))
	for _, team := range s.teams {
		names = append(names, team.Name)
	}
	return names, nil
}

// FindRelated finds agents connected by a specific relationship type.
func (s *Store) FindRelated(ctx context.Context, agentName string, relType domain.RelationType) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalized := domain.NormalizeName(agentName)
	var result []string
	seen := make(map[string]bool)

	for _, team := range s.teams {
		for _, rel := range team.Relationships {
			if rel.Type == relType && rel.FromAgent == normalized {
				if !seen[rel.ToAgent] {
					result = append(result, rel.ToAgent)
					seen[rel.ToAgent] = true
				}
			}
		}
	}
	return result, nil
}

// FindPath uses BFS to find shortest path between two agents.
func (s *Store) FindPath(ctx context.Context, fromAgent, toAgent string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fr := domain.NormalizeName(fromAgent)
	to := domain.NormalizeName(toAgent)

	// Find team containing both agents
	var team *domain.TeamCoordination
	for _, t := range s.teams {
		if _, hasFrom := t.Members[fr]; hasFrom {
			if _, hasTo := t.Members[to]; hasTo {
				team = t
				break
			}
		}
	}
	if team == nil {
		return nil, fmt.Errorf("agents not found in same team")
	}

	// BFS
	visited := make(map[string]bool)
	parent := make(map[string]string)
	queue := []string{fr}
	visited[fr] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == to {
			break
		}

		for _, rel := range team.Relationships {
			if rel.FromAgent == current && !visited[rel.ToAgent] {
				visited[rel.ToAgent] = true
				parent[rel.ToAgent] = current
				queue = append(queue, rel.ToAgent)
			}
		}
	}

	if !visited[to] {
		return nil, fmt.Errorf("no path found")
	}

	// Reconstruct path
	var path []string
	current := to
	for current != "" {
		path = append([]string{current}, path...)
		if current == fr {
			break
		}
		current = parent[current]
	}
	return path, nil
}

// FindByRole returns team members with a specific role.
func (s *Store) FindByRole(ctx context.Context, teamName string, role domain.AgentRole) ([]domain.TeamMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := domain.NormalizeName(teamName)
	team, ok := s.teams[key]
	if !ok {
		return nil, fmt.Errorf("team not found: %s", teamName)
	}

	return team.ForRole(role), nil
}

// FindByExpertise returns team members with matching skills.
func (s *Store) FindByExpertise(ctx context.Context, teamName string, skills []string) ([]domain.TeamMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := domain.NormalizeName(teamName)
	team, ok := s.teams[key]
	if !ok {
		return nil, fmt.Errorf("team not found: %s", teamName)
	}

	var result []domain.TeamMember
	skillSet := make(map[string]bool)
	for _, s := range skills {
		skillSet[s] = true
	}

	for _, member := range team.Members {
		for _, skill := range member.Expertise {
			if skillSet[skill] {
				result = append(result, member)
				break
			}
		}
	}
	return result, nil
}

// AddRelationship records a new relationship.
func (s *Store) AddRelationship(ctx context.Context, teamName string, rel domain.Relationship) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := domain.NormalizeName(teamName)
	team, ok := s.teams[key]
	if !ok {
		return fmt.Errorf("team not found: %s", teamName)
	}

	team.AddRelationship(rel.FromAgent, rel.ToAgent, rel.Type)
	return nil
}

// RemoveRelationship deletes a relationship.
func (s *Store) RemoveRelationship(ctx context.Context, teamName string, rel domain.Relationship) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := domain.NormalizeName(teamName)
	team, ok := s.teams[key]
	if !ok {
		return fmt.Errorf("team not found: %s", teamName)
	}

	// Filter out the matching relationship
	filtered := make([]domain.Relationship, 0, len(team.Relationships))
	for _, r := range team.Relationships {
		if !(r.FromAgent == rel.FromAgent && r.ToAgent == rel.ToAgent && r.Type == rel.Type) {
			filtered = append(filtered, r)
		}
	}
	team.Relationships = filtered
	return nil
}

// RegisterNode adds a network node.
func (s *Store) RegisterNode(ctx context.Context, node domain.NetworkNode) error {
	if node.ID == "" {
		return fmt.Errorf("node ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nodes[node.ID] = &node
	return nil
}

// HeartbeatNode updates a node's last-seen timestamp.
func (s *Store) HeartbeatNode(ctx context.Context, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	node.LastHeartbeat = domain.NowUnix()
	return nil
}

// FindNodesForAgent returns nodes hosting a given agent.
func (s *Store) FindNodesForAgent(ctx context.Context, agentName string) ([]domain.NetworkNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normalized := domain.NormalizeName(agentName)
	var result []domain.NetworkNode

	for _, node := range s.nodes {
		for _, agent := range node.Agents {
			if domain.NormalizeName(agent) == normalized {
				result = append(result, *node)
				break
			}
		}
	}
	return result, nil
}
