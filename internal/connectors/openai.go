package connectors

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
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	defaultTimeout = 60 * time.Second
)

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	Stream      bool          `json:"stream"`
}

type ChatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type ChatCompletionChunk struct {
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"delta"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    any    `json:"code"`
}

type APIErrorResponse struct {
	APIError APIError `json:"error"`
}

func (e APIErrorResponse) Error() string {
	if e.APIError.Message == "" {
		return "openai api error"
	}
	return e.APIError.Message
}

type OpenAIClientConfig struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

type OpenAIClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewOpenAIClient(cfg OpenAIClientConfig) (*OpenAIClient, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("openai api key is required")
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

	return &OpenAIClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: httpClient,
	}, nil
}

func (c *OpenAIClient) CreateChatCompletion(ctx context.Context, req ChatCompletionRequest) (*ChatCompletionResponse, error) {
	ctx, err := c.prepareRequestContextAndValidate(ctx, req)
	if err != nil {
		return nil, err
	}
	if req.Stream {
		var built strings.Builder
		err := c.StreamChatCompletion(ctx, req, func(delta string) error {
			built.WriteString(delta)
			return nil
		})
		if err != nil {
			return nil, err
		}
		return buildResponseFromStream(built.String()), nil
	}

	httpResp, err := c.doChatCompletionRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()
	return decodeChatCompletionResponse(httpResp.Body)
}

func (c *OpenAIClient) StreamChatCompletion(ctx context.Context, req ChatCompletionRequest, onDelta func(delta string) error) error {
	ctx, err := c.prepareRequestContextAndValidate(ctx, req)
	if err != nil {
		return err
	}
	if onDelta == nil {
		return errors.New("onDelta callback is required")
	}

	req.Stream = true
	httpResp, err := c.doChatCompletionRequest(ctx, req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	return consumeStream(httpResp.Body, onDelta)
}

func (c *OpenAIClient) prepareRequestContextAndValidate(ctx context.Context, req ChatCompletionRequest) (context.Context, error) {
	if c == nil {
		return nil, errors.New("openai client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(req.Model) == "" {
		return nil, errors.New("chat completion model is required")
	}
	if len(req.Messages) == 0 {
		return nil, errors.New("at least one message is required")
	}
	return ctx, nil
}

func buildResponseFromStream(content string) *ChatCompletionResponse {
	response := &ChatCompletionResponse{}
	response.Choices = append(response.Choices, struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	}{
		Index:        0,
		Message:      ChatMessage{Role: "assistant", Content: content},
		FinishReason: "stop",
	})
	return response
}

func decodeChatCompletionResponse(body io.Reader) (*ChatCompletionResponse, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("read chat completion response: %w", err)
	}

	var parsed ChatCompletionResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("decode chat completion response: %w", err)
	}

	return &parsed, nil
}

func consumeStream(body io.Reader, onDelta func(delta string) error) error {
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
		return fmt.Errorf("read stream: %w", err)
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

	var chunk ChatCompletionChunk
	if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
		return "", false, fmt.Errorf("decode stream chunk: %w", err)
	}

	var built strings.Builder
	for _, choice := range chunk.Choices {
		content := choice.Delta.Content
		if content == "" {
			content = choice.Message.Content
		}
		if content != "" {
			built.WriteString(content)
		}
	}

	return built.String(), false, nil
}

func (c *OpenAIClient) doChatCompletionRequest(ctx context.Context, req ChatCompletionRequest) (*http.Response, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal chat completion request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build chat completion request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("chat completion request failed: %w", err)
	}

	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		defer httpResp.Body.Close()
		body, readErr := io.ReadAll(httpResp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("read error response body: %w", readErr)
		}

		var apiErr APIErrorResponse
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.APIError.Message != "" {
			return nil, apiErr
		}
		return nil, fmt.Errorf("openai api returned status %d: %s", httpResp.StatusCode, strings.TrimSpace(string(body)))
	}

	return httpResp, nil
}
