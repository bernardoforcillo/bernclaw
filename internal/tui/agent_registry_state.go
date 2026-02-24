package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

type resolvedAgentRef struct {
	Spec      domain.Spec
	TeamName  string
	ScopedKey string
}

type agentRegistryState struct {
	agents           port.AgentRepository
	connectors       port.ConnectorRepository
	teams            map[string]*domain.Team
	standaloneAgents map[string]domain.Spec
	activeTeam       string
	activeAgent      string
	defaultModelName string
}

func newAgentRegistryState(agents port.AgentRepository, connectors port.ConnectorRepository, defaultModelName string) agentRegistryState {
	return agentRegistryState{
		agents:           agents,
		connectors:       connectors,
		teams:            map[string]*domain.Team{},
		standaloneAgents: map[string]domain.Spec{},
		activeTeam:       "",
		activeAgent:      "",
		defaultModelName: strings.TrimSpace(defaultModelName),
	}
}

func (state *agentRegistryState) resolveActiveAgent() (domain.Spec, bool) {
	name := strings.TrimSpace(state.activeAgent)
	if name == "" {
		return domain.Spec{}, false
	}

	if team := state.getActiveTeam(); team != nil {
		if spec, found := team.Find(name); found {
			return spec, true
		}
	}

	if spec, found := state.standaloneAgents[domain.NormalizeName(name)]; found {
		return spec, true
	}

	return domain.Spec{}, false
}

func (state *agentRegistryState) resolveActiveModelName() (string, error) {
	active, ok := state.resolveActiveAgent()
	if !ok {
		return "", fmt.Errorf("no active agent set • use /agent create [name]")
	}
	modelName := strings.TrimSpace(active.ModelName)
	if modelName == "" {
		return "", fmt.Errorf("active agent model is empty • set spec.model.name in agent resource")
	}
	return modelName, nil
}

func (state *agentRegistryState) activeModelLabel() string {
	modelName, err := state.resolveActiveModelName()
	if err != nil {
		return "none"
	}
	return modelName
}

func (state *agentRegistryState) listAgentNames() []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, 8)

	if team := state.getActiveTeam(); team != nil {
		for _, spec := range team.Specs() {
			if strings.TrimSpace(spec.Name) == "" {
				continue
			}
			key := domain.NormalizeName(spec.Name)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			items = append(items, spec.Name)
		}
	}

	for _, spec := range state.standaloneAgents {
		if strings.TrimSpace(spec.Name) == "" {
			continue
		}
		key := domain.NormalizeName(spec.Name)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, spec.Name)
	}

	sort.Strings(items)
	return items
}

