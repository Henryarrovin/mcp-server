package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Server  ServerConfig
	Auth    ServiceConfig
	Payment ServiceConfig
	K8s     K8sConfig
	Ollama  OllamaConfig
	Kafka   KafkaConfig
	Logger  LoggerConfig
}

type ServerConfig struct {
	Port string
}

type ServiceConfig struct {
	BaseURL string
}

type K8sConfig struct {
	Namespace string
}

type OllamaConfig struct {
	URL   string
	Model string
}

type KafkaConfig struct {
	Enabled bool
	Brokers []string
	Topic   string
	GroupID string
	LogDir  string
}

type LoggerConfig struct {
	Env string
}

func Load() (*Config, error) {
	for _, path := range []string{
		".env",
		"/.env",
		os.Getenv("HOME") + "/.mcp.env",
	} {
		if err := godotenv.Load(path); err == nil {
			break
		}
	}

	cfg := &Config{
		Server: ServerConfig{
			Port: getEnv("MCP_PORT", "8085"),
		},
		Auth: ServiceConfig{
			BaseURL: getEnv("MCP_AUTH_BASE_URL", "http://auth-service:8080"),
		},
		Payment: ServiceConfig{
			BaseURL: getEnv("MCP_PAYMENT_BASE_URL", "http://payment-gateway-service:8081"),
		},
		K8s: K8sConfig{
			Namespace: getEnv("MCP_K8S_NAMESPACE", "auth"),
		},
		Ollama: OllamaConfig{
			URL:   getEnv("OLLAMA_URL", ""),
			Model: getEnv("OLLAMA_MODEL", "llama3.2"),
		},
		Kafka: KafkaConfig{
			Enabled: getEnv("MCP_KAFKA_ENABLED", "false") == "true",
			Brokers: strings.Split(getEnv("MCP_KAFKA_BROKERS", "localhost:9092"), ","),
			Topic:   getEnv("MCP_KAFKA_TOPIC", "mcp-server-logs"),
			GroupID: getEnv("MCP_KAFKA_GROUP_ID", "mcp-log-consumer"),
			LogDir:  getEnv("MCP_KAFKA_LOG_DIR", "/apps/logs"),
		},
		Logger: LoggerConfig{
			Env: getEnv("MCP_SERVER_ENV", "development"),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.Server.Port == "" {
		return fmt.Errorf("MCP_PORT is required")
	}
	if c.Auth.BaseURL == "" {
		return fmt.Errorf("MCP_AUTH_BASE_URL is required")
	}
	if c.Payment.BaseURL == "" {
		return fmt.Errorf("MCP_PAYMENT_BASE_URL is required")
	}
	return nil
}

func (c *Config) Print() {
	fmt.Println("  Configuration:")
	fmt.Printf("    auth        → %s\n", c.Auth.BaseURL)
	fmt.Printf("    payment     → %s\n", c.Payment.BaseURL)
	fmt.Printf("    k8s ns      → %s\n", c.K8s.Namespace)
	fmt.Printf("    port        → %s\n", c.Server.Port)
	if c.Ollama.URL != "" {
		fmt.Printf("    ollama      → %s  (model: %s)\n", c.Ollama.URL, c.Ollama.Model)
	} else {
		fmt.Printf("    ollama      → disabled\n")
	}
}

func (c *Config) OllamaEnabled() bool {
	return c.Ollama.URL != ""
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}
