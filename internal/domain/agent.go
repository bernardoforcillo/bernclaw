package domain

import (
	"fmt"
	"strings"
)

// Spec describes a single agent's configuration.
type Spec struct {
	Name         string
	ModelName    string
	Connector    string
	IsDefault    bool
	SystemPrompt string
}

// Team is an in-memory collection of agents ordered by registration.
type Team struct {
	Name   string
	order  []string
	agents map[string]Spec
}

// NewTeam creates a new, empty team with the given name.
func NewTeam(name string) Team {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		cleanName = "team"
	}
	return Team{
		Name:   cleanName,
		order:  []string{},
		agents: map[string]Spec{},
	}
}

// DefaultAgentName returns the name of the first registered agent, or empty.
func (t Team) DefaultAgentName() string {
	if len(t.order) == 0 {
		return ""
	}
	return t.order[0]
}

// Find looks up an agent by name (case-insensitive).
func (t Team) Find(name string) (Spec, bool) {
	key := NormalizeName(name)
	if key == "" {
		return Spec{}, false
	}
	spec, found := t.agents[key]
	return spec, found
}

// Register adds or replaces an agent in the team.
func (t *Team) Register(spec Spec) error {
	if t == nil {
		return fmt.Errorf("team is nil")
	}
	key := NormalizeName(spec.Name)
	if key == "" {
		return fmt.Errorf("agent name is required")
	}
	if t.agents == nil {
		t.agents = map[string]Spec{}
	}
	if _, exists := t.agents[key]; !exists {
		t.order = append(t.order, key)
	}
	t.agents[key] = spec
	return nil
}

// Remove deletes an agent from the team. Returns false if not found.
func (t *Team) Remove(name string) bool {
	if t == nil {
		return false
	}
	key := NormalizeName(name)
	if key == "" {
		return false
	}
	if _, exists := t.agents[key]; !exists {
		return false
	}
	delete(t.agents, key)
	filtered := make([]string, 0, len(t.order))
	for _, item := range t.order {
		if item != key {
			filtered = append(filtered, item)
		}
	}
	t.order = filtered
	return true
}

// Specs returns all agent specs in registration order.
func (t Team) Specs() []Spec {
	specs := make([]Spec, 0, len(t.order))
	for _, key := range t.order {
		if spec, ok := t.agents[key]; ok {
			specs = append(specs, spec)
		}
	}
	return specs
}

// ListNames returns a comma-separated list of agent names.
func (t Team) ListNames() string {
	items := make([]string, 0, len(t.order))
	for _, key := range t.order {
		if spec, ok := t.agents[key]; ok {
			items = append(items, spec.Name)
		}
	}
	return strings.Join(items, ", ")
}

// Describe returns a human-readable summary of the named agent.
func (t Team) Describe(name string) string {
	spec, found := t.Find(name)
	if !found {
		return fmt.Sprintf("Agent not found: %s", name)
	}
	return strings.Join([]string{
		fmt.Sprintf("Agent: %s", spec.Name),
		fmt.Sprintf("Model: %s", spec.ModelName),
		fmt.Sprintf("Default: %t", spec.IsDefault),
		fmt.Sprintf("System prompt: %s", spec.SystemPrompt),
	}, "\n")
}

// NormalizeName returns a canonical lowercase key for a resource name.
func NormalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// StoredTeam is the lightweight record returned by AgentRepository.ListTeams.
type StoredTeam struct {
	Name string
}

// StoredAgent is the record returned by AgentRepository.ListAgents.
type StoredAgent struct {
	Team string
	Spec Spec
}
