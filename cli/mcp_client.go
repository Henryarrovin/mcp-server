package main

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

type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type MCPRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type MCPResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      int            `json:"id"`
	Result  map[string]any `json:"result"`
	Error   *MCPError      `json:"error"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type MCPClient struct {
	sseURL     string
	messageURL string
	sessionID  string
	client     *http.Client
	msgID      int
}

func NewMCPClient(sseURL string) (*MCPClient, error) {
	c := &MCPClient{
		sseURL: sseURL,
		client: &http.Client{Timeout: 30 * time.Second},
	}
	if err := c.connect(); err != nil {
		return nil, fmt.Errorf("SSE connect: %w", err)
	}
	if err := c.initialize(); err != nil {
		return nil, fmt.Errorf("initialize: %w", err)
	}
	return c, nil
}

func (c *MCPClient) connect() error {
	sseClient := &http.Client{}
	resp, err := sseClient.Get(c.sseURL)
	if err != nil {
		return fmt.Errorf("GET SSE failed: %w", err)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			if strings.Contains(data, "sessionId=") {
				parts := strings.Split(data, "sessionId=")
				if len(parts) == 2 {
					c.sessionID = strings.TrimSpace(parts[1])
					base := extractBase(c.sseURL)
					c.messageURL = base + data
					resp.Body.Close()
					return nil
				}
			}
		}
	}
	resp.Body.Close()
	return fmt.Errorf("could not get sessionId from SSE")
}

func extractBase(url string) string {
	parts := strings.Split(url, "/sse")
	if len(parts) > 0 {
		return parts[0]
	}
	return url
}

func (c *MCPClient) sendMessage(method string, params any) (*MCPResponse, error) {
	c.msgID++
	payload := MCPRequest{
		JSONRPC: "2.0",
		ID:      c.msgID,
		Method:  method,
		Params:  params,
	}
	body, _ := json.Marshal(payload)

	resp, err := c.client.Post(c.messageURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("POST failed: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var mcpResp MCPResponse
	if err := json.Unmarshal(data, &mcpResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if mcpResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}
	return &mcpResp, nil
}

func (c *MCPClient) initialize() error {
	_, err := c.sendMessage("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "mcp-cli", "version": "1.0.0"},
	})
	return err
}

func (c *MCPClient) ListTools() ([]MCPTool, error) {
	resp, err := c.sendMessage("tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	toolsRaw, ok := resp.Result["tools"].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid tools response")
	}
	var tools []MCPTool
	for _, t := range toolsRaw {
		toolMap, ok := t.(map[string]any)
		if !ok {
			continue
		}
		tool := MCPTool{
			Name:        fmt.Sprintf("%v", toolMap["name"]),
			Description: fmt.Sprintf("%v", toolMap["description"]),
		}
		if schema, ok := toolMap["inputSchema"].(map[string]any); ok {
			tool.InputSchema = schema
		}
		tools = append(tools, tool)
	}
	return tools, nil
}
