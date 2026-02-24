// Package policy defines security policies for tool execution and session access
package policy

import (
	"context"
	"fmt"

	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// DefaultToolPolicy implements port.ToolPolicy with sensible defaults
type DefaultToolPolicy struct {
	mainSessionTools  map[string]port.ExecutionEnvironment
	dmSessionTools    map[string]port.ExecutionEnvironment
	groupSessionTools map[string]port.ExecutionEnvironment
}

// NewDefaultToolPolicy creates a policy with predefined tool access rules
func NewDefaultToolPolicy() *DefaultToolPolicy {
	return &DefaultToolPolicy{
		mainSessionTools: map[string]port.ExecutionEnvironment{
			"read-file": {
				Type:           "host",
				Sandboxed:      false,
				NetworkAccess:  true,
				FileAccess:     true,
				MaxMemoryMB:    512,
				TimeoutSeconds: 300,
			},
			"write-file": {
				Type:           "host",
				Sandboxed:      false,
				NetworkAccess:  true,
				FileAccess:     true,
				MaxMemoryMB:    256,
				TimeoutSeconds: 60,
			},
			"execute-command": {
				Type:           "host",
				Sandboxed:      false,
				NetworkAccess:  true,
				FileAccess:     true,
				MaxMemoryMB:    1024,
				TimeoutSeconds: 300,
			},
			"analyze-data": {
				Type:           "host",
				Sandboxed:      false,
				NetworkAccess:  false,
				FileAccess:     true,
				MaxMemoryMB:    2048,
				TimeoutSeconds: 120,
			},
		},
		dmSessionTools: map[string]port.ExecutionEnvironment{
			"read-file": {
				Type:           "docker",
				Sandboxed:      true,
				NetworkAccess:  false,
				FileAccess:     true,
				MaxMemoryMB:    128,
				TimeoutSeconds: 30,
			},
			"write-file": {
				Type:           "none", // Disabled
				Sandboxed:      true,
				NetworkAccess:  false,
				FileAccess:     false,
				MaxMemoryMB:    0,
				TimeoutSeconds: 0,
			},
			"search-memory": {
				Type:           "docker",
				Sandboxed:      true,
				NetworkAccess:  false,
				FileAccess:     false,
				MaxMemoryMB:    256,
				TimeoutSeconds: 10,
			},
			"analyze-data": {
				Type:           "docker",
				Sandboxed:      true,
				NetworkAccess:  false,
				FileAccess:     false,
				MaxMemoryMB:    512,
				TimeoutSeconds: 30,
			},
		},
		groupSessionTools: map[string]port.ExecutionEnvironment{
			"read-file": {
				Type:           "none", // Disabled
				Sandboxed:      true,
				NetworkAccess:  false,
				FileAccess:     false,
				MaxMemoryMB:    0,
				TimeoutSeconds: 0,
			},
			"write-file": {
				Type:           "none", // Disabled
				Sandboxed:      true,
				NetworkAccess:  false,
				FileAccess:     false,
				MaxMemoryMB:    0,
				TimeoutSeconds: 0,
			},
			"search-memory": {
				Type:           "docker",
				Sandboxed:      true,
				NetworkAccess:  false,
				FileAccess:     false,
				MaxMemoryMB:    128,
				TimeoutSeconds: 5,
			},
			"analyze-data": {
				Type:           "docker",
				Sandboxed:      true,
				NetworkAccess:  false,
				FileAccess:     false,
				MaxMemoryMB:    256,
				TimeoutSeconds: 15,
			},
		},
	}
}

// CanExecute determines whether a tool can be executed in a session
func (p *DefaultToolPolicy) CanExecute(ctx context.Context, session port.Session, toolName string) (bool, error) {
	// First check session-level permissions
	if !session.HasPermission("invoke_tools") {
		return false, fmt.Errorf("session does not have tool invocation permission")
	}

	// Then check session tool allowlist
	if !session.CanUseTool(toolName) {
		return false, fmt.Errorf("tool %s not in session allowlist", toolName)
	}

	// Check if tool is available for this session type
	env := p.GetExecutionEnvironment(toolName)
	if env.Type == "none" {
		return false, fmt.Errorf("tool %s is disabled for session type %s", toolName, session.GetType())
	}

	return true, nil
}

// GetExecutionEnvironment returns how a tool should be executed for this session
func (p *DefaultToolPolicy) GetExecutionEnvironment(toolName string) port.ExecutionEnvironment {
	// This is simplified; in production, we'd need the session type
	// For now, return the main session policy as default

	if env, exists := p.mainSessionTools[toolName]; exists {
		return env
	}

	// Default fallback (disabled)
	return port.ExecutionEnvironment{
		Type:      "none",
		Sandboxed: true,
	}
}

// GetExecutionEnvironmentForSession returns the execution environment for a specific session
func (p *DefaultToolPolicy) GetExecutionEnvironmentForSession(sessionType, toolName string) port.ExecutionEnvironment {
	var tools map[string]port.ExecutionEnvironment

	switch sessionType {
	case "main":
		tools = p.mainSessionTools
	case "dm":
		tools = p.dmSessionTools
	case "group":
		tools = p.groupSessionTools
	default:
		// Unknown session type, use most restrictive
		tools = p.groupSessionTools
	}

	if env, exists := tools[toolName]; exists {
		return env
	}

	// Tool not defined for this session type, disable it
	return port.ExecutionEnvironment{
		Type:      "none",
		Sandboxed: true,
	}
}

// CustomToolPolicy allows fine-grained per-tool and per-session policies
type CustomToolPolicy struct {
	policies map[string]map[string]port.ExecutionEnvironment // session type -> tool -> env
}

// NewCustomToolPolicy creates an empty custom policy
func NewCustomToolPolicy() *CustomToolPolicy {
	return &CustomToolPolicy{
		policies: make(map[string]map[string]port.ExecutionEnvironment),
	}
}

// SetToolEnvironment sets the execution environment for a tool in a session type
func (p *CustomToolPolicy) SetToolEnvironment(sessionType, toolName string, env port.ExecutionEnvironment) {
	if _, exists := p.policies[sessionType]; !exists {
		p.policies[sessionType] = make(map[string]port.ExecutionEnvironment)
	}

	p.policies[sessionType][toolName] = env
}

// CanExecute checks if a tool can be executed
func (p *CustomToolPolicy) CanExecute(ctx context.Context, session port.Session, toolName string) (bool, error) {
	if !session.HasPermission("invoke_tools") {
		return false, fmt.Errorf("session does not have tool invocation permission")
	}

	if !session.CanUseTool(toolName) {
		return false, fmt.Errorf("tool %s not in session allowlist", toolName)
	}

	env := p.GetExecutionEnvironment(session.GetType(), toolName)
	if env.Type == "none" {
		return false, fmt.Errorf("tool %s disabled for session type %s", toolName, session.GetType())
	}

	return true, nil
}

// GetExecutionEnvironment returns the environment for a tool in a session type
func (p *CustomToolPolicy) GetExecutionEnvironment(sessionType, toolName string) port.ExecutionEnvironment {
	if tools, exists := p.policies[sessionType]; exists {
		if env, toolExists := tools[toolName]; toolExists {
			return env
		}
	}

	// Default to disabled
	return port.ExecutionEnvironment{
		Type: "none",
	}
}

// SecurityValidator validates tool execution requests against policies
type SecurityValidator struct {
	policy port.ToolPolicy
}

// NewSecurityValidator creates a validator
func NewSecurityValidator(policy port.ToolPolicy) *SecurityValidator {
	return &SecurityValidator{policy: policy}
}

// ValidateToolExecution checks if a tool can be executed in a session
func (v *SecurityValidator) ValidateToolExecution(
	ctx context.Context,
	session port.Session,
	toolName string,
) (port.ExecutionEnvironment, error) {
	// Check if tool can execute
	canExecute, err := v.policy.CanExecute(ctx, session, toolName)
	if err != nil {
		return port.ExecutionEnvironment{}, err
	}

	if !canExecute {
		return port.ExecutionEnvironment{}, fmt.Errorf("tool execution denied by policy")
	}

	// Get execution environment
	env := v.policy.GetExecutionEnvironment(toolName)

	// Validate environment
	if env.Type == "none" {
		return port.ExecutionEnvironment{}, fmt.Errorf("tool execution environment not configured")
	}

	return env, nil
}
