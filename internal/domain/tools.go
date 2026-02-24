// Package domain defines tools available to agents.
// These are compiled from TOOLS.md definitions.
package domain

// Tool represents a single tool that agents can invoke.
type Tool struct {
	Name         string               // Tool identifier (e.g., "read-file")
	Category     ToolCategory         // Category of tool
	Description  string               // What this tool does
	Parameters   map[string]Parameter // Input parameters
	ReturnType   string               // Type of return value
	Policies     ToolPolicies         // Execution policies by session type
	TokenCost    int                  // Estimated token cost
	Experimental bool                 // Is this tool experimental?
}

// ToolCategory represents a category of tools.
type ToolCategory string

const (
	CategoryData          ToolCategory = "data"
	CategoryCommunication ToolCategory = "communication"
	CategoryAnalysis      ToolCategory = "analysis"
	CategorySystem        ToolCategory = "system"
)

// Parameter represents a tool parameter.
type Parameter struct {
	Name        string      // Parameter name
	Type        string      // "string", "number", "boolean", "array", etc.
	Description string      // What this parameter does
	Required    bool        // Is this parameter required?
	Default     interface{} // Default value if not required
}

// ToolPolicies defines execution rules for different session types.
type ToolPolicies struct {
	Main  ToolPolicy // Policy for main/operator sessions
	DM    ToolPolicy // Policy for direct message sessions
	Group ToolPolicy // Policy for group chat sessions
}

// ToolPolicy defines how a tool can be executed in a session.
type ToolPolicy struct {
	Allowed        bool   // Is this tool allowed for this session type?
	Sandboxed      bool   // Should execution be sandboxed?
	TimeoutSeconds int    // Maximum execution time
	Description    string // Why this policy?
}

