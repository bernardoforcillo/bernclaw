package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ToolInput carries a tool invocation from the LLM.
type ToolInput struct {
	Name      string
	Arguments map[string]any
}

// ToolOutput carries the result of a tool execution back to the LLM.
type ToolOutput struct {
	Name    string
	Content string
	Meta    map[string]any
}

// ToolExecutor is implemented by anything that can handle a named tool call.
type ToolExecutor interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input ToolInput) (ToolOutput, error)
}

// MemoryStore persists and loads conversation messages per-agent.
type MemoryStore interface {
	Load(ctx context.Context, agentName string) ([]Message, error)
	Append(ctx context.Context, agentName string, message Message) error
}

// Planner builds the system prompt for an agent at a given point in time.
type Planner interface {
	BuildSystemPrompt(spec Spec, now time.Time) string
}

// BeforeRunHook is called before a run starts.
type BeforeRunHook func(ctx context.Context, state *RunState) error

// AfterRunHook is called after a run completes.
type AfterRunHook func(ctx context.Context, state *RunState) error

// RunState captures the complete mutable state for a single agent run.
type RunState struct {
	Agent         Spec
	Input         string
	Messages      []Message
	ToolCalls     []ToolInput
	ToolOutputs   []ToolOutput
	Response      string
	StartedAt     time.Time
	CompletedAt   time.Time
	FailureReason string
}

// Runtime orchestrates a team of agents together with optional tools and memory.
type Runtime struct {
	team        Team
	activeAgent string
	tools       map[string]ToolExecutor
	memory      MemoryStore
	planner     Planner
	beforeHooks []BeforeRunHook
	afterHooks  []AfterRunHook
}

// NewRuntime creates a Runtime for the given team.
func NewRuntime(team Team) *Runtime {
	return &Runtime{
		team:        team,
		activeAgent: team.DefaultAgentName(),
		tools:       map[string]ToolExecutor{},
	}
}

// Team returns the runtime's team.
func (r *Runtime) Team() Team { return r.team }

// ActiveAgent returns the name of the currently selected agent.
func (r *Runtime) ActiveAgent() string { return r.activeAgent }

// SetActiveAgent selects the named agent. Returns an error if not found.
func (r *Runtime) SetActiveAgent(name string) error {
	if _, found := r.team.Find(name); !found {
		return fmt.Errorf("agent not found: %s", name)
	}
	r.activeAgent = NormalizeName(name)
	return nil
}

// RegisterTool adds (or replaces) a tool available to this runtime.
func (r *Runtime) RegisterTool(tool ToolExecutor) {
	if tool == nil {
		return
	}
	r.tools[NormalizeName(tool.Name())] = tool
}

// SetMemory configures the memory store.
func (r *Runtime) SetMemory(store MemoryStore) { r.memory = store }

// SetPlanner configures the system-prompt planner.
func (r *Runtime) SetPlanner(planner Planner) { r.planner = planner }

// AddBeforeHook registers a hook to run before each execution.
func (r *Runtime) AddBeforeHook(hook BeforeRunHook) {
	if hook != nil {
		r.beforeHooks = append(r.beforeHooks, hook)
	}
}

// AddAfterHook registers a hook to run after each execution.
func (r *Runtime) AddAfterHook(hook AfterRunHook) {
	if hook != nil {
		r.afterHooks = append(r.afterHooks, hook)
	}
}

// BuildRunState prepares the RunState for a new execution, loading memory and
// running before-hooks.
func (r *Runtime) BuildRunState(ctx context.Context, input string) (*RunState, error) {
	spec, found := r.team.Find(r.activeAgent)
	if !found {
		return nil, fmt.Errorf("active agent not found: %s", r.activeAgent)
	}

	state := &RunState{
		Agent:     spec,
		Input:     strings.TrimSpace(input),
		StartedAt: time.Now(),
	}

	if r.memory != nil {
		messages, err := r.memory.Load(ctx, spec.Name)
		if err != nil {
			return nil, err
		}
		state.Messages = append(state.Messages, messages...)
	}

	for _, hook := range r.beforeHooks {
		if err := hook(ctx, state); err != nil {
			state.FailureReason = err.Error()
			return nil, err
		}
	}

	return state, nil
}

// CompleteRun finalises a RunState, running after-hooks and persisting the
// assistant reply to memory.
func (r *Runtime) CompleteRun(ctx context.Context, state *RunState) error {
	if state == nil {
		return nil
	}

	state.CompletedAt = time.Now()

	for _, hook := range r.afterHooks {
		if err := hook(ctx, state); err != nil {
			if state.FailureReason == "" {
				state.FailureReason = err.Error()
			}
			return err
		}
	}

	if r.memory != nil && state.Response != "" {
		return r.memory.Append(ctx, state.Agent.Name, Message{Role: "assistant", Content: state.Response})
	}

	return nil
}
