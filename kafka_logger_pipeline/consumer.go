package kafka

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

const (
	serviceName    = "mcp-server"
	dockerLogBase  = "/apps/logs"
	dockerFlagFile = "/.dockerenv"
)

type LogConsumer struct {
	brokers []string
	topic   string
	groupID string
	logDir  string
	logger  *zap.Logger
	files   map[string]*os.File
	mu      sync.Mutex
}

func NewLogConsumer(brokers []string, topic, groupID, logDir string, logger *zap.Logger) *LogConsumer {
	resolved := resolveLogDir(logDir, logger)
	return &LogConsumer{
		brokers: brokers,
		topic:   topic,
		groupID: groupID,
		logDir:  resolved,
		logger:  logger,
		files:   make(map[string]*os.File),
	}
}

func resolveLogDir(baseDir string, logger *zap.Logger) string {
	// Docker / k8s
	if _, err := os.Stat(dockerFlagFile); err == nil {
		dir := filepath.Join(dockerLogBase, serviceName)
		if err := os.MkdirAll(dir, 0755); err != nil {
			logger.Warn("could not create server log dir", zap.Error(err))
			return ensureDir("./logs/"+serviceName, logger)
		}
		logger.Info("server environment", zap.String("log_dir", dir))
		return dir
	}

	// Local dev
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ensureDir("./logs/"+serviceName, logger)
	}

	dir := filepath.Join(homeDir, "Desktop", "logs", serviceName)
	logger.Info("local dev environment", zap.String("log_dir", dir))
	return ensureDir(dir, logger)
}

func ensureDir(dir string, logger *zap.Logger) string {
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Warn("could not create log dir, using ./logs",
			zap.String("path", dir), zap.Error(err))
		fallback := "./logs/" + serviceName
		_ = os.MkdirAll(fallback, 0755)
		return fallback
	}
	logger.Info("log dir ready", zap.String("log_dir", dir))
	return dir
}

func (c *LogConsumer) Start(ctx context.Context) error {
	cfg := sarama.NewConfig()
	cfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{
		sarama.NewBalanceStrategyRoundRobin(),
	}
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	group, err := sarama.NewConsumerGroup(c.brokers, c.groupID, cfg)
	if err != nil {
		return fmt.Errorf("create consumer group: %w", err)
	}
	defer group.Close()

	c.logger.Info("kafka log consumer started",
		zap.Strings("brokers", c.brokers),
		zap.String("topic", c.topic),
		zap.String("log_dir", c.logDir),
	)

	handler := &consumerHandler{consumer: c}

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("kafka consumer shutting down")
			c.closeAllFiles()
			return nil
		default:
			if err := group.Consume(ctx, []string{c.topic}, handler); err != nil {
				c.logger.Error("consumer group error", zap.Error(err))
			}
		}
	}
}

func (c *LogConsumer) getFile(date string) (*os.File, error) {
	if f, ok := c.files[date]; ok {
		return f, nil
	}

	path := filepath.Join(c.logDir, fmt.Sprintf("log-%s.log", date))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file %s: %w", path, err)
	}

	c.logger.Info("opened log file", zap.String("file", path))
	c.files[date] = f
	return f, nil
}

func (c *LogConsumer) writeLog(date, message string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	f, err := c.getFile(date)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(f, message)
	return err
}

func (c *LogConsumer) closeAllFiles() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for date, f := range c.files {
		c.logger.Info("closing log file", zap.String("date", date))
		f.Close()
		delete(c.files, date)
	}
}

type consumerHandler struct {
	consumer *LogConsumer
}

func (h *consumerHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *consumerHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *consumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		date := string(msg.Key)
		if date == "" {
			date = time.Now().UTC().Format("2006-01-02")
		}
		if err := h.consumer.writeLog(date, string(msg.Value)); err != nil {
			h.consumer.logger.Error("write log failed",
				zap.String("date", date),
				zap.Error(err),
			)
		}
		session.MarkMessage(msg, "")
	}
	return nil
}
