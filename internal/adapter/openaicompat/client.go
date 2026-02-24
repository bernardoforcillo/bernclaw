// Package openaicompat provides a port.LLMClient implementation for any
// OpenAI-compatible chat-completions API endpoint (OpenAI, local models served
// via Ollama, LM Studio, etc.).
package openaicompat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultTimeout = 60 * time.Second
)

// Config holds the credentials and HTTP settings for the client.
type Config struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

// Client is a port.LLMClient backed by any OpenAI-compatible REST API.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewClient validates the config and returns a new Client.
func NewClient(cfg Config) (*Client, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("openai-compatible: api key is required")
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	baseURL = strings.TrimSuffix(baseURL, "/")

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	return &Client{apiKey: apiKey, baseURL: baseURL, httpClient: httpClient}, nil
}

// Generate implements port.LLMClient.  It sends the request to the
// /chat/completions endpoint (streaming) and returns the assembled reply.
func (c *Client) Generate(ctx context.Context, req port.GenerateRequest) (domain.Message, error) {
	if c == nil {
		return domain.Message{}, errors.New("openai-compatible: client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(req.ModelName) == "" {
		return domain.Message{}, errors.New("openai-compatible: model name is required")
	}
	if len(req.Messages) == 0 {
		return domain.Message{}, errors.New("openai-compatible: at least one message is required")
	}

	apiMessages := make([]chatMessage, 0, len(req.Messages))
	for _, m := range req.Messages {
		apiMessages = append(apiMessages, chatMessage{Role: m.Role, Content: m.Content})
	}

	apiReq := chatCompletionRequest{
		Model:    req.ModelName,
		Messages: apiMessages,
		Stream:   true,
	}

	var built strings.Builder
	if err := c.stream(ctx, apiReq, func(delta string) error {
		built.WriteString(delta)
		return nil
	}); err != nil {
		return domain.Message{}, err
	}

	return domain.Message{Role: "assistant", Content: strings.TrimSpace(built.String())}, nil
}

// ---- internal HTTP types --------------------------------------------------- //

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatCompletionChunk struct {
	Choices []struct {
		Delta struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"delta"`
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

type apiErrorBody struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

type apiErrorResponse struct {
	Error apiErrorBody `json:"error"`
}

// ---- internal helpers ------------------------------------------------------ //

func (c *Client) stream(ctx context.Context, req chatCompletionRequest, onDelta func(string) error) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("openai-compatible: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("openai-compatible: build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("openai-compatible: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(resp.Body)
		var apiErr apiErrorResponse
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
			return fmt.Errorf("openai-compatible: api error: %s", apiErr.Error.Message)
		}
		return fmt.Errorf("openai-compatible: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return consumeStream(resp.Body, onDelta)
}

func consumeStream(body io.Reader, onDelta func(string) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		delta, done, err := parseStreamLine(scanner.Text())
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if delta == "" {
			continue
		}
		if err := onDelta(delta); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("openai-compatible: read stream: %w", err)
	}
	return nil
}

func parseStreamLine(rawLine string) (delta string, done bool, err error) {
	line := strings.TrimSpace(rawLine)
	if line == "" || strings.HasPrefix(line, ":") {
		return "", false, nil
	}
	if !strings.HasPrefix(line, "data:") {
		return "", false, nil
	}

	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "[DONE]" {
		return "", true, nil
	}

	var chunk chatCompletionChunk
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return "", false, fmt.Errorf("openai-compatible: decode chunk: %w", err)
	}

	var out strings.Builder
	for _, choice := range chunk.Choices {
		content := choice.Delta.Content
		if content == "" {
			content = choice.Message.Content
		}
		if content != "" {
			out.WriteString(content)
		}
	}
	return out.String(), false, nil
}
