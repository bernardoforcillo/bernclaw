package core

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bernardoforcillo/bernclaw/internal/agent"
)

type resolvedAgentRef struct {
	Spec      agent.Spec
	TeamName  string
	ScopedKey string
}

type agentRegistryState struct {
	store            agentResourceStore
	teams            map[string]*agent.Team
	standaloneAgents map[string]agent.Spec
	activeTeam       string
	activeAgent      string
	defaultModelName string
}

func newAgentRegistryState(store agentResourceStore, defaultModelName string) agentRegistryState {
	return agentRegistryState{
		store:            store,
		teams:            map[string]*agent.Team{},
		standaloneAgents: map[string]agent.Spec{},
		activeTeam:       "",
		activeAgent:      "",
		defaultModelName: strings.TrimSpace(defaultModelName),
	}
}

func (state *agentRegistryState) resolveActiveAgent() (agent.Spec, bool) {
	name := strings.TrimSpace(state.activeAgent)
	if name == "" {
		return agent.Spec{}, false
	}

	if team := state.getActiveTeam(); team != nil {
		if spec, found := team.Find(name); found {
			return spec, true
		}
	}

	if spec, found := state.standaloneAgents[normalizeEntityName(name)]; found {
		return spec, true
	}

	return agent.Spec{}, false
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
			key := normalizeEntityName(spec.Name)
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
		key := normalizeEntityName(spec.Name)
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
	normalized := normalizeEntityName(kind)
	switch normalized {
	case "agent":
		return state.listAgentNames()
	case "team", "swarm":
		return state.teamNames()
	default:
		return nil
	}
}

