package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Henryarrovin/mcp-server/config"
	kafka "github.com/Henryarrovin/mcp-server/kafka_logger_pipeline"
	zaplogger "github.com/Henryarrovin/mcp-server/logger"
	"github.com/Henryarrovin/mcp-server/mcp"
	"github.com/Henryarrovin/mcp-server/tools"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Base logger
	baseLogger, err := zaplogger.New(cfg.Logger.Env)
	if err != nil {
		log.Fatalf("logger error: %v", err)
	}
	defer baseLogger.Sync()

	// Kafka tee logger
	logger, consumerCancel := buildLogger(cfg, baseLogger)
	defer consumerCancel()

	cfg.Print()
	logger.Info("mcp server initializing")

	// Create server
	s := mcp.NewServer("henry-microservices-mcp", "1.0.0")

	// Register tools
	tools.RegisterAuthTools(s, cfg.Auth.BaseURL)
	tools.RegisterPaymentTools(s, cfg.Payment.BaseURL)
	tools.RegisterKubernetesTools(s, cfg.K8s.Namespace)

	logger.Info("tools registered", zap.Int("count", s.ToolCount()))

	// Ollama
	if cfg.OllamaEnabled() {
		ollama := mcp.NewOllamaClient(cfg.Ollama.URL, cfg.Ollama.Model)
		if err := ollama.CheckHealth(); err != nil {
			logger.Warn("ollama unreachable — /chat disabled", zap.Error(err))
		} else {
			s.SetOllama(ollama)
			logger.Info("ollama ready",
				zap.String("url", cfg.Ollama.URL),
				zap.String("model", cfg.Ollama.Model),
			)
		}
	} else {
		logger.Info("ollama disabled — set OLLAMA_URL to enable")
	}

	logger.Info("mcp server starting", zap.String("port", cfg.Server.Port))
	if err := s.Start(":"+cfg.Server.Port, logger); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

// buildLogger creates a tee logger: console + kafka.
func buildLogger(cfg *config.Config, base *zap.Logger) (*zap.Logger, func()) {
	if !cfg.Kafka.Enabled {
		base.Info("kafka logging disabled")
		return base, func() {}
	}

	kafkaCore, err := kafka.NewKafkaCore(
		cfg.Kafka.Brokers,
		cfg.Kafka.Topic,
		zapcore.InfoLevel,
	)
	if err != nil {
		base.Error("kafka connection failed, console only", zap.Error(err))
		return base, func() {}
	}

	// Tee: console + kafka
	logger := zap.New(
		zapcore.NewTee(base.Core(), kafkaCore),
		zap.AddCaller(),
	)

	logger.Info("kafka connected",
		zap.Strings("brokers", cfg.Kafka.Brokers),
		zap.String("topic", cfg.Kafka.Topic),
	)

	// Start consumer — reads from Kafka, writes to disk
	consumer := kafka.NewLogConsumer(
		cfg.Kafka.Brokers,
		cfg.Kafka.Topic,
		cfg.Kafka.GroupID,
		cfg.Kafka.LogDir,
		base,
	)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := consumer.Start(ctx); err != nil {
			base.Error("kafka consumer stopped", zap.Error(err))
		}
	}()

	return logger, cancel
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
