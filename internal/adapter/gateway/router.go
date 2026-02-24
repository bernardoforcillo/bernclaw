// Package gateway provides the hub-and-spoke routing layer for multi-channel ingress.
// Inspired by OpenClaw's control plane pattern.
package gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// Router implements hub-and-spoke message routing to teams and agents.
// It orchestrates channel adapters (TUI, CLI, HTTP webhooks, etc.) and
// routes inbound messages to the appropriate team/agent based on context.
type Router struct {
	teamService port.TeamOrchestrator
	graphStore  port.GraphStore
	// sessionRouter maps session ID to team/agent
	sessionRouter map[string]SessionContext
	mu            sync.RWMutex
}

// SessionContext tracks active sessions and their team/agent mappings.
type SessionContext struct {
	SessionID string
	TeamName  string
	UserID    string
	// Future: auth context, permissions, preferences
}

// NewRouter creates a gateway router with team orchestration.
func NewRouter(teamService port.TeamOrchestrator, graphStore port.GraphStore) *Router {
	return &Router{
		teamService:   teamService,
		graphStore:    graphStore,
		sessionRouter: make(map[string]SessionContext),
	}
}

// HandleInboundMessage routes an inbound message to the appropriate handler.
// Input can be from any channel: TUI, CLI, HTTP, Voice, Webhook, etc.
func (r *Router) HandleInboundMessage(ctx context.Context, sessionID string, userInput string) (string, error) {
	r.mu.RLock()
	sess, ok := r.sessionRouter[sessionID]
	r.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	// Look up team
	team, err := r.graphStore.GetTeamCoordination(ctx, sess.TeamName)
	if err != nil {
		return "", fmt.Errorf("team not found: %s", sess.TeamName)
	}

	// Dispatch task to team via TeamService
	// This is a simplified flow; real implementation would parse commands, check permissions, etc.
	candidates, err := r.teamService.DispatchTask(ctx, sess.TeamName, domain.TaskRequest{
		Title:   "route incoming message",
		Context: map[string]any{"input": userInput},
	})

	if err != nil {
		return "", fmt.Errorf("task dispatch failed: %w", err)
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("no candidate agents for message routing in team %s", sess.TeamName)
	}

	// For now, return a placeholder response
	// In a real implementation, would invoke the first candidate and return their output
	return fmt.Sprintf("[Gateway] Routed to team '%s' agents %v", team.Name, candidates), nil
}

// OpenSession creates a new session routed to a specific team.
// Returns the session ID.
func (r *Router) OpenSession(ctx context.Context, teamName string, userID string) (string, error) {
	// Verify team exists
	_, err := r.graphStore.GetTeamCoordination(ctx, teamName)
	if err != nil {
		return "", fmt.Errorf("team not found: %s", teamName)
	}

	sessionID := fmt.Sprintf("sess_%s_%s_%d", userID, teamName, domain.NowUnix())

	r.mu.Lock()
	r.sessionRouter[sessionID] = SessionContext{
		SessionID: sessionID,
		TeamName:  teamName,
		UserID:    userID,
	}
	r.mu.Unlock()

	return sessionID, nil
}

// CloseSession terminates an active session.
func (r *Router) CloseSession(ctx context.Context, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessionRouter[sessionID]; !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	delete(r.sessionRouter, sessionID)
	return nil
}

// ListActiveSessions returns all open sessions (for debugging/monitoring).
func (r *Router) ListActiveSessions(ctx context.Context) []SessionContext {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []SessionContext
	for _, sess := range r.sessionRouter {
		result = append(result, sess)
	}
	return result
}
