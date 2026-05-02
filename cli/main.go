package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func main() {
	// Load .env from common locations
	for _, path := range []string{
		".env",
		os.Getenv("USERPROFILE") + "\\.mcp.env",
		os.Getenv("HOME") + "/.mcp.env",
	} {
		if err := godotenv.Load(path); err == nil {
			break
		}
	}

	mcpURL := getEnv("MCP_SSE_URL", "http://localhost:8085/sse")
	chatURL := getEnv("MCP_CHAT_URL", "http://localhost:8085/chat")

	clearScreen()
	printBanner(mcpURL)

	fmt.Print("  Connecting to MCP server... ")
	mcpClient, err := NewMCPClient(mcpURL)
	if err != nil {
		fmt.Printf("✗\n\n  Error: %v\n\n", err)
		fmt.Println("  Make sure MCP server is running:")
		fmt.Println("    cd mcp-server && go run .")
		os.Exit(1)
	}
	fmt.Println("✓")

	fmt.Print("  Loading tools... ")
	tools, err := mcpClient.ListTools()
	if err != nil {
		fmt.Printf("✗\n\n  Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ (%d tools)\n", len(tools))

	fmt.Print("  Checking Ollama... ")
	chatClient := NewChatClient(chatURL)
	if err := chatClient.Ping(); err != nil {
		fmt.Printf("✗\n\n  Error: %v\n\n", err)
		fmt.Println("  Make sure OLLAMA_URL is set in mcp-server config")
		os.Exit(1)
	}
	fmt.Println("✓")
	fmt.Println()

	fmt.Println(strings.Repeat("─", 64))
	fmt.Println("  Ready. Type your question below.")
	fmt.Println("  Commands: help · tools · clear · history · exit")
	fmt.Println(strings.Repeat("─", 64))

	sessionID := "default"
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\n\033[1;32m❯\033[0m ")

		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch strings.ToLower(input) {
		case "exit", "quit", "q":
			fmt.Println("\n  Goodbye!")
			return
		case "help":
			printHelp()
			continue
		case "tools":
			printTools(tools)
			continue
		case "clear", "cls":
			clearScreen()
			chatClient.ClearSession(sessionID)
			fmt.Println("  Conversation cleared.")
			continue
		case "history":
			fmt.Println("  (history is managed server-side per session)")
			continue
		}

		fmt.Println()

		// Send to /chat endpoint — Ollama + tools handled server-side
		resp, err := chatClient.Send(sessionID, input, func(toolName string) {
			fmt.Printf("  \033[33m⚙\033[0m %s\n", toolName)
		})

		if err != nil {
			fmt.Printf("  \033[31m✗ Error:\033[0m %v\n", err)
			continue
		}

		fmt.Printf("\n\033[1;34m●\033[0m %s\n", resp)
	}
}

// UI helpers

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func printBanner(mcpURL string) {
	fmt.Println()
	fmt.Println("  \033[1m╔════════════════════════════════════════════╗")
	fmt.Println("  \033[1m║mcp — Ollama + MCP Interactive Client       ║")
	fmt.Println("  \033[1m║auth-service · payment-gateway · kubernetes ║")
	fmt.Println("  \033[1m╚════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("  MCP  →  %s\n", mcpURL)
	fmt.Println()
}

func printHelp() {
	fmt.Println()
	fmt.Println("  \033[1mBuilt-in commands:\033[0m")
	fmt.Println("    help      show this help")
	fmt.Println("    tools     list all available MCP tools")
	fmt.Println("    clear     clear conversation history")
	fmt.Println("    exit      quit")
	fmt.Println()
}

func printTools(tools []MCPTool) {
	fmt.Printf("\n  \033[1m%d tools available:\033[0m\n\n", len(tools))

	groups := []struct {
		prefix string
		label  string
	}{
		{"auth_", "Auth Service"},
		{"payment_", "Payment Gateway"},
		{"k8s_", "Kubernetes"},
	}

	for _, g := range groups {
		fmt.Printf("  \033[1;36m%s\033[0m\n", g.label)
		for _, t := range tools {
			if strings.HasPrefix(t.Name, g.prefix) {
				desc := t.Description
				if len(desc) > 52 {
					desc = desc[:52] + "..."
				}
				fmt.Printf("    \033[33m%-42s\033[0m \033[2m%s\033[0m\n", t.Name, desc)
			}
		}
		fmt.Println()
	}
}

func dimText(s string) string {
	if len(s) > 80 {
		s = s[:80] + "..."
	}
	return "\033[2m" + s + "\033[0m"
}

var _ = dimText
var _ = json.Marshal
