package tui

import (
	"fmt"
	"strings"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	tea "github.com/charmbracelet/bubbletea"
)

type commandHandler func(model *AppContext, args []string) tea.Cmd

type commandDefinition struct {
	Name        string
	Aliases     []string
	Description string
	Handler     commandHandler
}

type commandRegistry struct {
	definitions []commandDefinition
	lookup      map[string]commandDefinition
	subcommands subcommandCatalog
}

type subcommandCatalog struct {
	definitions map[string][]subcommandDefinition
	lookup      map[string]map[string]subcommandDefinition
}

func newSubcommandCatalog() subcommandCatalog {
	definitions := map[string][]subcommandDefinition{
		"swarm":     buildSwarmSubcommands(),
		"agent":     buildAgentSubcommands(),
		"connector": buildConnectorSubcommands(),
		"member":    buildMemberSubcommands(),
		"task":      buildTaskSubcommands(),
	}

	lookup := map[string]map[string]subcommandDefinition{}
	for parent, items := range definitions {
		parentLookup := map[string]subcommandDefinition{}
		for _, definition := range items {
			parentLookup[definition.Name] = definition
			for _, alias := range definition.Aliases {
				parentLookup[alias] = definition
			}
		}
		lookup[parent] = parentLookup
	}

	return subcommandCatalog{definitions: definitions, lookup: lookup}
}

func (catalog subcommandCatalog) get(parent string) []subcommandDefinition {
	key := strings.ToLower(strings.TrimSpace(parent))
	if key == "" {
		return nil
	}
	return catalog.definitions[key]
}

func (catalog subcommandCatalog) resolve(parent string, token string) (subcommandDefinition, bool) {
	parentKey := strings.ToLower(strings.TrimSpace(parent))
	if parentKey == "" {
		return subcommandDefinition{}, false
	}
	itemKey := strings.ToLower(strings.TrimSpace(token))
	if itemKey == "" {
		return subcommandDefinition{}, false
	}
	parentLookup := catalog.lookup[parentKey]
	if parentLookup == nil {
		return subcommandDefinition{}, false
	}
	item, ok := parentLookup[itemKey]
	return item, ok
}

type resourceNameProvider interface {
	resourceNames(kind string) []string
}

type commandSuggestion struct {
	Title string
	Usage string
}

type subcommandDefinition struct {
	Name         string
	Aliases      []string
	Usage        string
	Description  string
	MinArgs      int
	ResourceArgs []string
	Handler      commandHandler
}

func newCommandRegistry() commandRegistry {
	subcommands := newSubcommandCatalog()

	definitions := []commandDefinition{
		helpCommand(),
		swarmCommand(subcommands),
		agentCommand(subcommands),
		connectorCommand(subcommands),
		memberCommand(subcommands),
		taskCommand(subcommands),
		clearCommand(),
		systemCommand(),
		settingCommand(),
		quitCommand(),
	}

	lookup := make(map[string]commandDefinition, len(definitions)*2)
	for _, definition := range definitions {
		lookup[definition.Name] = definition
		for _, alias := range definition.Aliases {
			lookup[alias] = definition
		}
	}

	return commandRegistry{definitions: definitions, lookup: lookup, subcommands: subcommands}
}

func swarmCommand(catalog subcommandCatalog) commandDefinition {
	return commandDefinition{
		Name:        "swarm",
		Description: "Team CRUD: /swarm create|get|update|delete|list|use",
		Handler: func(model *AppContext, args []string) tea.Cmd {
			if len(args) == 0 {
				teamLabel := "none"
				if team := model.getActiveTeam(); team != nil && strings.TrimSpace(team.Name) != "" {
					teamLabel = team.Name
				}
				teamList := strings.Join(model.teamNames(), ", ")
				if strings.TrimSpace(teamList) == "" {
					teamList = "(none)"
				}
				model.appendUtilityMessage("Active team: "+teamLabel+"\nTeams: "+teamList, "Utility: swarm")
				return nil
			}

			if cmd, handled := dispatchSubcommand("swarm", model, args, catalog); handled {
				return cmd
			}

			if err := model.useTeam(strings.Join(args, " ")); err != nil {
				model.appendUtilityMessage("Usage: /swarm create|get|update|delete|list|use", "Utility: swarm")
				return nil
			}
			model.appendUtilityMessage("Switched to team: "+strings.Join(args, " "), "Utility: swarm")
			return nil
		},
	}
}

