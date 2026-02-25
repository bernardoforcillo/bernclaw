package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/bernardoforcillo/bernclaw/internal/adapter/genai"
	"github.com/bernardoforcillo/bernclaw/internal/adapter/openaicompat"
	"github.com/bernardoforcillo/bernclaw/internal/app"
	"github.com/bernardoforcillo/bernclaw/internal/config"
	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorTerracotta).
			MarginBottom(0).
			PaddingLeft(1)

	headerMetaStyle = lipgloss.NewStyle().
			Foreground(colorMidGray).
			PaddingLeft(1)

	chipStyle = lipgloss.NewStyle().
			Foreground(colorLightGray).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSubtleGray).
			Padding(0, 1).
			MarginLeft(1)

	statusStyle = lipgloss.NewStyle().
			Foreground(colorMidGray).
			PaddingLeft(1)

	settingsHintStyle = lipgloss.NewStyle().
				Foreground(colorMidGray).
				PaddingLeft(1)

	helperStyle = lipgloss.NewStyle().
			Foreground(colorMidGray).
			PaddingLeft(1)

	tooltipPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorSubtleGray).
				Padding(0, 1).
				MarginLeft(1)

	tooltipTitleStyle = lipgloss.NewStyle().
				Foreground(colorTerracotta).
				Bold(true)

	tooltipItemStyle = lipgloss.NewStyle().
				Foreground(colorLightGray)

	tooltipSelectedItemStyle = lipgloss.NewStyle().
					Foreground(colorWhite).
					Background(colorDarkGray).
					Bold(true)

	transcriptPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorSubtleGray).
				Padding(0, 1)

	promptPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorSubtleGray).
				Padding(0, 1)

	systemRoleStyle    = lipgloss.NewStyle().Foreground(colorMidGray).Bold(true)
	userRoleStyle      = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	assistantRoleStyle = lipgloss.NewStyle().Foreground(colorTerracotta).Bold(true)
	utilityRoleStyle   = lipgloss.NewStyle().Foreground(colorBlue).Bold(true)
	bodyStyle          = lipgloss.NewStyle().Foreground(colorLightGray)
	thinkStyle         = lipgloss.NewStyle().Foreground(colorMidGray).Italic(true)
)

type assistantReplyMsg struct {
	text string
	err  error
}

const (
	defaultStatusText     = "Enter: send • ↑/↓: history • Ctrl+R: search • Tab: complete command • Ctrl+C: quit"
	maxVisibleSuggestions = 6
)

type AppContext struct {
	cfg             config.Config
	commands        commandRegistry
	agents          agentRegistryState
	chat            *app.ChatService
	commandHistory  commandHistoryState
	history         []domain.Message
	viewport        viewport.Model
	input           textarea.Model
	settingsInput   textarea.Model
	width           int
	height          int
	isReady         bool
	isSending       bool
	inSettings      bool
	statusText      string
	suggestions     suggestionState
	suggestionArmed bool
}

func RunChatUI(cfg config.Config) error {
	program := tea.NewProgram(newAppContext(cfg), tea.WithAltScreen())
	_, err := program.Run()
	return err
}

func newAppContext(cfg config.Config) AppContext {
	input := textarea.New()
	input.Placeholder = "Type message or /command (Tab to complete)"
	input.Prompt = lipgloss.NewStyle().Foreground(colorTerracotta).Render("› ")
	input.CharLimit = 0
	input.SetHeight(2)
	input.Focus()
	input.ShowLineNumbers = false

	settingsInput := textarea.New()
	settingsInput.Placeholder = "System prompt"
	settingsInput.CharLimit = 0
	settingsInput.SetHeight(10)
	settingsInput.ShowLineNumbers = false

	history := make([]domain.Message, 0, 32)

	status := defaultStatusText
	workspace := app.DefaultWorkspace()

	// Factory selects the right LLM adapter based on the connector's provider.
	factory := buildLLMClientFactory()

	model := AppContext{
		cfg:             cfg,
		commands:        newCommandRegistry(),
		agents:          newAgentRegistryState(workspace.Agents, workspace.Connectors, "gpt-4o"),
		chat:            app.NewChatService(workspace.Connectors, factory),
		commandHistory:  newCommandHistoryState(filepath.Join(".bernclaw", ".commands_history")),
		history:         history,
		input:           input,
		settingsInput:   settingsInput,
		statusText:      status,
		suggestions:     newSuggestionState(maxVisibleSuggestions),
		suggestionArmed: false,
	}

	if err := model.agents.loadStoredResources(); err != nil {
		model.statusText = "Resource load warning: " + err.Error()
	}
	if strings.TrimSpace(model.agents.activeAgent) == "" {
		model.statusText = "No default agent set • use /agent create [name]"
	}
	if err := model.commandHistory.loadFromDisk(); err != nil {
		model.statusText = "History load warning: " + err.Error()
	}

	return model
}

