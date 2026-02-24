// Package context provides message flow and execution context assembly
// Following OpenClaw's 6-phase execution: session → context → skills → memory → tools → stream
package context

import (
	"context"
	"fmt"
	"strings"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// Assembler builds complete execution context from session and workspace
type Assembler struct {
	workspace  port.Workspace
	memory     port.MemoryStore
	toolPolicy port.ToolPolicy
}

// NewAssembler creates a new context assembler
func NewAssembler(ws port.Workspace, memory port.MemoryStore, policy port.ToolPolicy) *Assembler {
	return &Assembler{
		workspace:  ws,
		memory:     memory,
		toolPolicy: policy,
	}
}

// AssembleSystemPrompt builds the complete system prompt from workspace files
// Follows the order: AGENTS.md (baseline) → SOUL.md (personality) → TOOLS.md (conventions)
func (a *Assembler) AssembleSystemPrompt(ctx context.Context, session port.Session, workspace port.Workspace) (string, error) {
	var parts []string

	// 1. Load AGENTS.md baseline
	if agentsContent, err := workspace.ReadFile("AGENTS.md"); err == nil && agentsContent != "" {
		// Extract only the section relevant to this agent
		agentSection := extractAgentSection(agentsContent, session.GetAgentName())
		if agentSection != "" {
			parts = append(parts, "# Agent Definition\n\n"+agentSection)
		}
	}

	// 2. Load SOUL.md for personality and values
	if soulContent, err := workspace.ReadFile("SOUL.md"); err == nil && soulContent != "" {
		parts = append(parts, "# Agent Values & Personality\n\n"+soulContent)
	}

	// 3. Load TOOLS.md for tool usage conventions
	if toolsContent, err := workspace.ReadFile("TOOLS.md"); err == nil && toolsContent != "" {
		parts = append(parts, "# Tool Usage Conventions\n\n"+toolsContent)
	}

	// 4. Session-specific context
	sessionContext := a.buildSessionContext(session)
	if sessionContext != "" {
		parts = append(parts, "# Session Context\n\n"+sessionContext)
	}

	// 5. Fallback system prompt if no files found
	if len(parts) == 0 {
		parts = append(parts, a.defaultSystemPrompt(session))
	}

	return strings.Join(parts, "\n\n---\n\n"), nil
}

// AssembleExecutionContext builds the full execution context for model invocation
func (a *Assembler) AssembleExecutionContext(ctx context.Context, session port.Session, workspace port.Workspace, memoryStore port.MemoryStore) (port.ExecutionContext, error) {
	execCtx := port.ExecutionContext{
		SessionID:      session.GetID(),
		SessionType:    session.GetType(),
		AgentName:      session.GetAgentName(),
		UserID:         session.GetUserID(),
		Permissions:    make(map[string]bool),
		AvailableTools: []port.Tool{},
		InjectedSkills: []string{},
	}

	// 1. Assemble system prompt
	systemPrompt, err := a.AssembleSystemPrompt(ctx, session, workspace)
	if err != nil {
		return execCtx, fmt.Errorf("failed to assemble system prompt: %w", err)
	}
	execCtx.SystemPrompt = systemPrompt

	// 2. Get conversation history
	execCtx.Messages = session.GetHistory(0) // Get all history

	// 3. Load available tools (filtered by session permissions)
	for _, tool := range workspace.GetTools() {
		if session.CanUseTool(tool.GetName()) {
			execCtx.AvailableTools = append(execCtx.AvailableTools, tool)
		}
	}

	// 4. Search for relevant memories
	if len(execCtx.Messages) > 0 {
		// Use last user message as search query
		lastUserMsg := ""
		for i := len(execCtx.Messages) - 1; i >= 0; i-- {
			if execCtx.Messages[i].Role == "user" {
				lastUserMsg = execCtx.Messages[i].Content
				break
			}
		}

		if lastUserMsg != "" && memoryStore != nil {
			memories, err := memoryStore.SearchSemantic(ctx, lastUserMsg, 5)
			if err == nil {
				execCtx.RelevantMemories = memories
			}
		}
	}

	// 5. Copy permissions from session
	for perm := range execCtx.Permissions {
		execCtx.Permissions[perm] = session.HasPermission(perm)
	}

	execCtx.Sandbox = session.GetType() != "main"

	return execCtx, nil
}

// InjectSkills selectively includes relevant skills in the context
func (a *Assembler) InjectSkills(ctx context.Context, skills []port.Skill, constraints *port.SecurityConstraints) ([]port.Skill, error) {
	var injected []port.Skill

	for _, skill := range skills {
		// Check trust level matches session type
		if a.skillAppropriateForSession(skill, constraints.SessionType) {
			injected = append(injected, skill)
		}
	}

	return injected, nil
}

// extractAgentSection finds the section for a specific agent in AGENTS.md
func extractAgentSection(content string, agentName string) string {
	lines := strings.Split(content, "\n")
	var section strings.Builder
	inSection := false

	for _, line := range lines {
		// Check if this is the agent's section (e.g., "## AgentName" or "### AgentName")
		if strings.HasPrefix(line, "##") && strings.Contains(line, agentName) {
			inSection = true
			section.WriteString(line + "\n")
			continue
		}

		// Stop at next section header
		if inSection && strings.HasPrefix(line, "##") && !strings.Contains(line, agentName) {
			break
		}

		if inSection {
			section.WriteString(line + "\n")
		}
	}

	return strings.TrimSpace(section.String())
}

// buildSessionContext creates context-specific information about the session
func (a *Assembler) buildSessionContext(session port.Session) string {
	lines := []string{
		fmt.Sprintf("Session Type: %s", session.GetType()),
		fmt.Sprintf("Channel: %s", session.GetChannel()),
		fmt.Sprintf("User: %s", session.GetUserID()),
	}

	// Add session-specific constraints
	switch session.GetType() {
	case "main":
		lines = append(lines, "Trust Level: Full (operator-controlled)")
		lines = append(lines, "Constraints: None - you have full access to the system")
	case "dm":
		lines = append(lines, "Trust Level: Medium (direct message from external user)")
		lines = append(lines, "Constraints: Sandboxed execution, limited file access, no network access")
	case "group":
		lines = append(lines, "Trust Level: Low (group message from potentially untrusted users)")
		lines = append(lines, "Constraints: Strict sandboxing, read-only file access, no network access")
	}

	return strings.Join(lines, "\n")
}

// defaultSystemPrompt returns a fallback system prompt
func (a *Assembler) defaultSystemPrompt(session port.Session) string {
	return fmt.Sprintf(`You are %s, an AI agent designed to help with various tasks.

Current Session:
- Type: %s
- Channel: %s
- User: %s

You have access to various tools and should use them appropriately to help the user.
Always be helpful, honest, and harmless.`, session.GetAgentName(), session.GetType(), session.GetChannel(), session.GetUserID())
}

// skillAppropriateForSession checks if a skill is appropriate for the session trust level
func (a *Assembler) skillAppropriateForSession(skill port.Skill, sessionType string) bool {
	switch sessionType {
	case "main":
		// All skills available to main session
		return true
	case "dm":
		// Direct messages can use public and user-level skills
		return skill.TrustLevel == "public" || skill.TrustLevel == "user"
	case "group":
		// Groups only get public skills
		return skill.TrustLevel == "public"
	}

	return false
}

// MessageFlow represents the 6-phase execution flow
// Phase 1: MessageInput → Phase 2: SessionResolution → Phase 3: ContextAssembly →
// Phase 4: SkillInjection → Phase 5: ToolExecution → Phase 6: StreamCompletion
type MessageFlow struct {
	sessionManager port.SessionManager
	assembler      *Assembler
	toolPolicy     port.ToolPolicy
}

// NewMessageFlow creates a new message flow handler
func NewMessageFlow(
	sessionMgr port.SessionManager,
	asm *Assembler,
	toolPolicy port.ToolPolicy,
) *MessageFlow {
	return &MessageFlow{
		sessionManager: sessionMgr,
		assembler:      asm,
		toolPolicy:     toolPolicy,
	}
}

// ProcessMessage executes the 6-phase message flow
func (mf *MessageFlow) ProcessMessage(
	ctx context.Context,
	agentName, channel, identifier, userID, userMessage string,
) (port.ExecutionContext, error) {
	// Phase 1: Already have the input message

	// Phase 2: Resolve or create session
	session, err := mf.sessionManager.ResolveOrCreate(ctx, agentName, channel, identifier, userID)
	if err != nil {
		return port.ExecutionContext{}, fmt.Errorf("phase 2 (session resolution) failed: %w", err)
	}

	// Add user message to session
	session.AddMessage(domain.Message{
		Role:    "user",
		Content: userMessage,
	})

	// Phase 3: Assemble execution context (system prompt, history, etc.)
	// Note: workspace and memory would need to be injected
	// For now, we just load from the session
	execCtx := port.ExecutionContext{
		SessionID:   session.GetID(),
		SessionType: session.GetType(),
		AgentName:   agentName,
		UserID:      userID,
		Messages:    session.GetHistory(0),
		Sandbox:     session.GetType() != "main",
	}

	// Phase 4: Skills would be injected here based on context

	// Phase 5: Tool execution would happen during model invocation (streaming)

	// Phase 6: Stream completion happens in the model client

	return execCtx, nil
}