func agentCommand(catalog subcommandCatalog) commandDefinition {
	return commandDefinition{
		Name:        "agent",
		Description: "Agent CRUD: /agent create|get|update|delete|list|use|default",
		Handler: func(model *AppContext, args []string) tea.Cmd {
			if len(args) == 0 {
				agentList := strings.Join(model.listAgentNames(), ", ")
				if strings.TrimSpace(agentList) == "" {
					agentList = "(none)"
				}
				active := model.activeAgentName()
				if strings.TrimSpace(active) == "" {
					active = "none"
				}
				model.appendUtilityMessage("Active agent: "+active+"\nAgents: "+agentList, "Utility: agent")
				return nil
			}

			if cmd, handled := dispatchSubcommand("agent", model, args, catalog); handled {
				return cmd
			}

			name := strings.TrimSpace(strings.Join(args, " "))
			if err := model.useAgent(name); err != nil {
				model.appendUtilityMessage(err.Error(), "Utility: agent")
				return nil
			}
			model.appendUtilityMessage("Using agent: "+name, "Utility: agent")
			return nil
		},
	}
}

func buildSwarmSubcommands() []subcommandDefinition {
	return []subcommandDefinition{
		{
			Name:        "create",
			Aliases:     []string{"init"},
			Usage:       "/swarm create [name]",
			Description: "Create team",
			Handler: func(model *AppContext, args []string) tea.Cmd {
				name := strings.TrimSpace(strings.Join(args, " "))
				created, err := model.initTeam(name)
				if err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: swarm")
					return nil
				}
				model.appendUtilityMessage("Team initialized: "+created, "Utility: swarm")
				return nil
			},
		},
		{
			Name:         "use",
			Aliases:      []string{"activate"},
			Usage:        "/swarm use [name]",
			Description:  "Set active team",
			MinArgs:      1,
			ResourceArgs: []string{"team"},
			Handler: func(model *AppContext, args []string) tea.Cmd {
				name := strings.TrimSpace(strings.Join(args, " "))
				if err := model.useTeam(name); err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: swarm")
					return nil
				}
				model.appendUtilityMessage("Switched to team: "+name, "Utility: swarm")
				return nil
			},
		},
		{
			Name:        "list",
			Usage:       "/swarm list",
			Description: "List teams",
			Handler: func(model *AppContext, _ []string) tea.Cmd {
				teamList := strings.Join(model.teamNames(), ", ")
				if strings.TrimSpace(teamList) == "" {
					teamList = "(none)"
				}
				model.appendUtilityMessage("Teams: "+teamList, "Utility: swarm")
				return nil
			},
		},
		{
			Name:         "get",
			Usage:        "/swarm get [name]",
			Description:  "Get team details",
			MinArgs:      1,
			ResourceArgs: []string{"team"},
			Handler: func(model *AppContext, args []string) tea.Cmd {
				text, err := model.getTeam(strings.Join(args, " "))
				if err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: swarm")
					return nil
				}
				model.appendUtilityMessage(text, "Utility: swarm")
				return nil
			},
		},
		{
			Name:         "update",
			Usage:        "/swarm update [name] [new-name]",
			Description:  "Rename team",
			MinArgs:      2,
			ResourceArgs: []string{"team"},
			Handler: func(model *AppContext, args []string) tea.Cmd {
				oldName := strings.TrimSpace(args[0])
				newName := strings.TrimSpace(strings.Join(args[1:], " "))
				if err := model.updateTeam(oldName, newName); err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: swarm")
					return nil
				}
				model.appendUtilityMessage("Team updated: "+oldName+" -> "+newName, "Utility: swarm")
				return nil
			},
		},
		{
			Name:         "delete",
			Aliases:      []string{"remove"},
			Usage:        "/swarm delete [name]",
			Description:  "Delete team",
			MinArgs:      1,
			ResourceArgs: []string{"team"},
			Handler: func(model *AppContext, args []string) tea.Cmd {
				name := strings.TrimSpace(strings.Join(args, " "))
				if err := model.deleteTeam(name); err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: swarm")
					return nil
				}
				model.appendUtilityMessage("Team deleted: "+name, "Utility: swarm")
				return nil
			},
		},
	}
}