func (m AppContext) Init() tea.Cmd {
	return textarea.Blink
}

func (m AppContext) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		return m.handleWindowSize(typed), nil

	case assistantReplyMsg:
		return m.handleAssistantReply(typed), nil

	case tea.KeyMsg:
		if m.inSettings {
			return m.handleSettingsKey(typed)
		}

		return m.handleMainKey(typed)
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.viewport, _ = m.viewport.Update(msg)
	return m, cmd
}

func (m AppContext) View() string {
	if !m.isReady {
		return "Initializing context UI..."
	}
	if m.inSettings {
		return m.renderSettingsView()
	}

	header := headerStyle.Render("bernclaw")
	teamLabel := "none"
	if team := m.getActiveTeam(); team != nil && strings.TrimSpace(team.Name) != "" {
		teamLabel = team.Name
	}
	agentLabel := m.activeAgentName()
	if strings.TrimSpace(agentLabel) == "" {
		agentLabel = "none"
	}
	modelLabel := m.activeModelLabel()
	headerMeta := lipgloss.JoinHorizontal(
		lipgloss.Left,
		chipStyle.Render("team: "+teamLabel),
		chipStyle.Render("model: "+modelLabel),
		chipStyle.Render("agent: "+agentLabel),
		headerMetaStyle.Render("/help"),
	)
	status := statusStyle.Italic(true).Render(m.statusText)
	helperLine := m.commands.helperLine(m.input.Value())
	helperSuggestions := m.currentSuggestions()
	showTooltip := len(helperSuggestions) > 0
	showHelper := !showTooltip
	helper := statusStyle.Render("Type message and press Enter • Use /help for commands")
	if showHelper && helperLine != "" {
		helper = helperStyle.Render(helperLine)
	}
	tooltip := ""
	if showTooltip {
		visibleSuggestions, start := m.visibleSuggestions(helperSuggestions)
		rows := make([]string, 0, len(visibleSuggestions)+2)
		rows = append(rows, tooltipTitleStyle.Render("Suggestions"))
		for rowIndex, suggestion := range visibleSuggestions {
			absoluteIndex := start + rowIndex
			line := "• " + suggestion.Title + "  " + suggestion.Usage
			if absoluteIndex == m.suggestions.index {
				rows = append(rows, tooltipSelectedItemStyle.Render(line))
				continue
			}
			rows = append(rows, tooltipItemStyle.Render(line))
		}
		if len(helperSuggestions) > len(visibleSuggestions) {
			rows = append(rows, helperStyle.Render(fmt.Sprintf("%d/%d", m.suggestions.index+1, len(helperSuggestions))))
		}
		tooltip = tooltipPanelStyle.Render(strings.Join(rows, "\n"))
	}

	transcriptPanel := transcriptPanelStyle.
		Width(max(30, m.width-2)).
		Render(m.viewport.View())
	promptSections := []string{}
	if showHelper {
		promptSections = append(promptSections, helper)
	}
	if showTooltip {
		promptSections = append(promptSections, tooltip)
	}
	promptSections = append(promptSections, m.input.View())

	promptPanel := promptPanelStyle.
		Width(max(30, m.width-2)).
		Render(lipgloss.JoinVertical(lipgloss.Left, promptSections...))

	sections := []string{
		strings.Join([]string{header, headerMeta}, "\n"),
		transcriptPanel,
		promptPanel,
		status,
	}

	return strings.Join(sections, "\n")
}

