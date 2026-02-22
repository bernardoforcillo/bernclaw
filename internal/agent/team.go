package agent

import (
	"fmt"
	"strings"
)

type Spec struct {
	Name         string
	ModelName    string
	IsDefault    bool
	SystemPrompt string
}

type Team struct {
	Name   string
	order  []string
	agents map[string]Spec
}

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

func (team Team) DefaultAgentName() string {
	if len(team.order) == 0 {
		return ""
	}
	return team.order[0]
}

func (team Team) Find(name string) (Spec, bool) {
	key := normalizeName(name)
	if key == "" {
		return Spec{}, false
	}
	agent, found := team.agents[key]
	return agent, found
}

func (team *Team) Register(spec Spec) error {
	if team == nil {
		return fmt.Errorf("team is nil")
	}

	key := normalizeName(spec.Name)
	if key == "" {
		return fmt.Errorf("agent name is required")
	}

	if team.agents == nil {
		team.agents = map[string]Spec{}
	}
	if _, exists := team.agents[key]; !exists {
		team.order = append(team.order, key)
	}
	team.agents[key] = spec
	return nil
}

func (team *Team) Remove(name string) bool {
	if team == nil {
		return false
	}

	key := normalizeName(name)
	if key == "" {
		return false
	}
	if _, exists := team.agents[key]; !exists {
		return false
	}

	delete(team.agents, key)
	filtered := make([]string, 0, len(team.order))
	for _, item := range team.order {
		if item != key {
			filtered = append(filtered, item)
		}
	}
	team.order = filtered
	return true
}

func (team Team) Specs() []Spec {
	specs := make([]Spec, 0, len(team.order))
	for _, key := range team.order {
		if spec, ok := team.agents[key]; ok {
			specs = append(specs, spec)
		}
	}
	return specs
}

func (team Team) ListNames() string {
	items := make([]string, 0, len(team.order))
	for _, key := range team.order {
		agent := team.agents[key]
		items = append(items, agent.Name)
	}
	return strings.Join(items, ", ")
}

func (team Team) Describe(name string) string {
	agent, found := team.Find(name)
	if !found {
		return fmt.Sprintf("Agent not found: %s", name)
	}

	return strings.Join([]string{
		fmt.Sprintf("Agent: %s", agent.Name),
		fmt.Sprintf("Model: %s", agent.ModelName),
		fmt.Sprintf("Default: %t", agent.IsDefault),
		fmt.Sprintf("System prompt: %s", agent.SystemPrompt),
	}, "\n")
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
