package kafka

import (
	"fmt"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type KafkaCore struct {
	producer sarama.SyncProducer
	topic    string
	level    zapcore.Level
	enc      zapcore.Encoder
}

func NewKafkaCore(brokers []string, topic string, level zapcore.Level) (*KafkaCore, error) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Retry.Max = 3

	// Auto-create topic if it doesn't exist
	admin, err := sarama.NewClusterAdmin(brokers, cfg)
	if err == nil {
		_ = admin.CreateTopic(topic, &sarama.TopicDetail{
			NumPartitions:     1,
			ReplicationFactor: 1,
		}, false)
		admin.Close()
	}

	producer, err := sarama.NewSyncProducer(brokers, cfg)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "timestamp"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	return &KafkaCore{
		producer: producer,
		topic:    topic,
		level:    level,
		enc:      zapcore.NewJSONEncoder(encCfg),
	}, nil
}

func (k *KafkaCore) Enabled(level zapcore.Level) bool {
	return level >= k.level
}

func (k *KafkaCore) With(fields []zapcore.Field) zapcore.Core {
	clone := *k
	enc := k.enc.Clone()
	for _, f := range fields {
		f.AddTo(enc)
	}
	clone.enc = enc
	return &clone
}

func (k *KafkaCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if k.Enabled(entry.Level) {
		return ce.AddCore(entry, k)
	}
	return ce
}

func (k *KafkaCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	buf, err := k.enc.EncodeEntry(entry, fields)
	if err != nil {
		return fmt.Errorf("encode log entry: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: k.topic,
		Key:   sarama.StringEncoder(time.Now().UTC().Format("2006-01-02")),
		Value: sarama.ByteEncoder(buf.Bytes()),
	}

	_, _, err = k.producer.SendMessage(msg)
	return err
}

func (k *KafkaCore) Sync() error {
	return k.producer.Close()
}