func (m AppContext) renderSettingsView() string {
	header := headerStyle.Render("bernclaw settings")
	meta := headerMetaStyle.Render("Agent system prompt editor")
	hint := settingsHintStyle.Render("Ctrl+S save • Esc cancel\nVars: {{Date}} | {{Date:+1}} | {{Now}} | {{System}}")

	content := promptPanelStyle.
		Width(max(30, m.width-2)).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			headerMetaStyle.Render("System Prompt"),
			hint,
			m.settingsInput.View(),
		))

	return strings.Join([]string{header, meta, content}, "\n")
}

func (m AppContext) handleWindowSize(msg tea.WindowSizeMsg) AppContext {
	m.width = msg.Width
	m.height = msg.Height
	tooltipCount := len(m.currentSuggestions())
	if tooltipCount > m.suggestions.maxVisible {
		tooltipCount = m.suggestions.maxVisible
	}
	helperVisible := tooltipCount == 0 && m.commands.helperLine(m.input.Value()) != ""
	inputPanelHeight := promptPanelHeight(helperVisible, tooltipCount)

	if !m.isReady {
		m.viewport = viewport.New(msg.Width-2, max(5, msg.Height-inputPanelHeight-3))
		m.isReady = true
	} else {
		m.viewport.Width = msg.Width - 2
		m.viewport.Height = max(5, msg.Height-inputPanelHeight-3)
	}

	m.input.SetWidth(max(20, msg.Width-6))
	m.refreshViewport()
	return m
}

func (m AppContext) handleAssistantReply(msg assistantReplyMsg) AppContext {
	m.isSending = false
	if msg.err != nil {
		m.statusText = msg.err.Error()
		m.refreshViewport()
		return m
	}

	m.history = append(m.history, domain.Message{Role: "assistant", Content: msg.text})
	m.statusText = "Response received • Enter: send • Ctrl+C: quit"
	m.refreshViewport()
	return m
}

func (m AppContext) handleSettingsKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.closeSettings("Settings canceled")
		return m, nil
	case "ctrl+s":
		newPrompt := strings.TrimSpace(m.settingsInput.Value())
		if strings.TrimSpace(m.activeAgentName()) == "" {
			m.closeSettings("No active agent selected")
			return m, nil
		}
		if err := m.updateAgent(m.activeAgentName(), newPrompt); err != nil {
			m.closeSettings("Save failed: " + err.Error())
			return m, nil
		}
		m.closeSettings("Agent prompt updated")
		m.refreshViewport()
		return m, nil
	}

	var settingsCmd tea.Cmd
	m.settingsInput, settingsCmd = m.settingsInput.Update(key)
	return m, settingsCmd
}

func (m AppContext) handleMainKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		completed, status, changed := m.commands.completeSlashInput(m.input.Value())
		if status != "" {
			m.statusText = status
		}
		if changed {
			m.input.SetValue(completed)
		}
		m.suggestionArmed = false
		return m, nil
	case "up":
		if m.moveSuggestionSelection(-1) {
			m.suggestionArmed = true
			return m, nil
		}
		m.moveHistoryUp()
		m.suggestionArmed = false
		return m, nil
	case "down":
		if m.moveSuggestionSelection(1) {
			m.suggestionArmed = true
			return m, nil
		}
		m.moveHistoryDown()
		m.suggestionArmed = false
		return m, nil
	case "ctrl+r":
		if hit := m.reverseSearchHistory(); hit {
			m.statusText = "History search hit"
		} else {
			m.statusText = "History search: no match"
		}
		return m, nil
	case "enter":
		if m.isSending {
			return m, nil
		}

		if m.applySelectedSuggestion() {
			m.suggestionArmed = false
			return m, nil
		}

		text := strings.TrimSpace(m.input.Value())
		if text == "" {
			return m, nil
		}
		m.rememberInput(text)
		m.suggestionArmed = false

		if strings.HasPrefix(text, "/") {
			m.input.Reset()
			updated, cmd, handled := m.executeSlashCommand(text)
			if handled {
				return updated, cmd
			}
		}

		m.history = append(m.history, domain.Message{Role: "user", Content: text})
		m.input.Reset()
		m.isSending = true
		m.statusText = "Sending..."
		m.refreshViewport()

		modelName, modelErr := m.resolveActiveModelName()
		if modelErr != nil {
			m.isSending = false
			m.statusText = modelErr.Error()
			m.refreshViewport()
			return m, nil
		}

		connectorName, connectorErr := m.resolveActiveConnectorName()
		if connectorErr != nil {
			m.isSending = false
			m.statusText = connectorErr.Error()
			m.refreshViewport()
			return m, nil
		}

		historyCopy := buildAPIHistory(m.history)
		historyCopy = m.withAgentSystemPrompt(historyCopy)
		return m, requestAssistantReply(m.chat, connectorName, modelName, historyCopy)
	}

	var inputCmd tea.Cmd
	m.input, inputCmd = m.input.Update(key)
	m.normalizeSuggestionState()
	m.suggestionArmed = false
	return m, inputCmd
}