func buildAgentSubcommands() []subcommandDefinition {
	return []subcommandDefinition{
		{
			Name:         "use",
			Usage:        "/agent use [name]",
			Description:  "Set active agent",
			MinArgs:      1,
			ResourceArgs: []string{"agent"},
			Handler: func(model *AppContext, args []string) tea.Cmd {
				name := strings.TrimSpace(strings.Join(args, " "))
				if err := model.useAgent(name); err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: agent")
					return nil
				}
				model.appendUtilityMessage("Using agent: "+name, "Utility: agent")
				return nil
			},
		},
		{
			Name:        "list",
			Usage:       "/agent list",
			Description: "List agents",
			Handler: func(model *AppContext, _ []string) tea.Cmd {
				agentList := strings.Join(model.listAgentNames(), ", ")
				if strings.TrimSpace(agentList) == "" {
					agentList = "(none)"
				}
				model.appendUtilityMessage("Agents: "+agentList, "Utility: agent")
				return nil
			},
		},
		{
			Name:        "create",
			Usage:       "/agent create [name] [--connector name]",
			Description: "Create agent",
			MinArgs:     1,
			Handler: func(model *AppContext, args []string) tea.Cmd {
				name := ""
				connector := ""
				for i := 0; i < len(args); i++ {
					if args[i] == "--connector" && i+1 < len(args) {
						connector = args[i+1]
						i++
					} else if name == "" && !strings.HasPrefix(args[i], "--") {
						name = args[i]
					}
				}
				if name == "" {
					model.appendUtilityMessage("Agent name is required", "Utility: agent")
					return nil
				}
				if err := model.createAgent(name, connector); err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: agent")
					return nil
				}
				model.appendUtilityMessage("Agent created: "+name, "Utility: agent")
				return nil
			},
		},
		{
			Name:         "get",
			Usage:        "/agent get [name]",
			Description:  "Get agent details",
			MinArgs:      1,
			ResourceArgs: []string{"agent"},
			Handler: func(model *AppContext, args []string) tea.Cmd {
				text, err := model.getAgent(strings.Join(args, " "))
				if err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: agent")
					return nil
				}
				model.appendUtilityMessage(text, "Utility: agent")
				return nil
			},
		},
		{
			Name:         "update",
			Usage:        "/agent update [name] [system-prompt]",
			Description:  "Update agent system prompt",
			MinArgs:      2,
			ResourceArgs: []string{"agent"},
			Handler: func(model *AppContext, args []string) tea.Cmd {
				name := strings.TrimSpace(args[0])
				prompt := strings.TrimSpace(strings.Join(args[1:], " "))
				if err := model.updateAgent(name, prompt); err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: agent")
					return nil
				}
				model.appendUtilityMessage("Agent updated: "+name, "Utility: agent")
				return nil
			},
		},
		{
			Name:         "delete",
			Aliases:      []string{"remove"},
			Usage:        "/agent delete [name]",
			Description:  "Delete agent",
			MinArgs:      1,
			ResourceArgs: []string{"agent"},
			Handler: func(model *AppContext, args []string) tea.Cmd {
				name := strings.TrimSpace(strings.Join(args, " "))
				if err := model.deleteAgent(name); err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: agent")
					return nil
				}
				model.appendUtilityMessage("Agent deleted: "+name, "Utility: agent")
				return nil
			},
		},
		{
			Name:         "default",
			Usage:        "/agent default [name]",
			Description:  "Set global default agent",
			MinArgs:      1,
			ResourceArgs: []string{"agent"},
			Handler: func(model *AppContext, args []string) tea.Cmd {
				name := strings.TrimSpace(strings.Join(args, " "))
				if err := model.setDefaultAgent(name); err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: agent")
					return nil
				}
				model.appendUtilityMessage("Default agent set: "+name, "Utility: agent")
				return nil
			},
		},
	}
}

