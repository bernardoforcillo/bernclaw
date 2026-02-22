package agent

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Message struct {
	Role    string
	Content string
}

type ToolInput struct {
	Name      string
	Arguments map[string]any
}

type ToolOutput struct {
	Name    string
	Content string
	Meta    map[string]any
}

type ToolExecutor interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input ToolInput) (ToolOutput, error)
}

type MemoryStore interface {
	Load(ctx context.Context, agentName string) ([]Message, error)
	Append(ctx context.Context, agentName string, message Message) error
}

type Planner interface {
	BuildSystemPrompt(spec Spec, now time.Time) string
}

type BeforeRunHook func(ctx context.Context, state *RunState) error
type AfterRunHook func(ctx context.Context, state *RunState) error

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

type Runtime struct {
	team        Team
	activeAgent string
	tools       map[string]ToolExecutor
	memory      MemoryStore
	planner     Planner
	beforeHooks []BeforeRunHook
	afterHooks  []AfterRunHook
}

func NewRuntime(team Team) *Runtime {
	return &Runtime{
		team:        team,
		activeAgent: team.DefaultAgentName(),
		tools:       map[string]ToolExecutor{},
	}
}

func (runtime *Runtime) Team() Team {
	return runtime.team
}

func (runtime *Runtime) ActiveAgent() string {
	return runtime.activeAgent
}

func (runtime *Runtime) SetActiveAgent(name string) error {
	if _, found := runtime.team.Find(name); !found {
		return fmt.Errorf("agent not found: %s", name)
	}
	runtime.activeAgent = normalizeName(name)
	return nil
}

func (runtime *Runtime) RegisterTool(tool ToolExecutor) {
	if tool == nil {
		return
	}
	runtime.tools[normalizeName(tool.Name())] = tool
}

func (runtime *Runtime) SetMemory(store MemoryStore) {
	runtime.memory = store
}

func (runtime *Runtime) SetPlanner(planner Planner) {
	runtime.planner = planner
}

func (runtime *Runtime) AddBeforeHook(hook BeforeRunHook) {
	if hook != nil {
		runtime.beforeHooks = append(runtime.beforeHooks, hook)
	}
}

func (runtime *Runtime) AddAfterHook(hook AfterRunHook) {
	if hook != nil {
		runtime.afterHooks = append(runtime.afterHooks, hook)
	}
}

func (runtime *Runtime) BuildRunState(ctx context.Context, input string) (*RunState, error) {
	agentSpec, found := runtime.team.Find(runtime.activeAgent)
	if !found {
		return nil, fmt.Errorf("active agent not found: %s", runtime.activeAgent)
	}

	state := &RunState{
		Agent:     agentSpec,
		Input:     strings.TrimSpace(input),
		StartedAt: time.Now(),
	}

	if runtime.memory != nil {
		messages, err := runtime.memory.Load(ctx, agentSpec.Name)
		if err != nil {
			return nil, err
		}
		state.Messages = append(state.Messages, messages...)
	}

	for _, hook := range runtime.beforeHooks {
		if err := hook(ctx, state); err != nil {
			state.FailureReason = err.Error()
			return nil, err
		}
	}

	return state, nil
}

func (runtime *Runtime) CompleteRun(ctx context.Context, state *RunState) error {
	if state == nil {
		return nil
	}

	state.CompletedAt = time.Now()

	for _, hook := range runtime.afterHooks {
		if err := hook(ctx, state); err != nil {
			if state.FailureReason == "" {
				state.FailureReason = err.Error()
			}
			return err
		}
	}

	if runtime.memory != nil && state.Response != "" {
		return runtime.memory.Append(ctx, state.Agent.Name, Message{Role: "assistant", Content: state.Response})
	}

	return nil
}