func (m *AppContext) closeSettings(status string) {
	m.inSettings = false
	m.settingsInput.Blur()
	m.input.Focus()
	m.statusText = status
}

func (m *AppContext) openSettings() {
	resolved, found := m.resolveActiveAgent()
	if !found {
		m.statusText = "Settings require an active agent (/agent use [name])"
		return
	}

	m.inSettings = true
	m.settingsInput.SetValue(resolved.SystemPrompt)
	m.settingsInput.Focus()
	m.input.Blur()
	m.statusText = "Settings mode"
}

func (m *AppContext) resetConversation() {
	m.history = make([]domain.Message, 0, 1)
}

func (m *AppContext) appendUtilityMessage(content string, status string) {
	m.history = append(m.history, domain.Message{Role: "utility", Content: content})
	m.statusText = status
	m.refreshViewport()
}

func (m *AppContext) refreshViewport() {
	m.viewport.SetContent(renderTranscript(m.history))
	m.viewport.GotoBottom()
}

// buildLLMClientFactory returns a port.LLMClientFactory that selects the
// correct adapter implementation based on the connector's provider field.
func buildLLMClientFactory() port.LLMClientFactory {
	return func(connector domain.Connector) (port.LLMClient, error) {
		switch strings.TrimSpace(connector.Provider) {
		case domain.ConnectorProviderOpenAICompat:
			return openaicompat.NewClient(openaicompat.Config{
				APIKey:  connector.APIKey,
				BaseURL: connector.BaseURL,
			})
		case domain.ConnectorProviderGeminiAI, domain.ConnectorProviderGeminiAIAlt:
			return genai.NewAIStudioClient(context.Background(), connector.APIKey, "")
		default:
			return nil, fmt.Errorf("provider %q is not supported yet", connector.Provider)
		}
	}
}

func requestAssistantReply(chat *app.ChatService, connectorName string, modelName string, history []domain.Message) tea.Cmd {
	return func() tea.Msg {
		reply, err := chat.Send(context.Background(), connectorName, modelName, history)
		if err != nil {
			return assistantReplyMsg{err: err}
		}
		return assistantReplyMsg{text: reply.Content}
	}
}

// resolveActiveConnectorName returns the connector name configured on the
// active agent.  The actual credential lookup is done by ChatService.Send.
func (m *AppContext) resolveActiveConnectorName() (string, error) {
	activeAgent, found := m.resolveActiveAgent()
	if !found {
		return "", fmt.Errorf("no active agent set • use /agent create [name]")
	}
	connectorName := strings.TrimSpace(activeAgent.Connector)
	if connectorName == "" {
		return "", fmt.Errorf("active agent has no connector • use /agent create [name] --connector [connector-name]")
	}
	return connectorName, nil
}

