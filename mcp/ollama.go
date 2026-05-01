package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type OllamaClient struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewOllamaClient(baseURL, model string) *OllamaClient {
	return &OllamaClient{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// CheckHealth verifies Ollama is running and the model is pulled.
func (o *OllamaClient) CheckHealth() error {
	resp, err := o.client.Get(o.baseURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("ollama unreachable at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("parse tags: %w", err)
	}

	for _, m := range result.Models {
		if m.Name == o.model || m.Name == o.model+":latest" {
			return nil
		}
	}

	return fmt.Errorf("model %q not found — run: ollama pull %s", o.model, o.model)
}

// Chat sends a prompt with tool definitions to Ollama.
// When Ollama calls a tool, executor is called to run it.
// Loops until Ollama returns a final text answer.
func (o *OllamaClient) Chat(
	prompt string,
	history []OllamaMessage,
	tools []Tool,
	executor func(name string, args map[string]interface{}) (string, error),
	onToolCall func(name string, args map[string]interface{}),
) (string, []OllamaMessage, error) {

	ollamaTools := toOllamaTools(tools)

	messages := append(history, OllamaMessage{
		Role:    "user",
		Content: prompt,
	})

	for {
		body, err := json.Marshal(OllamaChatRequest{
			Model:    o.model,
			Messages: messages,
			Tools:    ollamaTools,
			Stream:   false,
			Options: OllamaOptions{
				Temperature: 0.1,
				NumCtx:      4096,
			},
		})
		if err != nil {
			return "", history, fmt.Errorf("marshal: %w", err)
		}

		resp, err := o.client.Post(o.baseURL+"/api/chat", "application/json", bytes.NewBuffer(body))
		if err != nil {
			return "", history, fmt.Errorf("ollama chat: %w", err)
		}
		defer resp.Body.Close()

		data, _ := io.ReadAll(resp.Body)

		var chatResp OllamaChatResponse
		if err := json.Unmarshal(data, &chatResp); err != nil {
			return "", history, fmt.Errorf("parse response: %w", err)
		}

		messages = append(messages, chatResp.Message)

		// No tool calls — final answer
		if len(chatResp.Message.ToolCalls) == 0 {
			return chatResp.Message.Content, messages, nil
		}

		// Execute each tool call
		for _, tc := range chatResp.Message.ToolCalls {
			name := tc.Function.Name
			args := tc.Function.Arguments

			if onToolCall != nil {
				onToolCall(name, args)
			}

			result, err := executor(name, args)
			if err != nil {
				result = fmt.Sprintf("tool error: %v", err)
			}

			// Add tool result back to conversation
			messages = append(messages, OllamaMessage{
				Role:    "tool",
				Content: result,
			})
		}
	}
}

// converts MCP tool definitions to Ollama's format.
func toOllamaTools(tools []Tool) []OllamaTool {
	var out []OllamaTool
	for _, t := range tools {
		props := make(map[string]any)
		for name, p := range t.InputSchema.Properties {
			props[name] = map[string]string{
				"type":        p.Type,
				"description": p.Description,
			}
		}

		params := map[string]any{
			"type":       "object",
			"properties": props,
		}
		if len(t.InputSchema.Required) > 0 {
			params["required"] = t.InputSchema.Required
		}

		out = append(out, OllamaTool{
			Type: "function",
			Function: OllamaToolFn{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return out
}