func dispatchSubcommand(parent string, model *AppContext, args []string, catalog subcommandCatalog) (tea.Cmd, bool) {
	if len(args) == 0 {
		return nil, false
	}
	definition, found := catalog.resolve(parent, args[0])
	if !found {
		return nil, false
	}

	subArgs := []string{}
	if len(args) > 1 {
		subArgs = args[1:]
	}
	if len(subArgs) < definition.MinArgs {
		model.appendUtilityMessage("Usage: "+definition.Usage, "Utility: "+parent)
		return nil, true
	}

	return definition.Handler(model, subArgs), true
}

func helpCommand() commandDefinition {
	return commandDefinition{
		Name:        "help",
		Description: "Show available slash commands",
		Handler: func(model *AppContext, _ []string) tea.Cmd {
			model.appendUtilityMessage("Commands: "+model.commands.helpList(), "Utility: help")
			return nil
		},
	}
}

func clearCommand() commandDefinition {
	return commandDefinition{
		Name:        "clear",
		Aliases:     []string{"reset"},
		Description: "Clear conversation history",
		Handler: func(model *AppContext, _ []string) tea.Cmd {
			model.resetConversation()
			model.statusText = "Conversation cleared"
			model.refreshViewport()
			return nil
		},
	}
}

func systemCommand() commandDefinition {
	return commandDefinition{
		Name:        "system",
		Description: "Show active agent system prompt",
		Handler: func(model *AppContext, _ []string) tea.Cmd {
			agentSpec, found := model.resolveActiveAgent()
			if !found {
				model.appendUtilityMessage("No active agent selected", "Utility: system")
				return nil
			}
			systemPrompt := strings.TrimSpace(agentSpec.SystemPrompt)
			if systemPrompt == "" {
				systemPrompt = "(empty)"
			}
			model.appendUtilityMessage("System prompt ("+agentSpec.Name+"): "+systemPrompt, "Utility: system")
			return nil
		},
	}
}

func settingCommand() commandDefinition {
	return commandDefinition{
		Name:        "setting",
		Aliases:     []string{"settings"},
		Description: "Open settings page to edit system prompt",
		Handler: func(model *AppContext, _ []string) tea.Cmd {
			model.openSettings()
			return nil
		},
	}
}

func quitCommand() commandDefinition {
	return commandDefinition{
		Name:        "quit",
		Aliases:     []string{"exit"},
		Description: "Quit context UI",
		Handler: func(_ *AppContext, _ []string) tea.Cmd {
			return tea.Quit
		},
	}
}

func (registry commandRegistry) execute(raw string, model *AppContext) (tea.Cmd, bool) {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) == 0 {
		return nil, false
	}

	name := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	args := []string{}
	if len(parts) > 1 {
		args = parts[1:]
	}

	definition, found := registry.lookup[name]
	if !found {
		model.appendUtilityMessage("Unknown command: /"+name+" (try /help)", "Utility: unknown command")
		return nil, true
	}

	return definition.Handler(model, args), true
}

func (registry commandRegistry) helpList() string {
	items := make([]string, 0, len(registry.definitions)+2)
	for _, definition := range registry.definitions {
		items = append(items, fmt.Sprintf("/%s", definition.Name))
		for _, alias := range definition.Aliases {
			items = append(items, fmt.Sprintf("/%s", alias))
		}
	}
	return strings.Join(items, ", ")
}

func (registry commandRegistry) helperLine(rawInput string) string {
	trimmed := strings.TrimSpace(rawInput)
	if !strings.HasPrefix(trimmed, "/") {
		return ""
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return ""
	}

	commandToken := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	if commandToken == "" {
		return "Helper: type command, use Tab to complete"
	}

	matches := registry.matchingDefinitions(commandToken)
	if len(matches) == 0 {
		return fmt.Sprintf("Helper: unknown /%s (use /help)", commandToken)
	}

	if len(parts) > 1 {
		subToken := strings.ToLower(strings.TrimSpace(parts[1]))
		if subToken == "" {
			return "Helper: choose a subcommand from suggestions"
		}
		return fmt.Sprintf("Helper: subcommand '%s' • Enter to run", subToken)
	}

	if len(matches) == 1 {
		return fmt.Sprintf("Helper: /%s • choose subcommand below", matches[0].Name)
	}

	return fmt.Sprintf("Helper: %d command matches • Tab to autocomplete", len(matches))
}