func buildAPIHistory(history []domain.Message) []domain.Message {
	filtered := make([]domain.Message, 0, len(history))
	for _, message := range history {
		switch message.Role {
		case "system", "user", "assistant":
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func renderTranscript(history []domain.Message) string {
	if len(history) == 0 {
		return lipgloss.NewStyle().Foreground(colorMidGray).Render("Bernclaw initialized. Ready to help.")
	}

	var out strings.Builder
	for _, message := range history {
		role := "Message"
		roleView := userRoleStyle.Render("You")
		switch message.Role {
		case "system":
			role = "System"
			roleView = systemRoleStyle.Render(role)
		case "user":
			role = "You"
			roleView = userRoleStyle.Render(role)
		case "assistant":
			role = "Assistant"
			roleView = assistantRoleStyle.Render(role)
		case "utility":
			role = "Utility"
			roleView = utilityRoleStyle.Render(role)
		default:
			roleView = userRoleStyle.Render(role)
		}
		out.WriteString(roleView)
		out.WriteString("\n")
		if message.Role == "assistant" {
			out.WriteString(renderAssistantContent(message.Content))
		} else {
			out.WriteString(bodyStyle.Render(message.Content))
		}
		out.WriteString("\n\n")
	}

	return strings.TrimSpace(out.String())
}

func renderAssistantContent(content string) string {
	if content == "" {
		return ""
	}

	const startTag = "<think>"
	const endTag = "</think>"

	remaining := content
	var out strings.Builder

	for len(remaining) > 0 {
		startIndex := strings.Index(remaining, startTag)
		if startIndex < 0 {
			out.WriteString(bodyStyle.Render(remaining))
			break
		}

		if startIndex > 0 {
			out.WriteString(bodyStyle.Render(remaining[:startIndex]))
		}

		thinkPayload := remaining[startIndex+len(startTag):]
		endIndex := strings.Index(thinkPayload, endTag)
		if endIndex < 0 {
			out.WriteString(thinkStyle.Render(thinkPayload))
			break
		}

		out.WriteString(thinkStyle.Render(thinkPayload[:endIndex]))
		remaining = thinkPayload[endIndex+len(endTag):]
	}

	return out.String()
}

func max(first int, second int) int {
	if first > second {
		return first
	}
	return second
}

func promptPanelHeight(helperVisible bool, tooltipCount int) int {
	base := 7
	if helperVisible {
		base++
	}
	if tooltipCount > 0 {
		base += tooltipCount + 2
	}
	return base
}

func (m AppContext) withAgentSystemPrompt(history []domain.Message) []domain.Message {
	now := time.Now()
	agentPrompt := ""

	agent, found := m.resolveActiveAgent()
	if found {
		agentPrompt = applyPromptVars(strings.TrimSpace(agent.SystemPrompt), now)
	}

	augmented := make([]domain.Message, 0, len(history)+1)
	if agentPrompt != "" {
		augmented = append(augmented, domain.Message{Role: "system", Content: agentPrompt})
	}
	augmented = append(augmented, history...)
	return augmented
}

func (m *AppContext) resolveActiveAgent() (domain.Spec, bool) {
	return m.agents.resolveActiveAgent()
}

func (m *AppContext) resolveActiveModelName() (string, error) {
	return m.agents.resolveActiveModelName()
}

func (m *AppContext) activeModelLabel() string {
	return m.agents.activeModelLabel()
}

func (m *AppContext) listAgentNames() []string {
	return m.agents.listAgentNames()
}

func (m *AppContext) resourceNames(kind string) []string {
	return m.agents.resourceNames(kind)
}

func (m *AppContext) getActiveTeam() *domain.Team {
	return m.agents.getActiveTeam()
}

func (m *AppContext) teamNames() []string {
	return m.agents.teamNames()
}

func (m *AppContext) initTeam(name string) (string, error) {
	return m.agents.initTeam(name)
}

func (m *AppContext) useTeam(name string) error {
	return m.agents.useTeam(name)
}

func (m *AppContext) createAgent(name string, connector string) error {
	return m.agents.createAgent(name, connector)
}

func (m *AppContext) useAgent(name string) error {
	return m.agents.useAgent(name)
}

func (m *AppContext) getTeam(name string) (string, error) {
	return m.agents.getTeam(name)
}

func (m *AppContext) updateTeam(currentName string, nextName string) error {
	return m.agents.updateTeam(currentName, nextName)
}

func (m *AppContext) deleteTeam(name string) error {
	return m.agents.deleteTeam(name)
}

func (m *AppContext) resolveAgent(name string) (resolvedAgentRef, bool) {
	return m.agents.resolveAgent(name)
}

func (m *AppContext) getAgent(name string) (string, error) {
	return m.agents.getAgent(name)
}

func (m *AppContext) updateAgent(name string, newSystemPrompt string) error {
	return m.agents.updateAgent(name, newSystemPrompt)
}

func (m *AppContext) setDefaultAgent(name string) error {
	return m.agents.setDefaultAgent(name)
}

func (m *AppContext) clearDefaultAgentFlags() error {
	return m.agents.clearDefaultAgentFlags()
}

func (m *AppContext) normalizeDefaultAgents(defaultTeam string, defaultAgent string) error {
	return m.agents.normalizeDefaultAgents(defaultTeam, defaultAgent)
}

func (m *AppContext) deleteAgent(name string) error {
	return m.agents.deleteAgent(name)
}

func (m *AppContext) saveConnector(value domain.Connector) error {
	return m.agents.connectors.SaveConnector(value)
}

func (m *AppContext) listConnectors() ([]domain.Connector, error) {
	return m.agents.connectors.ListConnectors()
}

func (m *AppContext) deleteConnector(name string) error {
	return m.agents.connectors.DeleteConnector(name)
}

func (m *AppContext) activeAgentName() string {
	return m.agents.activeAgent
}

func (m *AppContext) rememberInput(value string) {
	_ = m.commandHistory.remember(value)
}

func (m *AppContext) moveHistoryUp() {
	_ = m.commandHistory.loadFromDisk()
	if value, ok := m.commandHistory.moveUp(m.input.Value()); ok {
		m.input.SetValue(value)
	}
}

func (m *AppContext) moveHistoryDown() {
	_ = m.commandHistory.loadFromDisk()
	if value, ok := m.commandHistory.moveDown(); ok {
		m.input.SetValue(value)
	}
}

func (m *AppContext) reverseSearchHistory() bool {
	_ = m.commandHistory.loadFromDisk()
	if value, ok := m.commandHistory.reverseSearch(m.input.Value(), m.input.Value()); ok {
		m.input.SetValue(value)
		return true
	}
	return false
}

func (m *AppContext) currentSuggestions() []commandSuggestion {
	return m.commands.helperSuggestions(m.input.Value(), m)
}

func (m *AppContext) normalizeSuggestionState() {
	suggestions := m.currentSuggestions()
	m.suggestions.normalize(len(suggestions))
}

func (m *AppContext) visibleSuggestions(suggestions []commandSuggestion) ([]commandSuggestion, int) {
	if len(suggestions) == 0 {
		return nil, 0
	}
	start, end := m.suggestions.visibleRange(len(suggestions))
	return suggestions[start:end], start
}

func (m *AppContext) moveSuggestionSelection(step int) bool {
	suggestions := m.currentSuggestions()
	if len(suggestions) == 0 {
		return false
	}
	return m.suggestions.move(step, len(suggestions))
}

func (m *AppContext) applySelectedSuggestion() bool {
	if !m.suggestionArmed {
		return false
	}

	suggestions := m.currentSuggestions()
	if len(suggestions) == 0 {
		return false
	}
	m.normalizeSuggestionState()
	selected := strings.TrimSpace(suggestions[m.suggestions.index].Title)
	if selected == "" {
		return false
	}

	raw := strings.TrimSpace(m.input.Value())
	if !strings.HasPrefix(raw, "/") {
		return false
	}
	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return false
	}

	clean := strings.TrimPrefix(selected, "/")
	if len(parts) == 1 {
		m.input.SetValue("/" + clean + " ")
		m.normalizeSuggestionState()
		return true
	}
	if strings.HasSuffix(raw, " ") {
		parts = append(parts, clean)
	} else {
		parts[len(parts)-1] = clean
	}
	joined := strings.Join(parts, " ")
	if !strings.HasSuffix(joined, " ") {
		joined += " "
	}
	m.input.SetValue(joined)
	m.normalizeSuggestionState()
	return true
}
