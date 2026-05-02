package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ChatClient calls the MCP server's /chat endpoint
type ChatClient struct {
	baseURL string
	client  *http.Client
}

type chatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type chatResponse struct {
	SessionID   string   `json:"session_id"`
	Response    string   `json:"response"`
	ToolsCalled []string `json:"tools_called"`
	Error       string   `json:"error,omitempty"`
}

func NewChatClient(baseURL string) *ChatClient {
	return &ChatClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 300 * time.Second},
	}
}

// Ping checks if the /chat endpoint is available and Ollama is configured.
func (c *ChatClient) Ping() error {
	req := chatRequest{SessionID: "ping", Message: "ping"}
	body, _ := json.Marshal(req)

	resp, err := c.client.Post(c.baseURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("chat endpoint unreachable: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var chatResp chatResponse
	if err := json.Unmarshal(data, &chatResp); err != nil {
		return fmt.Errorf("parse ping response: %w", err)
	}
	if chatResp.Error != "" && chatResp.Error != "ollama not configured" {
		// Any error other than "not configured" means server is up
		return nil
	}
	if chatResp.Error == "ollama not configured — set OLLAMA_URL env var" {
		return fmt.Errorf("ollama not configured in mcp-server — set OLLAMA_URL")
	}
	return nil
}

// Send sends a message to the /chat endpoint and returns the response.
// onToolCall is called each time Ollama uses a tool — for live feedback.
func (c *ChatClient) Send(
	sessionID string,
	message string,
	onToolCall func(toolName string),
) (string, error) {
	req := chatRequest{
		SessionID: sessionID,
		Message:   message,
	}
	body, _ := json.Marshal(req)

	resp, err := c.client.Post(c.baseURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("chat request failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var chatResp chatResponse
	if err := json.Unmarshal(data, &chatResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if chatResp.Error != "" {
		return "", fmt.Errorf("%s", chatResp.Error)
	}

	// Notify which tools were called
	if onToolCall != nil {
		for _, tool := range chatResp.ToolsCalled {
			onToolCall(tool)
		}
	}

	return chatResp.Response, nil
}

// ClearSession clears the conversation history for a session.
func (c *ChatClient) ClearSession(sessionID string) {
	clearURL := fmt.Sprintf("%s/../chat/clear?session_id=%s", c.baseURL, sessionID)
	req, err := http.NewRequest(http.MethodPost, clearURL, nil)
	if err != nil {
		return
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}