func (registry commandRegistry) helperSuggestions(rawInput string, provider resourceNameProvider) []commandSuggestion {
	trimmed := strings.TrimSpace(rawInput)
	if !strings.HasPrefix(trimmed, "/") {
		return nil
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return []commandSuggestion{{Title: "Commands", Usage: registry.helpList()}}
	}

	commandToken := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	if commandToken == "" {
		return []commandSuggestion{{Title: "Commands", Usage: registry.helpList()}}
	}

	matches := registry.matchingDefinitions(commandToken)
	if len(parts) == 1 {
		items := make([]commandSuggestion, 0, len(matches))
		for _, definition := range matches {
			items = append(items, commandSuggestion{Title: "/" + definition.Name, Usage: definition.Description})
		}
		if len(items) == 0 {
			return []commandSuggestion{{Title: "Unknown", Usage: "Use /help"}}
		}
		return items
	}

	if commandToken == "setting" || commandToken == "settings" {
		return []commandSuggestion{{Title: "Open", Usage: "/setting"}}
	}

	definitions := registry.subcommands.get(commandToken)
	if len(definitions) == 0 {
		items := make([]commandSuggestion, 0, len(matches))
		for _, definition := range matches {
			items = append(items, commandSuggestion{Title: "/" + definition.Name, Usage: definition.Description})
		}
		return items
	}

	hasTrailingSpace := strings.HasSuffix(rawInput, " ")
	if len(parts) == 2 && !hasTrailingSpace {
		return filterSubcommandSuggestions(definitions, parts[1])
	}

	if len(parts) >= 2 {
		subToken := strings.ToLower(strings.TrimSpace(parts[1]))
		if subDefinition, found := findSubcommand(definitions, subToken); found {
			if suggestions := registry.resourceArgSuggestions(*subDefinition, parts, hasTrailingSpace, provider); len(suggestions) > 0 {
				return suggestions
			}
		}
	}

	return subcommandSuggestions(definitions)
}

func subcommandSuggestions(definitions []subcommandDefinition) []commandSuggestion {
	items := make([]commandSuggestion, 0, len(definitions))
	seen := map[string]struct{}{}
	for _, definition := range definitions {
		if _, exists := seen[definition.Name]; exists {
			continue
		}
		seen[definition.Name] = struct{}{}
		items = append(items, commandSuggestion{
			Title: definition.Name,
			Usage: definition.Usage,
		})
	}
	return items
}

func filterSubcommandSuggestions(definitions []subcommandDefinition, prefix string) []commandSuggestion {
	normalized := strings.ToLower(strings.TrimSpace(prefix))
	items := make([]commandSuggestion, 0, len(definitions))
	seen := map[string]struct{}{}
	for _, definition := range definitions {
		match := strings.HasPrefix(definition.Name, normalized)
		if !match {
			for _, alias := range definition.Aliases {
				if strings.HasPrefix(alias, normalized) {
					match = true
					break
				}
			}
		}
		if !match {
			continue
		}
		if _, exists := seen[definition.Name]; exists {
			continue
		}
		seen[definition.Name] = struct{}{}
		items = append(items, commandSuggestion{Title: definition.Name, Usage: definition.Usage})
	}
	if len(items) == 0 {
		return subcommandSuggestions(definitions)
	}
	return items
}

func findSubcommand(definitions []subcommandDefinition, token string) (*subcommandDefinition, bool) {
	normalized := strings.ToLower(strings.TrimSpace(token))
	for _, definition := range definitions {
		if definition.Name == normalized {
			item := definition
			return &item, true
		}
		for _, alias := range definition.Aliases {
			if alias == normalized {
				item := definition
				return &item, true
			}
		}
	}
	return nil, false
}