// AllTools returns all available tools.
func AllTools() []Tool {
	return []Tool{
		// Data Tools
		{
			Name:        "read-file",
			Category:    CategoryData,
			Description: "Read contents of workspace files",
			Parameters: map[string]Parameter{
				"path": {
					Name:        "path",
					Type:        "string",
					Description: "Relative path to file in workspace",
					Required:    true,
				},
			},
			ReturnType: "string",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 300,
					Description:    "Full file access in main session",
				},
				DM: ToolPolicy{
					Allowed:        true,
					Sandboxed:      true,
					TimeoutSeconds: 30,
					Description:    "Sandboxed read-only file access",
				},
				Group: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "No file access for group sessions",
				},
			},
			TokenCost:    50,
			Experimental: false,
		},
		{
			Name:        "write-file",
			Category:    CategoryData,
			Description: "Save results back to workspace",
			Parameters: map[string]Parameter{
				"path": {
					Name:        "path",
					Type:        "string",
					Description: "Relative path to file in workspace",
					Required:    true,
				},
				"content": {
					Name:        "content",
					Type:        "string",
					Description: "File content to write",
					Required:    true,
				},
			},
			ReturnType: "boolean",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 60,
					Description:    "Full write access in main session",
				},
				DM: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "No write access for DM sessions",
				},
				Group: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "No write access for group sessions",
				},
			},
			TokenCost:    50,
			Experimental: false,
		},
		{
			Name:        "search-memory",
			Category:    CategoryData,
			Description: "Semantic search over agent memory",
			Parameters: map[string]Parameter{
				"query": {
					Name:        "query",
					Type:        "string",
					Description: "Search query",
					Required:    true,
				},
				"limit": {
					Name:        "limit",
					Type:        "number",
					Description: "Maximum results to return",
					Required:    false,
					Default:     5,
				},
			},
			ReturnType: "array",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 10,
					Description:    "Full memory access in main session",
				},
				DM: ToolPolicy{
					Allowed:        true,
					Sandboxed:      true,
					TimeoutSeconds: 10,
					Description:    "Search own memories in DM session",
				},
				Group: ToolPolicy{
					Allowed:        true,
					Sandboxed:      true,
					TimeoutSeconds: 5,
					Description:    "Search public memories only in group",
				},
			},
			TokenCost:    30,
			Experimental: false,
		},
		{
			Name:        "query-database",
			Category:    CategoryData,
			Description: "Execute read-only database queries",
			Parameters: map[string]Parameter{
				"query": {
					Name:        "query",
					Type:        "string",
					Description: "SQL SELECT query",
					Required:    true,
				},
			},
			ReturnType: "array",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 30,
					Description:    "Full database access in main session",
				},
				DM: ToolPolicy{
					Allowed:        true,
					Sandboxed:      true,
					TimeoutSeconds: 10,
					Description:    "Limited database queries in DM session",
				},
				Group: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "No database access for group sessions",
				},
			},
			TokenCost:    100,
			Experimental: false,
		},
		// Communication Tools
		{
			Name:        "send-message",
			Category:    CategoryCommunication,
			Description: "Send message to user or channel",
			Parameters: map[string]Parameter{
				"recipient": {
					Name:        "recipient",
					Type:        "string",
					Description: "User ID or channel to send to",
					Required:    true,
				},
				"message": {
					Name:        "message",
					Type:        "string",
					Description: "Message content",
					Required:    true,
				},
			},
			ReturnType: "boolean",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 5,
					Description:    "Send messages from main session",
				},
				DM: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 5,
					Description:    "Send reply in DM session",
				},
				Group: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 5,
					Description:    "Send group message",
				},
			},
			TokenCost:    20,
			Experimental: false,
		},
		{
			Name:        "notify-agent",
			Category:    CategoryCommunication,
			Description: "Alert another agent about an issue",
			Parameters: map[string]Parameter{
				"agent": {
					Name:        "agent",
					Type:        "string",
					Description: "Name of agent to notify",
					Required:    true,
				},
				"message": {
					Name:        "message",
					Type:        "string",
					Description: "Notification message",
					Required:    true,
				},
			},
			ReturnType: "boolean",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 10,
					Description:    "Notify other agents from main session",
				},
				DM: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "Cannot notify agents from DM session",
				},
				Group: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "Cannot notify agents from group session",
				},
			},
			TokenCost:    15,
			Experimental: false,
		},
		{
			Name:        "session-spawn",
			Category:    CategoryCommunication,
			Description: "Create a new sub-session for delegation",
			Parameters: map[string]Parameter{
				"agent": {
					Name:        "agent",
					Type:        "string",
					Description: "Agent to execute task",
					Required:    true,
				},
				"task": {
					Name:        "task",
					Type:        "string",
					Description: "Task description",
					Required:    true,
				},
				"timeout": {
					Name:        "timeout",
					Type:        "number",
					Description: "Timeout in seconds",
					Required:    false,
					Default:     30,
				},
			},
			ReturnType: "object",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 60,
					Description:    "Spawn sub-sessions from main session",
				},
				DM: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "Cannot spawn sessions from DM",
				},
				Group: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "Cannot spawn sessions from group",
				},
			},
			TokenCost:    40,
			Experimental: false,
		},
		// Analysis Tools
		{
			Name:        "analyze-data",
			Category:    CategoryAnalysis,
			Description: "Statistical analysis of datasets",
			Parameters: map[string]Parameter{
				"data": {
					Name:        "data",
					Type:        "array",
					Description: "Data to analyze",
					Required:    true,
				},
				"analysis_type": {
					Name:        "analysis_type",
					Type:        "string",
					Description: "Type of analysis: descriptive, trend, correlation",
					Required:    false,
					Default:     "descriptive",
				},
			},
			ReturnType: "object",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 120,
					Description:    "Full analysis capability in main session",
				},
				DM: ToolPolicy{
					Allowed:        true,
					Sandboxed:      true,
					TimeoutSeconds: 30,
					Description:    "Sandboxed analysis in DM session",
				},
				Group: ToolPolicy{
					Allowed:        true,
					Sandboxed:      true,
					TimeoutSeconds: 15,
					Description:    "Limited analysis in group session",
				},
			},
			TokenCost:    100,
			Experimental: false,
		},
		{
			Name:        "generate-report",
			Category:    CategoryAnalysis,
			Description: "Create formatted reports",
			Parameters: map[string]Parameter{
				"title": {
					Name:        "title",
					Type:        "string",
					Description: "Report title",
					Required:    true,
				},
				"sections": {
					Name:        "sections",
					Type:        "array",
					Description: "Report sections",
					Required:    true,
				},
				"format": {
					Name:        "format",
					Type:        "string",
					Description: "Output format: markdown, json, html",
					Required:    false,
					Default:     "markdown",
				},
			},
			ReturnType: "string",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 60,
					Description:    "Generate reports from main session",
				},
				DM: ToolPolicy{
					Allowed:        true,
					Sandboxed:      true,
					TimeoutSeconds: 30,
					Description:    "Generate reports in DM session",
				},
				Group: ToolPolicy{
					Allowed:        true,
					Sandboxed:      true,
					TimeoutSeconds: 20,
					Description:    "Generate reports in group session",
				},
			},
			TokenCost:    80,
			Experimental: false,
		},
		{
			Name:        "extract-insights",
			Category:    CategoryAnalysis,
			Description: "Identify key patterns",
			Parameters: map[string]Parameter{
				"data": {
					Name:        "data",
					Type:        "array",
					Description: "Data to extract insights from",
					Required:    true,
				},
				"focus": {
					Name:        "focus",
					Type:        "string",
					Description: "Focus area for insights",
					Required:    false,
				},
			},
			ReturnType: "array",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 60,
					Description:    "Extract insights in main session",
				},
				DM: ToolPolicy{
					Allowed:        true,
					Sandboxed:      true,
					TimeoutSeconds: 30,
					Description:    "Extract insights in DM session",
				},
				Group: ToolPolicy{
					Allowed:        true,
					Sandboxed:      true,
					TimeoutSeconds: 15,
					Description:    "Extract insights in group session",
				},
			},
			TokenCost:    70,
			Experimental: false,
		},
		// System Tools
		{
			Name:        "list-members",
			Category:    CategorySystem,
			Description: "Enumerate team members",
			Parameters: map[string]Parameter{
				"role": {
					Name:        "role",
					Type:        "string",
					Description: "Filter by role (optional)",
					Required:    false,
				},
			},
			ReturnType: "array",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 5,
					Description:    "List all members in main session",
				},
				DM: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "Cannot list members from DM session",
				},
				Group: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "Cannot list members from group session",
				},
			},
			TokenCost:    10,
			Experimental: false,
		},
		{
			Name:        "check-permissions",
			Category:    CategorySystem,
			Description: "Verify access to resources",
			Parameters: map[string]Parameter{
				"resource": {
					Name:        "resource",
					Type:        "string",
					Description: "Resource to check",
					Required:    true,
				},
			},
			ReturnType: "boolean",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 5,
					Description:    "Check permissions in main session",
				},
				DM: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 5,
					Description:    "Check own permissions in DM session",
				},
				Group: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 5,
					Description:    "Check permissions in group session",
				},
			},
			TokenCost:    15,
			Experimental: false,
		},
		{
			Name:        "get-status",
			Category:    CategorySystem,
			Description: "Query system health",
			Parameters: map[string]Parameter{
				"component": {
					Name:        "component",
					Type:        "string",
					Description: "Component to check, or 'all'",
					Required:    false,
					Default:     "all",
				},
			},
			ReturnType: "object",
			Policies: ToolPolicies{
				Main: ToolPolicy{
					Allowed:        true,
					Sandboxed:      false,
					TimeoutSeconds: 10,
					Description:    "Full system status in main session",
				},
				DM: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "No system status access from DM",
				},
				Group: ToolPolicy{
					Allowed:        false,
					Sandboxed:      true,
					TimeoutSeconds: 0,
					Description:    "No system status access from group",
				},
			},
			TokenCost:    20,
			Experimental: false,
		},
	}
}