func (state *agentRegistryState) resourceNames(kind string) []string {
	normalized := domain.NormalizeName(kind)
	switch normalized {
	case "agent":
		return state.listAgentNames()
	case "team", "swarm":
		return state.teamNames()
	case "connector":
		connectors, err := state.connectors.ListConnectors()
		if err != nil {
			return nil
		}
		names := make([]string, 0, len(connectors))
		for _, item := range connectors {
			name := strings.TrimSpace(item.Name)
			if name != "" {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		return names
	default:
		return nil
	}
}

func (state *agentRegistryState) loadStoredResources() error {
	teams, err := state.agents.ListTeams()
	if err != nil {
		return err
	}

	loadedTeams := map[string]*domain.Team{}
	for _, item := range teams {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		team := domain.NewTeam(name)
		key := domain.NormalizeName(team.Name)
		loadedTeams[key] = &team
	}

	agents, err := state.agents.ListAgents()
	if err != nil {
		return err
	}

	standalone := map[string]domain.Spec{}
	defaultActiveTeam := ""
	defaultActiveAgent := ""
	for _, item := range agents {
		if strings.TrimSpace(item.Spec.Name) == "" {
			continue
		}

		teamName := strings.TrimSpace(item.Team)
		if teamName == "" {
			standalone[domain.NormalizeName(item.Spec.Name)] = item.Spec
			if item.Spec.IsDefault && defaultActiveAgent == "" {
				defaultActiveTeam = ""
				defaultActiveAgent = item.Spec.Name
			}
			continue
		}

		teamKey := domain.NormalizeName(teamName)
		team := loadedTeams[teamKey]
		if team == nil {
			newTeam := domain.NewTeam(teamName)
			loadedTeams[teamKey] = &newTeam
			team = &newTeam
		}

		if err := team.Register(item.Spec); err != nil {
			continue
		}
		if item.Spec.IsDefault && defaultActiveAgent == "" {
			defaultActiveTeam = teamKey
			defaultActiveAgent = item.Spec.Name
		}
	}

	state.teams = loadedTeams
	state.standaloneAgents = standalone
	if state.activeTeam != "" {
		if _, ok := state.teams[state.activeTeam]; !ok {
			state.activeTeam = ""
		}
	}
	if state.activeAgent != "" {
		if _, ok := state.resolveActiveAgent(); !ok {
			state.activeAgent = ""
		}
	}
	if defaultActiveAgent != "" {
		state.activeTeam = defaultActiveTeam
		state.activeAgent = defaultActiveAgent
	}

	if err := state.normalizeDefaultAgents(defaultActiveTeam, defaultActiveAgent); err != nil {
		return err
	}

	return nil
}

func (state *agentRegistryState) getActiveTeam() *domain.Team {
	if state == nil {
		return nil
	}
	return state.teams[state.activeTeam]
}

func (state *agentRegistryState) teamNames() []string {
	names := make([]string, 0, len(state.teams))
	for _, team := range state.teams {
		if team == nil {
			continue
		}
		names = append(names, team.Name)
	}
	sort.Strings(names)
	return names
}

func (state *agentRegistryState) initTeam(name string) (string, error) {
	teamName := strings.TrimSpace(name)
	if teamName == "" {
		teamName = fmt.Sprintf("team-%d", time.Now().Unix())
	}

	key := domain.NormalizeName(teamName)
	if key == "" {
		return "", fmt.Errorf("invalid team name")
	}
	if _, exists := state.teams[key]; exists {
		return "", fmt.Errorf("team already exists: %s", teamName)
	}

	team := domain.NewTeam(teamName)
	state.teams[key] = &team
	state.activeTeam = key
	state.activeAgent = ""

	if err := state.agents.SaveTeam(teamName); err != nil {
		delete(state.teams, key)
		state.activeTeam = ""
		return "", err
	}

	return teamName, nil
}

func (state *agentRegistryState) useTeam(name string) error {
	key := domain.NormalizeName(name)
	team, exists := state.teams[key]
	if !exists || team == nil {
		return fmt.Errorf("team not found: %s", name)
	}

	state.activeTeam = key
	if state.activeAgent == "" {
		state.activeAgent = team.DefaultAgentName()
	}
	if state.activeAgent != "" {
		if _, found := team.Find(state.activeAgent); !found {
			state.activeAgent = team.DefaultAgentName()
		}
	}

	return nil
}

func (state *agentRegistryState) createAgent(name string, connector string) error {
	agentName := strings.TrimSpace(name)
	if agentName == "" {
		return fmt.Errorf("agent name is required")
	}

	if team := state.getActiveTeam(); team != nil {
		if _, found := team.Find(agentName); found {
			return fmt.Errorf("agent already exists: %s", agentName)
		}

		isDefault := strings.TrimSpace(state.activeAgent) == ""
		if isDefault {
			for _, existing := range team.Specs() {
				existing.IsDefault = false
				if err := team.Register(existing); err == nil {
					_ = state.agents.SaveAgent(existing, team.Name)
				}
			}
		}

		spec := domain.Spec{
			Name:         agentName,
			ModelName:    state.defaultModelName,
			Connector:    strings.TrimSpace(connector),
			IsDefault:    isDefault,
			SystemPrompt: fmt.Sprintf("You are %s agent. Be concise and helpful.", agentName),
		}

		if err := team.Register(spec); err != nil {
			return err
		}
		if err := state.agents.SaveAgent(spec, team.Name); err != nil {
			_ = team.Remove(agentName)
			return err
		}
		if strings.TrimSpace(state.activeAgent) == "" {
			state.activeAgent = agentName
		}
		return nil
	}

	key := domain.NormalizeName(agentName)
	if _, found := state.standaloneAgents[key]; found {
		return fmt.Errorf("agent already exists: %s", agentName)
	}
	isDefault := strings.TrimSpace(state.activeAgent) == ""
	if isDefault {
		for existingKey, existing := range state.standaloneAgents {
			existing.IsDefault = false
			state.standaloneAgents[existingKey] = existing
			_ = state.agents.SaveAgent(existing, "")
		}
	}

	spec := domain.Spec{
		Name:         agentName,
		ModelName:    state.defaultModelName,
		Connector:    strings.TrimSpace(connector),
		IsDefault:    isDefault,
		SystemPrompt: fmt.Sprintf("You are %s agent. Be concise and helpful.", agentName),
	}
	state.standaloneAgents[key] = spec
	if err := state.agents.SaveAgent(spec, ""); err != nil {
		delete(state.standaloneAgents, key)
		return err
	}
	if strings.TrimSpace(state.activeAgent) == "" {
		state.activeAgent = agentName
	}
	return nil
}

func (state *agentRegistryState) useAgent(name string) error {
	agentName := strings.TrimSpace(name)
	if agentName == "" {
		return fmt.Errorf("agent name is required")
	}

	if team := state.getActiveTeam(); team != nil {
		if _, found := team.Find(agentName); found {
			state.activeAgent = agentName
			return nil
		}
	}

	if _, found := state.standaloneAgents[domain.NormalizeName(agentName)]; !found {
		return fmt.Errorf("agent not found: %s", agentName)
	}

	state.activeAgent = agentName
	return nil
}

func (state *agentRegistryState) getTeam(name string) (string, error) {
	key := domain.NormalizeName(name)
	team, ok := state.teams[key]
	if !ok || team == nil {
		return "", fmt.Errorf("team not found: %s", name)
	}
	return "Team: " + team.Name + "\nAgents: " + team.ListNames(), nil
}

func (state *agentRegistryState) updateTeam(currentName string, nextName string) error {
	currentKey := domain.NormalizeName(currentName)
	team, ok := state.teams[currentKey]
	if !ok || team == nil {
		return fmt.Errorf("team not found: %s", currentName)
	}

	newLabel := strings.TrimSpace(nextName)
	if newLabel == "" {
		return fmt.Errorf("new team name is required")
	}
	newKey := domain.NormalizeName(newLabel)
	if newKey != currentKey {
		if _, exists := state.teams[newKey]; exists {
			return fmt.Errorf("team already exists: %s", newLabel)
		}
	}

	oldLabel := team.Name
	oldSpecs := append([]domain.Spec{}, team.Specs()...)
	team.Name = newLabel

	delete(state.teams, currentKey)
	state.teams[newKey] = team

	if state.activeTeam == currentKey {
		state.activeTeam = newKey
	}

	if err := state.agents.SaveTeam(newLabel); err != nil {
		team.Name = oldLabel
		delete(state.teams, newKey)
		state.teams[currentKey] = team
		if state.activeTeam == newKey {
			state.activeTeam = currentKey
		}
		return err
	}

	for _, spec := range oldSpecs {
		if err := state.agents.SaveAgent(spec, newLabel); err != nil {
			return err
		}
		_ = state.agents.DeleteAgent(spec.Name, oldLabel)
	}
	_ = state.agents.DeleteTeam(oldLabel)

	return nil
}

func (state *agentRegistryState) deleteTeam(name string) error {
	key := domain.NormalizeName(name)
	team, exists := state.teams[key]
	if !exists || team == nil {
		return fmt.Errorf("team not found: %s", name)
	}

	specs := append([]domain.Spec{}, team.Specs()...)
	for _, spec := range specs {
		if err := state.agents.DeleteAgent(spec.Name, team.Name); err != nil {
			return err
		}
	}
	if err := state.agents.DeleteTeam(team.Name); err != nil {
		return err
	}

	delete(state.teams, key)
	if state.activeTeam == key {
		state.activeTeam = ""
		state.activeAgent = ""
	}
	return nil
}

func (state *agentRegistryState) resolveAgent(name string) (resolvedAgentRef, bool) {
	needle := strings.TrimSpace(name)
	if needle == "" {
		return resolvedAgentRef{}, false
	}

	if team := state.getActiveTeam(); team != nil {
		if spec, found := team.Find(needle); found {
			return resolvedAgentRef{Spec: spec, TeamName: team.Name, ScopedKey: domain.NormalizeName(spec.Name)}, true
		}
	}

	if spec, found := state.standaloneAgents[domain.NormalizeName(needle)]; found {
		return resolvedAgentRef{Spec: spec, TeamName: "", ScopedKey: domain.NormalizeName(spec.Name)}, true
	}

	return resolvedAgentRef{}, false
}

func (state *agentRegistryState) getAgent(name string) (string, error) {
	resolved, ok := state.resolveAgent(name)
	if !ok {
		return "", fmt.Errorf("agent not found: %s", name)
	}
	scope := "standalone"
	if resolved.TeamName != "" {
		scope = resolved.TeamName
	}
	return strings.Join([]string{
		"Agent: " + resolved.Spec.Name,
		"Scope: " + scope,
		"Model: " + resolved.Spec.ModelName,
		fmt.Sprintf("Default: %t", resolved.Spec.IsDefault),
		"System prompt: " + resolved.Spec.SystemPrompt,
	}, "\n"), nil
}

func (state *agentRegistryState) updateAgent(name string, newSystemPrompt string) error {
	prompt := strings.TrimSpace(newSystemPrompt)
	if prompt == "" {
		return fmt.Errorf("system prompt is required")
	}

	resolved, ok := state.resolveAgent(name)
	if !ok {
		return fmt.Errorf("agent not found: %s", name)
	}

	updated := resolved.Spec
	updated.SystemPrompt = prompt

	if resolved.TeamName != "" {
		team := state.teams[domain.NormalizeName(resolved.TeamName)]
		if team == nil {
			return fmt.Errorf("team not found: %s", resolved.TeamName)
		}
		if err := team.Register(updated); err != nil {
			return err
		}
		return state.agents.SaveAgent(updated, resolved.TeamName)
	}

	state.standaloneAgents[resolved.ScopedKey] = updated
	return state.agents.SaveAgent(updated, "")
}

func (state *agentRegistryState) setDefaultAgent(name string) error {
	resolved, ok := state.resolveAgent(name)
	if !ok {
		return fmt.Errorf("agent not found: %s", name)
	}

	if err := state.clearDefaultAgentFlags(); err != nil {
		return err
	}

	updated := resolved.Spec
	updated.IsDefault = true
	if resolved.TeamName != "" {
		team := state.teams[domain.NormalizeName(resolved.TeamName)]
		if team == nil {
			return fmt.Errorf("team not found: %s", resolved.TeamName)
		}
		if err := team.Register(updated); err != nil {
			return err
		}
		if err := state.agents.SaveAgent(updated, resolved.TeamName); err != nil {
			return err
		}
		state.activeTeam = domain.NormalizeName(resolved.TeamName)
	} else {
		state.standaloneAgents[resolved.ScopedKey] = updated
		if err := state.agents.SaveAgent(updated, ""); err != nil {
			return err
		}
		state.activeTeam = ""
	}

	state.activeAgent = updated.Name
	return nil
}

func (state *agentRegistryState) clearDefaultAgentFlags() error {
	for _, team := range state.teams {
		if team == nil {
			continue
		}
		for _, spec := range team.Specs() {
			if !spec.IsDefault {
				continue
			}
			spec.IsDefault = false
			if err := team.Register(spec); err != nil {
				return err
			}
			if err := state.agents.SaveAgent(spec, team.Name); err != nil {
				return err
			}
		}
	}

	for key, spec := range state.standaloneAgents {
		if !spec.IsDefault {
			continue
		}
		spec.IsDefault = false
		state.standaloneAgents[key] = spec
		if err := state.agents.SaveAgent(spec, ""); err != nil {
			return err
		}
	}

	return nil
}

func (state *agentRegistryState) normalizeDefaultAgents(defaultTeam string, defaultAgent string) error {
	chosenTeamKey := domain.NormalizeName(defaultTeam)
	chosenAgentKey := domain.NormalizeName(defaultAgent)
	if chosenAgentKey == "" {
		return nil
	}

	for teamKey, team := range state.teams {
		if team == nil {
			continue
		}
		for _, spec := range team.Specs() {
			shouldDefault := teamKey == chosenTeamKey && domain.NormalizeName(spec.Name) == chosenAgentKey
			if spec.IsDefault == shouldDefault {
				continue
			}
			spec.IsDefault = shouldDefault
			if err := team.Register(spec); err != nil {
				return err
			}
			if err := state.agents.SaveAgent(spec, team.Name); err != nil {
				return err
			}
		}
	}

	for key, spec := range state.standaloneAgents {
		shouldDefault := chosenTeamKey == "" && key == chosenAgentKey
		if spec.IsDefault == shouldDefault {
			continue
		}
		spec.IsDefault = shouldDefault
		state.standaloneAgents[key] = spec
		if err := state.agents.SaveAgent(spec, ""); err != nil {
			return err
		}
	}

	return nil
}

func (state *agentRegistryState) deleteAgent(name string) error {
	resolved, ok := state.resolveAgent(name)
	if !ok {
		return fmt.Errorf("agent not found: %s", name)
	}

	if resolved.TeamName != "" {
		team := state.teams[domain.NormalizeName(resolved.TeamName)]
		if team == nil {
			return fmt.Errorf("team not found: %s", resolved.TeamName)
		}
		if !team.Remove(resolved.Spec.Name) {
			return fmt.Errorf("agent not found: %s", name)
		}
		if err := state.agents.DeleteAgent(resolved.Spec.Name, resolved.TeamName); err != nil {
			return err
		}
	} else {
		delete(state.standaloneAgents, resolved.ScopedKey)
		if err := state.agents.DeleteAgent(resolved.Spec.Name, ""); err != nil {
			return err
		}
	}

	if domain.NormalizeName(state.activeAgent) == domain.NormalizeName(resolved.Spec.Name) {
		state.activeAgent = ""
	}
	return nil
}