func (registry commandRegistry) resourceArgSuggestions(definition subcommandDefinition, parts []string, hasTrailingSpace bool, provider resourceNameProvider) []commandSuggestion {
	if provider == nil || len(definition.ResourceArgs) == 0 {
		return nil
	}

	typedArgCount := len(parts) - 2
	if typedArgCount < 0 {
		typedArgCount = 0
	}
	currentArgIndex := typedArgCount - 1
	currentPrefix := ""
	if hasTrailingSpace {
		currentArgIndex = typedArgCount
	} else if len(parts) > 2 {
		currentPrefix = strings.TrimSpace(parts[len(parts)-1])
	}

	if currentArgIndex < 0 || currentArgIndex >= len(definition.ResourceArgs) {
		return nil
	}

	kind := strings.TrimSpace(definition.ResourceArgs[currentArgIndex])
	if kind == "" {
		return nil
	}

	names := provider.resourceNames(kind)
	if len(names) == 0 {
		return nil
	}

	prefix := strings.ToLower(strings.TrimSpace(currentPrefix))
	items := make([]commandSuggestion, 0, len(names))
	for _, name := range names {
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), prefix) {
			continue
		}
		items = append(items, commandSuggestion{Title: name, Usage: kind + " name"})
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func (registry commandRegistry) matchingDefinitions(prefix string) []commandDefinition {
	matched := make([]commandDefinition, 0, len(registry.definitions))
	for _, definition := range registry.definitions {
		if strings.HasPrefix(definition.Name, prefix) {
			matched = append(matched, definition)
			continue
		}

		for _, alias := range definition.Aliases {
			if strings.HasPrefix(alias, prefix) {
				matched = append(matched, definition)
				break
			}
		}
	}

	return matched
}

func (registry commandRegistry) completeSlashInput(rawInput string) (string, string, bool) {
	raw := strings.TrimSpace(rawInput)
	if !strings.HasPrefix(raw, "/") {
		return rawInput, "", false
	}

	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return rawInput, "", false
	}

	commandToken := strings.ToLower(strings.TrimPrefix(parts[0], "/"))
	if commandToken == "" {
		return rawInput, "Tab: type a command prefix, e.g. /he", false
	}

	matches := registry.matchingDefinitions(commandToken)
	if len(matches) == 0 {
		return rawInput, "Tab: no command matches /" + commandToken, false
	}

	if len(matches) == 1 {
		completed := "/" + matches[0].Name
		rest := ""
		if len(parts) > 1 {
			rest = " " + strings.Join(parts[1:], " ")
		} else {
			rest = " "
		}
		return completed + rest, "Tab: completed /" + matches[0].Name, true
	}

	prefix := longestCommonPrefix(matches)
	if prefix != "" && len(prefix) > len(commandToken) {
		rest := ""
		if len(parts) > 1 {
			rest = " " + strings.Join(parts[1:], " ")
		}
		return "/" + prefix + rest, "Tab: multiple matches, narrowed prefix", true
	}

	items := make([]string, 0, len(matches))
	for _, match := range matches {
		items = append(items, "/"+match.Name)
	}
	return rawInput, "Tab matches: " + strings.Join(items, ", "), false
}

