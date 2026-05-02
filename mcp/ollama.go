package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
		client:  &http.Client{Timeout: 300 * time.Second},
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
// Handles both streaming and non-streaming responses automatically.
// When Ollama decides to call a tool, executor is called to run it.
// Loops until Ollama returns a final text answer with no more tool calls.
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

		// Ollama may stream even with stream:false on some versions.
		// We read line by line:
		//   - streaming:     many lines with done=false, last line done=true
		//   - non-streaming: single line with done=true
		// Content is accumulated across chunks for streaming.
		// Tool calls only appear in the done=true line.
		var finalResp OllamaChatResponse
		var accumulatedContent strings.Builder

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			var chunk OllamaChatResponse
			if err := json.Unmarshal([]byte(line), &chunk); err != nil {
				continue
			}

			// Accumulate streamed text content
			if chunk.Message.Content != "" {
				accumulatedContent.WriteString(chunk.Message.Content)
			}

			// Capture tool calls whenever they appear
			if len(chunk.Message.ToolCalls) > 0 {
				finalResp = chunk
			}

			if chunk.Done {
				// Only use chunk as finalResp if no tool calls captured yet
				if len(finalResp.Message.ToolCalls) == 0 {
					finalResp = chunk
					// Use accumulated content for streaming responses
					if finalResp.Message.Content == "" {
						finalResp.Message.Content = accumulatedContent.String()
					}
				}
				break
			}
		}
		resp.Body.Close()

		if finalResp.Message.Content == "" && len(finalResp.Message.ToolCalls) == 0 {
			return "", history, fmt.Errorf("empty response from ollama — model may not support tools, try: ollama pull llama3.2")
		}

		// Add assistant message to conversation history
		messages = append(messages, finalResp.Message)

		// No tool calls → Ollama gave final answer
		if len(finalResp.Message.ToolCalls) == 0 {
			return finalResp.Message.Content, messages, nil
		}

		// Ollama wants to call tools — execute each one
		for _, tc := range finalResp.Message.ToolCalls {
			name := tc.Function.Name
			args := tc.Function.Arguments

			if onToolCall != nil {
				onToolCall(name, args)
			}

			result, err := executor(name, args)
			if err != nil {
				result = fmt.Sprintf("tool error: %v", err)
			}

			// Add tool result back into conversation
			messages = append(messages, OllamaMessage{
				Role:    "tool",
				Content: result,
			})
		}
	}
}

// toOllamaTools converts MCP tool definitions to Ollama's expected format.
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