// FindTool finds a tool by name
func FindTool(name string) *Tool {
	for _, tool := range AllTools() {
		if tool.Name == name {
			return &tool
		}
	}
	return nil
}

// FindToolsByCategory returns all tools in a category
func FindToolsByCategory(category ToolCategory) []Tool {
	var result []Tool
	for _, tool := range AllTools() {
		if tool.Category == category {
			result = append(result, tool)
		}
	}
	return result
}

// AvailableToolsForSession returns tools available in a session type
func AvailableToolsForSession(sessionType string) []Tool {
	var result []Tool
	for _, tool := range AllTools() {
		var policy ToolPolicy
		switch sessionType {
		case "main":
			policy = tool.Policies.Main
		case "dm":
			policy = tool.Policies.DM
		case "group":
			policy = tool.Policies.Group
		default:
			continue
		}

		if policy.Allowed {
			result = append(result, tool)
		}
	}
	return result
}

// ExportToolDefinitions generates documentation of tools for system prompt injection
func ExportToolDefinitions(sessionType string) string {
	tools := AvailableToolsForSession(sessionType)

	doc := "# Available Tools\n\n"

	// Group by category
	categories := make(map[ToolCategory][]Tool)
	for _, tool := range tools {
		categories[tool.Category] = append(categories[tool.Category], tool)
	}

	for _, cat := range []ToolCategory{CategoryData, CategoryCommunication, CategoryAnalysis, CategorySystem} {
		if toolList, exists := categories[cat]; exists && len(toolList) > 0 {
			doc += "## " + string(cat) + " Tools\n\n"
			for _, tool := range toolList {
				doc += "### " + tool.Name + "\n\n"
				doc += tool.Description + "\n\n"

				if len(tool.Parameters) > 0 {
					doc += "**Parameters:**\n"
					for _, param := range tool.Parameters {
						required := "optional"
						if param.Required {
							required = "required"
						}
						doc += "- `" + param.Name + "` (" + param.Type + ", " + required + "): " + param.Description + "\n"
					}
					doc += "\n"
				}

				doc += "**Returns:** " + tool.ReturnType + "\n\n"
			}
		}
	}

	return doc
}