func longestCommonPrefix(definitions []commandDefinition) string {
	if len(definitions) == 0 {
		return ""
	}

	prefix := definitions[0].Name
	for _, definition := range definitions[1:] {
		for !strings.HasPrefix(definition.Name, prefix) {
			if len(prefix) <= 1 {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}

	return prefix
}

func (m AppContext) executeSlashCommand(raw string) (AppContext, tea.Cmd, bool) {
	cmd, handled := m.commands.execute(raw, &m)
	return m, cmd, handled
}
func connectorCommand(catalog subcommandCatalog) commandDefinition {
	return commandDefinition{
		Name:        "connector",
		Description: "Connector CRUD: /connector create|delete|list (providers: gemini-ai|gemini-vertex|openai-compatible)",
		Handler: func(model *AppContext, args []string) tea.Cmd {
			if len(args) == 0 {
				connectors, err := model.listConnectors()
				if err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: connector")
					return nil
				}
				var names []string
				for _, c := range connectors {
					names = append(names, c.Name)
				}
				list := strings.Join(names, ", ")
				if list == "" {
					list = "(none)"
				}
				model.appendUtilityMessage("Connectors: "+list, "Utility: connector")
				return nil
			}

			if cmd, handled := dispatchSubcommand("connector", model, args, catalog); handled {
				return cmd
			}

			model.appendUtilityMessage("Usage: /connector create|delete|list", "Utility: connector")
			return nil
		},
	}
}

func buildConnectorSubcommands() []subcommandDefinition {
	return []subcommandDefinition{
		{
			Name:        "create",
			Usage:       "/connector create [name] --provider [gemini-ai|gemini-vertex|openai-compatible] --api-key [key] --base-url [url]",
			Description: "Create connector",
			Handler: func(model *AppContext, args []string) tea.Cmd {
				name := ""
				provider := domain.ConnectorProviderOpenAICompat
				apiKey := ""
				baseURL := ""

				for i := 0; i < len(args); i++ {
					if args[i] == "--provider" && i+1 < len(args) {
						provider = args[i+1]
						i++
					} else if args[i] == "--api-key" && i+1 < len(args) {
						apiKey = args[i+1]
						i++
					} else if args[i] == "--base-url" && i+1 < len(args) {
						baseURL = args[i+1]
						i++
					} else if name == "" && !strings.HasPrefix(args[i], "--") {
						name = args[i]
					}
				}

				if name == "" {
					model.appendUtilityMessage("Connector name is required", "Utility: connector")
					return nil
				}

				err := model.saveConnector(domain.Connector{
					Name:     name,
					Provider: provider,
					APIKey:   apiKey,
					BaseURL:  baseURL,
				})
				if err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: connector")
					return nil
				}
				model.appendUtilityMessage("Connector created: "+name, "Utility: connector")
				return nil
			},
		},
		{
			Name:        "delete",
			Usage:       "/connector delete [name]",
			Description: "Delete connector",
			Handler: func(model *AppContext, args []string) tea.Cmd {
				name := strings.TrimSpace(strings.Join(args, " "))
				if name == "" {
					model.appendUtilityMessage("Connector name is required", "Utility: connector")
					return nil
				}
				err := model.deleteConnector(name)
				if err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: connector")
					return nil
				}
				model.appendUtilityMessage("Connector deleted: "+name, "Utility: connector")
				return nil
			},
		},
		{
			Name:        "list",
			Usage:       "/connector list",
			Description: "List connectors",
			Handler: func(model *AppContext, args []string) tea.Cmd {
				connectors, err := model.listConnectors()
				if err != nil {
					model.appendUtilityMessage(err.Error(), "Utility: connector")
					return nil
				}
				if len(connectors) == 0 {
					model.appendUtilityMessage("No connectors found", "Utility: connector")
					return nil
				}
				var lines []string
				for _, c := range connectors {
					lines = append(lines, fmt.Sprintf("- %s (provider: %s)", c.Name, c.Provider))
				}
				model.appendUtilityMessage(strings.Join(lines, "\n"), "Utility: connector")
				return nil
			},
		},
	}
}

func memberCommand(catalog subcommandCatalog) commandDefinition {
	return commandDefinition{
		Name:        "member",
		Description: "Team member management: /member add|remove|list|role",
		Handler: func(model *AppContext, args []string) tea.Cmd {
			if len(args) == 0 {
				team := model.getActiveTeam()
				if team == nil {
					model.appendUtilityMessage("No active team", "Utility: member")
					return nil
				}
				model.appendUtilityMessage("Active team: "+team.Name+"\nUse: /member add|remove|list|role", "Utility: member")
				return nil
			}

			if cmd, handled := dispatchSubcommand("member", model, args, catalog); handled {
				return cmd
			}

			model.appendUtilityMessage("Usage: /member add|remove|list|role", "Utility: member")
			return nil
		},
	}
}

func taskCommand(catalog subcommandCatalog) commandDefinition {
	return commandDefinition{
		Name:        "task",
		Description: "Task dispatch and workflow: /task dispatch|status|approve",
		Handler: func(model *AppContext, args []string) tea.Cmd {
			if len(args) == 0 {
				model.appendUtilityMessage("Usage: /task dispatch|status|approve", "Utility: task")
				return nil
			}

			if cmd, handled := dispatchSubcommand("task", model, args, catalog); handled {
				return cmd
			}

			model.appendUtilityMessage("Usage: /task dispatch|status|approve", "Utility: task")
			return nil
		},
	}
}

