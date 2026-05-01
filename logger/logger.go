package logger

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(env string) (*zap.Logger, error) {
	var cfg zap.Config

	if env == "production" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}

	cfg.EncoderConfig.CallerKey = "caller"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger, err := cfg.Build(zap.AddCaller())
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}

	return logger, nil
}

// Server / Docker:  /apps/logs/mcp-server/
// Linux/Mac dev:    ~/Desktop/logs/mcp-server/
// Windows dev:      C:\Users\<user>\Desktop\logs\mcp-server\
func ResolveLogDir(baseDir string) string {
	const serviceName = "mcp-server"

	// Docker / k8s
	if isDocker() {
		dir := fmt.Sprintf("%s/%s", baseDir, serviceName)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fallbackDir(serviceName)
		}
		return dir
	}

	// Local dev — Desktop/logs/mcp-server
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fallbackDir(serviceName)
	}

	desktopDir := fmt.Sprintf("%s/Desktop/logs/%s", homeDir, serviceName)
	if err := os.MkdirAll(desktopDir, 0755); err != nil {
		return fallbackDir(serviceName)
	}
	return desktopDir
}

func isDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

func fallbackDir(service string) string {
	dir := fmt.Sprintf("./logs/%s", service)
	_ = os.MkdirAll(dir, 0755)
	return dir
}

func IsProduction(env string) bool {
	return strings.ToLower(env) == "production"
}
