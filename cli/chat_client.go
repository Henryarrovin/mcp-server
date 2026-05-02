package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
		client:  &http.Client{Timeout: 0},
	}
}

func (c *ChatClient) Ping() error {
	// check if the MCP server /health endpoint is reachable
	healthURL := strings.Replace(c.baseURL, "/chat", "/health", 1)

	pingClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := pingClient.Get(healthURL)
	if err != nil {
		return fmt.Errorf("chat endpoint unreachable: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)

	var result struct {
		Status string `json:"status"`
		Ollama string `json:"ollama"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse health response: %w", err)
	}

	if result.Status != "ok" {
		return fmt.Errorf("server unhealthy")
	}
	if result.Ollama == "unreachable" {
		return fmt.Errorf("ollama unreachable — make sure ollama is running and OLLAMA_URL is set")
	}
	if result.Ollama == "disabled" {
		return fmt.Errorf("ollama not configured — set OLLAMA_URL in mcp-server .env")
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

	// Show spinner while waiting — Ollama can be slow on CPU
	done := make(chan struct{})
	go spinner(done)

	resp, err := c.client.Post(c.baseURL, "application/json", bytes.NewBuffer(body))
	close(done)             // stop spinner
	fmt.Print("\r  \033[K") // clear spinner line

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

// spinner shows a rotating indicator while waiting for Ollama
func spinner(done chan struct{}) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-done:
			return
		default:
			fmt.Printf("\r  \033[33m%s\033[0m thinking...", frames[i%len(frames)])
			i++
			time.Sleep(100 * time.Millisecond)
		}
	}
}
