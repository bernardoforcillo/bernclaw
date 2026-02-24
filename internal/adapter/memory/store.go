// Package memory provides in-memory memory storage with semantic search capabilities
package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// InMemoryStore implements port.MemoryStore with basic semantic search
type InMemoryStore struct {
	entries map[string]port.MemoryEntry
	mu      sync.RWMutex
}

// NewInMemoryStore creates an empty memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		entries: make(map[string]port.MemoryEntry),
	}
}

// SearchSemantic searches by semantic similarity (mock implementation)
// In production, this would use vector similarity with embeddings
func (s *InMemoryStore) SearchSemantic(ctx context.Context, query string, limit int) ([]port.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []port.MemoryEntry

	// Simple keyword matching for now
	queryWords := strings.Fields(strings.ToLower(query))

	for _, entry := range s.entries {
		contentLower := strings.ToLower(entry.Content)
		score := 0

		// Score based on keyword matches
		for _, word := range queryWords {
			if strings.Contains(contentLower, word) {
				score++
			}
		}

		// Also check keywords
		for _, keyword := range entry.Keywords {
			for _, queryWord := range queryWords {
				if strings.Contains(strings.ToLower(keyword), queryWord) {
					score++
				}
			}
		}

		if score > 0 {
			results = append(results, entry)
		}
	}

	// Sort by score (higher first) and timestamp (newer first)
	sort.Slice(results, func(i, j int) bool {
		// In production, this would be by embedding similarity
		// For now, score is based on keyword matches
		if results[i].Timestamp != results[j].Timestamp {
			return results[i].Timestamp > results[j].Timestamp
		}
		return results[i].ID < results[j].ID
	})

	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// SearchKeywords searches by exact keyword matches
func (s *InMemoryStore) SearchKeywords(ctx context.Context, keywords []string, limit int) ([]port.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []port.MemoryEntry

	for _, entry := range s.entries {
		// Check if any keywords match
		for _, keyword := range keywords {
			for _, entryKeyword := range entry.Keywords {
				if strings.EqualFold(keyword, entryKeyword) {
					results = append(results, entry)
					goto nextEntry
				}
			}
		}
	nextEntry:
	}

	// Sort by timestamp (newer first)
	sort.Slice(results, func(i, j int) bool {
		if results[i].Timestamp != results[j].Timestamp {
			return results[i].Timestamp > results[j].Timestamp
		}
		return results[i].ID < results[j].ID
	})

	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Search performs hybrid search (semantic + keyword)
func (s *InMemoryStore) Search(ctx context.Context, query string, keywords []string, limit int) ([]port.MemoryEntry, error) {
	// In production, this would combine semantic and keyword search with scoring
	// For now, semantic search takes priority

	if query != "" {
		return s.SearchSemantic(ctx, query, limit)
	}

	if len(keywords) > 0 {
		return s.SearchKeywords(ctx, keywords, limit)
	}

	// Return recent entries if no query/keywords
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []port.MemoryEntry
	for _, entry := range s.entries {
		results = append(results, entry)
	}

	// Sort by timestamp (newer first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp > results[j].Timestamp
	})

	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// Store saves a memory entry
func (s *InMemoryStore) Store(ctx context.Context, entry port.MemoryEntry) error {
	if entry.ID == "" {
		return fmt.Errorf("memory entry must have an ID")
	}

	// Set timestamp if not provided
	if entry.Timestamp == 0 {
		entry.Timestamp = time.Now().Unix()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[entry.ID] = entry
	return nil
}

// Get retrieves a memory entry by ID
func (s *InMemoryStore) Get(ctx context.Context, id string) (port.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[id]
	if !exists {
		return port.MemoryEntry{}, fmt.Errorf("memory entry not found: %s", id)
	}

	return entry, nil
}

// ListBySession returns all memory entries for a session
func (s *InMemoryStore) ListBySession(ctx context.Context, sessionID string) ([]port.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []port.MemoryEntry
	for _, entry := range s.entries {
		if entry.SessionID == sessionID {
			results = append(results, entry)
		}
	}

	// Sort by timestamp (newer first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp > results[j].Timestamp
	})

	return results, nil
}

// ListBySource returns all entries from a specific source
func (s *InMemoryStore) ListBySource(ctx context.Context, source string) ([]port.MemoryEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []port.MemoryEntry
	for _, entry := range s.entries {
		if entry.Source == source {
			results = append(results, entry)
		}
	}

	// Sort by timestamp (newer first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp > results[j].Timestamp
	})

	return results, nil
}

// Count returns the total number of entries
func (s *InMemoryStore) Count(ctx context.Context) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Clear removes all entries
func (s *InMemoryStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make(map[string]port.MemoryEntry)
	return nil
}

// MemoryCollector automatically stores important interactions
type MemoryCollector struct {
	store port.MemoryStore
}

// NewMemoryCollector creates a collector that saves interactions
func NewMemoryCollector(store port.MemoryStore) *MemoryCollector {
	return &MemoryCollector{store: store}
}

// StoreUserMessage records a user message in memory
func (c *MemoryCollector) StoreUserMessage(ctx context.Context, sessionID, userMessage string, keywords []string) error {
	entry := port.MemoryEntry{
		ID:        fmt.Sprintf("msg_%s_%d", sessionID, time.Now().UnixNano()),
		Content:   userMessage,
		Keywords:  keywords,
		Source:    "user_message",
		Timestamp: time.Now().Unix(),
		SessionID: sessionID,
	}

	return c.store.Store(ctx, entry)
}

// StoreToolResult records a tool execution result
func (c *MemoryCollector) StoreToolResult(ctx context.Context, sessionID, toolName string, result interface{}, keywords []string) error {
	resultStr := fmt.Sprintf("%v", result)
	entry := port.MemoryEntry{
		ID:        fmt.Sprintf("tool_%s_%s_%d", sessionID, toolName, time.Now().UnixNano()),
		Content:   fmt.Sprintf("Tool %s result: %s", toolName, resultStr),
		Keywords:  append(keywords, toolName),
		Source:    "tool_result",
		Timestamp: time.Now().Unix(),
		SessionID: sessionID,
	}

	return c.store.Store(ctx, entry)
}

// StoreModelResponse records a model response
func (c *MemoryCollector) StoreModelResponse(ctx context.Context, sessionID, response string, keywords []string) error {
	entry := port.MemoryEntry{
		ID:        fmt.Sprintf("response_%s_%d", sessionID, time.Now().UnixNano()),
		Content:   response,
		Keywords:  keywords,
		Source:    "model_response",
		Timestamp: time.Now().Unix(),
		SessionID: sessionID,
	}

	return c.store.Store(ctx, entry)
}
