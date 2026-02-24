package domain

// Message represents a single turn in an LLM conversation.
// Role is one of "system", "user", "assistant", or "utility" (a
// display-only role used by the TUI and never forwarded to the LLM).
type Message struct {
	Role    string
	Content string
}