func (state *agentRegistryState) loadStoredResources() error {
	teams, err := state.store.ListTeams()
	if err != nil {
		return err
	}

	loadedTeams := map[string]*agent.Team{}
	for _, item := range teams {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		team := agent.NewTeam(name)
		key := normalizeEntityName(team.Name)
		loadedTeams[key] = &team
	}

	agents, err := state.store.ListAgents()
	if err != nil {
		return err
	}

	standalone := map[string]agent.Spec{}
	defaultActiveTeam := ""
	defaultActiveAgent := ""
	for _, item := range agents {
		if strings.TrimSpace(item.Spec.Name) == "" {
			continue
		}

		teamName := strings.TrimSpace(item.Team)
		if teamName == "" {
			standalone[normalizeEntityName(item.Spec.Name)] = item.Spec
			if item.Spec.IsDefault && defaultActiveAgent == "" {
				defaultActiveTeam = ""
				defaultActiveAgent = item.Spec.Name
			}
			continue
		}

		teamKey := normalizeEntityName(teamName)
		team := loadedTeams[teamKey]
		if team == nil {
			newTeam := agent.NewTeam(teamName)
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

func (state *agentRegistryState) getActiveTeam() *agent.Team {
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

	key := normalizeEntityName(teamName)
	if key == "" {
		return "", fmt.Errorf("invalid team name")
	}
	if _, exists := state.teams[key]; exists {
		return "", fmt.Errorf("team already exists: %s", teamName)
	}

	team := agent.NewTeam(teamName)
	state.teams[key] = &team
	state.activeTeam = key
	state.activeAgent = ""

	if err := state.store.SaveTeam(teamName); err != nil {
		delete(state.teams, key)
		state.activeTeam = ""
		return "", err
	}

	return teamName, nil
}

func (state *agentRegistryState) useTeam(name string) error {
	key := normalizeEntityName(name)
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

func (state *agentRegistryState) createAgent(name string) error {
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
					_ = state.store.SaveAgent(existing, team.Name)
				}
			}
		}

		spec := agent.Spec{
			Name:         agentName,
			ModelName:    state.defaultModelName,
			IsDefault:    isDefault,
			SystemPrompt: fmt.Sprintf("You are %s agent. Be concise and helpful.", agentName),
		}

		if err := team.Register(spec); err != nil {
			return err
		}
		if err := state.store.SaveAgent(spec, team.Name); err != nil {
			_ = team.Remove(agentName)
			return err
		}
		if strings.TrimSpace(state.activeAgent) == "" {
			state.activeAgent = agentName
		}
		return nil
	}

	key := normalizeEntityName(agentName)
	if _, found := state.standaloneAgents[key]; found {
		return fmt.Errorf("agent already exists: %s", agentName)
	}
	isDefault := strings.TrimSpace(state.activeAgent) == ""
	if isDefault {
		for existingKey, existing := range state.standaloneAgents {
			existing.IsDefault = false
			state.standaloneAgents[existingKey] = existing
			_ = state.store.SaveAgent(existing, "")
		}
	}

	spec := agent.Spec{
		Name:         agentName,
		ModelName:    state.defaultModelName,
		IsDefault:    isDefault,
		SystemPrompt: fmt.Sprintf("You are %s agent. Be concise and helpful.", agentName),
	}
	state.standaloneAgents[key] = spec
	if err := state.store.SaveAgent(spec, ""); err != nil {
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

	if _, found := state.standaloneAgents[normalizeEntityName(agentName)]; !found {
		return fmt.Errorf("agent not found: %s", agentName)
	}

	state.activeAgent = agentName
	return nil
}

func (state *agentRegistryState) getTeam(name string) (string, error) {
	key := normalizeEntityName(name)
	team, ok := state.teams[key]
	if !ok || team == nil {
		return "", fmt.Errorf("team not found: %s", name)
	}
	return "Team: " + team.Name + "\nAgents: " + team.ListNames(), nil
}

func (state *agentRegistryState) updateTeam(currentName string, nextName string) error {
	currentKey := normalizeEntityName(currentName)
	team, ok := state.teams[currentKey]
	if !ok || team == nil {
		return fmt.Errorf("team not found: %s", currentName)
	}

	newLabel := strings.TrimSpace(nextName)
	if newLabel == "" {
		return fmt.Errorf("new team name is required")
	}
	newKey := normalizeEntityName(newLabel)
	if newKey != currentKey {
		if _, exists := state.teams[newKey]; exists {
			return fmt.Errorf("team already exists: %s", newLabel)
		}
	}

	oldLabel := team.Name
	oldSpecs := append([]agent.Spec{}, team.Specs()...)
	team.Name = newLabel

	delete(state.teams, currentKey)
	state.teams[newKey] = team

	if state.activeTeam == currentKey {
		state.activeTeam = newKey
	}

	if err := state.store.SaveTeam(newLabel); err != nil {
		team.Name = oldLabel
		delete(state.teams, newKey)
		state.teams[currentKey] = team
		if state.activeTeam == newKey {
			state.activeTeam = currentKey
		}
		return err
	}

	for _, spec := range oldSpecs {
		if err := state.store.SaveAgent(spec, newLabel); err != nil {
			return err
		}
		_ = state.store.DeleteAgent(spec.Name, oldLabel)
	}
	_ = state.store.DeleteTeam(oldLabel)

	return nil
}

func (state *agentRegistryState) deleteTeam(name string) error {
	key := normalizeEntityName(name)
	team, exists := state.teams[key]
	if !exists || team == nil {
		return fmt.Errorf("team not found: %s", name)
	}

	specs := append([]agent.Spec{}, team.Specs()...)
	for _, spec := range specs {
		if err := state.store.DeleteAgent(spec.Name, team.Name); err != nil {
			return err
		}
	}
	if err := state.store.DeleteTeam(team.Name); err != nil {
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
			return resolvedAgentRef{Spec: spec, TeamName: team.Name, ScopedKey: normalizeEntityName(spec.Name)}, true
		}
	}

	if spec, found := state.standaloneAgents[normalizeEntityName(needle)]; found {
		return resolvedAgentRef{Spec: spec, TeamName: "", ScopedKey: normalizeEntityName(spec.Name)}, true
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
		team := state.teams[normalizeEntityName(resolved.TeamName)]
		if team == nil {
			return fmt.Errorf("team not found: %s", resolved.TeamName)
		}
		if err := team.Register(updated); err != nil {
			return err
		}
		return state.store.SaveAgent(updated, resolved.TeamName)
	}

	state.standaloneAgents[resolved.ScopedKey] = updated
	return state.store.SaveAgent(updated, "")
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
		team := state.teams[normalizeEntityName(resolved.TeamName)]
		if team == nil {
			return fmt.Errorf("team not found: %s", resolved.TeamName)
		}
		if err := team.Register(updated); err != nil {
			return err
		}
		if err := state.store.SaveAgent(updated, resolved.TeamName); err != nil {
			return err
		}
		state.activeTeam = normalizeEntityName(resolved.TeamName)
	} else {
		state.standaloneAgents[resolved.ScopedKey] = updated
		if err := state.store.SaveAgent(updated, ""); err != nil {
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
			if err := state.store.SaveAgent(spec, team.Name); err != nil {
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
		if err := state.store.SaveAgent(spec, ""); err != nil {
			return err
		}
	}

	return nil
}

func (state *agentRegistryState) normalizeDefaultAgents(defaultTeam string, defaultAgent string) error {
	chosenTeamKey := normalizeEntityName(defaultTeam)
	chosenAgentKey := normalizeEntityName(defaultAgent)
	if chosenAgentKey == "" {
		return nil
	}

	for teamKey, team := range state.teams {
		if team == nil {
			continue
		}
		for _, spec := range team.Specs() {
			shouldDefault := teamKey == chosenTeamKey && normalizeEntityName(spec.Name) == chosenAgentKey
			if spec.IsDefault == shouldDefault {
				continue
			}
			spec.IsDefault = shouldDefault
			if err := team.Register(spec); err != nil {
				return err
			}
			if err := state.store.SaveAgent(spec, team.Name); err != nil {
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
		if err := state.store.SaveAgent(spec, ""); err != nil {
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
		team := state.teams[normalizeEntityName(resolved.TeamName)]
		if team == nil {
			return fmt.Errorf("team not found: %s", resolved.TeamName)
		}
		if !team.Remove(resolved.Spec.Name) {
			return fmt.Errorf("agent not found: %s", name)
		}
		if err := state.store.DeleteAgent(resolved.Spec.Name, resolved.TeamName); err != nil {
			return err
		}
	} else {
		delete(state.standaloneAgents, resolved.ScopedKey)
		if err := state.store.DeleteAgent(resolved.Spec.Name, ""); err != nil {
			return err
		}
	}

	if normalizeEntityName(state.activeAgent) == normalizeEntityName(resolved.Spec.Name) {
		state.activeAgent = ""
	}
	return nil
}

func normalizeEntityName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