func buildMemberSubcommands() []subcommandDefinition {
	return []subcommandDefinition{
		{
			Name:        "list",
			Usage:       "/member list",
			Description: "List team members and their roles",
			Handler: func(model *AppContext, _ []string) tea.Cmd {
				team := model.getActiveTeam()
				if team == nil {
					model.appendUtilityMessage("No active team", "Utility: member")
					return nil
				}
				model.appendUtilityMessage("Team: "+team.Name+"\nMembers: (coordination details not yet integrated)", "Utility: member")
				return nil
			},
		},
		{
			Name:        "add",
			Usage:       "/member add [agent-name] [role]",
			Description: "Add agent as team member with role",
			MinArgs:     2,
			Handler: func(model *AppContext, args []string) tea.Cmd {
				team := model.getActiveTeam()
				if team == nil {
					model.appendUtilityMessage("No active team", "Utility: member")
					return nil
				}
				agentName := strings.TrimSpace(args[0])
				role := strings.TrimSpace(strings.Join(args[1:], " "))
				model.appendUtilityMessage("Added "+agentName+" to team "+team.Name+" as "+role, "Utility: member")
				return nil
			},
		},
		{
			Name:        "remove",
			Usage:       "/member remove [agent-name]",
			Description: "Remove agent from team",
			MinArgs:     1,
			Handler: func(model *AppContext, args []string) tea.Cmd {
				team := model.getActiveTeam()
				if team == nil {
					model.appendUtilityMessage("No active team", "Utility: member")
					return nil
				}
				agentName := strings.TrimSpace(strings.Join(args, " "))
				model.appendUtilityMessage("Removed "+agentName+" from team "+team.Name, "Utility: member")
				return nil
			},
		},
		{
			Name:        "role",
			Usage:       "/member role [agent-name]",
			Description: "Get or set agent role",
			MinArgs:     1,
			Handler: func(model *AppContext, args []string) tea.Cmd {
				team := model.getActiveTeam()
				if team == nil {
					model.appendUtilityMessage("No active team", "Utility: member")
					return nil
				}
				agentName := strings.TrimSpace(strings.Join(args, " "))
				model.appendUtilityMessage("Role for "+agentName+": (not yet queried)", "Utility: member")
				return nil
			},
		},
	}
}

func buildTaskSubcommands() []subcommandDefinition {
	return []subcommandDefinition{
		{
			Name:        "dispatch",
			Usage:       "/task dispatch [title] [context...]",
			Description: "Dispatch task to team",
			MinArgs:     1,
			Handler: func(model *AppContext, args []string) tea.Cmd {
				team := model.getActiveTeam()
				if team == nil {
					model.appendUtilityMessage("No active team", "Utility: task")
					return nil
				}
				title := strings.TrimSpace(args[0])
				context := ""
				if len(args) > 1 {
					context = strings.TrimSpace(strings.Join(args[1:], " "))
				}
				model.appendUtilityMessage("Dispatched task '"+title+"' to team "+team.Name, "Utility: task")
				if context != "" {
					model.appendUtilityMessage("Context: "+context, "Utility: task")
				}
				return nil
			},
		},
		{
			Name:        "status",
			Usage:       "/task status [task-id]",
			Description: "Check task status",
			Handler: func(model *AppContext, args []string) tea.Cmd {
				if len(args) == 0 {
					model.appendUtilityMessage("Usage: /task status [task-id]", "Utility: task")
					return nil
				}
				taskID := strings.TrimSpace(strings.Join(args, " "))
				model.appendUtilityMessage("Task "+taskID+" status: (not yet implemented)", "Utility: task")
				return nil
			},
		},
		{
			Name:        "approve",
			Usage:       "/task approve [task-id]",
			Description: "Approve pending task",
			Handler: func(model *AppContext, args []string) tea.Cmd {
				if len(args) == 0 {
					model.appendUtilityMessage("Usage: /task approve [task-id]", "Utility: task")
					return nil
				}
				taskID := strings.TrimSpace(strings.Join(args, " "))
				model.appendUtilityMessage("Approved task "+taskID, "Utility: task")
				return nil
			},
		},
	}
}
