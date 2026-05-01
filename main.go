package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Henryarrovin/mcp-server/config"
	"github.com/Henryarrovin/mcp-server/mcp"
	"github.com/Henryarrovin/mcp-server/tools"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	cfg.Print()

	// Create server
	s := mcp.NewServer("henry-microservices-mcp", "1.0.0")

	// Register tools
	tools.RegisterAuthTools(s, cfg.Auth.BaseURL)
	tools.RegisterPaymentTools(s, cfg.Payment.BaseURL)
	tools.RegisterKubernetesTools(s, cfg.K8s.Namespace)

	log.Printf("  tools       → %d registered", s.ToolCount())

	if cfg.OllamaEnabled() {
		ollama := mcp.NewOllamaClient(cfg.Ollama.URL, cfg.Ollama.Model)
		if err := ollama.CheckHealth(); err != nil {
			log.Printf("  ollama      → unreachable (%v) — /chat disabled", err)
		} else {
			s.SetOllama(ollama)
			log.Printf("  ollama   → %s  model: %s    -> ready", cfg.Ollama.URL, cfg.Ollama.Model)
		}
	} else {
		log.Printf("  ollama      → disabled (set OLLAMA_URL to enable)")
	}

	if err := s.Start(":" + cfg.Server.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func init() {
	lines := []string{
		"╔══════════════════════════════════════════╗",
		"║   MCP Server          					║",
		"║   JSON-RPC over HTTP + SSE           	║",
		"║   + Ollama /chat endpoint                ║",
		"╚══════════════════════════════════════════╝",
	}
	fmt.Println()
	for _, l := range lines {
		fmt.Println("  " + l)
	}
	fmt.Println()

	if _, err := os.Stat("/usr/local/bin/kubectl"); err != nil {
		if !strings.Contains(os.Getenv("PATH"), "kubectl") {
			fmt.Println("  Warning: kubectl not found — k8s tools will fail")
		}
	}
}
