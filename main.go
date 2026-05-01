package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Henryarrovin/mcp-server/mcp"
	"github.com/Henryarrovin/mcp-server/tools"
	"github.com/joho/godotenv"
)

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file (using system env)")
	}

	authBaseURL := getEnv("MCP_AUTH_BASE_URL", "http://auth-service:8080")
	paymentBaseURL := getEnv("MCP_PAYMENT_BASE_URL", "http://payment-gateway-service:8081")
	namespace := getEnv("MCP_K8S_NAMESPACE", "auth")
	port := getEnv("MCP_PORT", "8085")
	ollamaURL := getEnv("OLLAMA_URL", "")
	ollamaModel := getEnv("OLLAMA_MODEL", "llama3.2")

	log.Printf("MCP server starting (pure stdlib)")
	log.Printf("  auth     → %s", authBaseURL)
	log.Printf("  payment  → %s", paymentBaseURL)
	log.Printf("  k8s ns   → %s", namespace)
	log.Printf("  port     → %s", port)

	// Create server
	s := mcp.NewServer("henry-microservices-mcp", "1.0.0")

	// Register tools
	tools.RegisterAuthTools(s, authBaseURL)
	tools.RegisterPaymentTools(s, paymentBaseURL)
	tools.RegisterKubernetesTools(s, namespace)

	log.Printf("  tools    → %d registered", s.ToolCount())

	if ollamaURL != "" {
		ollama := mcp.NewOllamaClient(ollamaURL, ollamaModel)
		if err := ollama.CheckHealth(); err != nil {
			log.Printf("  ollama   → unreachable (%v) — /chat endpoint disabled", err)
		} else {
			s.SetOllama(ollama)
			log.Printf("  ollama   → %s  model: %s", ollamaURL, ollamaModel)
		}
	} else {
		log.Printf("  ollama   → not configured (set OLLAMA_URL to enable /chat)")
	}

	if err := s.Start(":" + port); err != nil {
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
