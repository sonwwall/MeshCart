package mq

import (
	"context"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaPublisher struct {
	writer *kafka.Writer
}

func NewKafkaPublisher(brokers []string) *KafkaPublisher {
	cleaned := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			cleaned = append(cleaned, broker)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}

	return &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(cleaned...),
			Balancer:     &kafka.Hash{},
			BatchTimeout: 10 * time.Millisecond,
			RequiredAcks: kafka.RequireOne,
		},
	}
}

func (p *KafkaPublisher) Publish(ctx context.Context, topic, key string, body []byte, headers map[string]string) error {
	if p == nil || p.writer == nil {
		return nil
	}
	messageHeaders := make([]kafka.Header, 0, len(headers))
	for k, v := range headers {
		messageHeaders = append(messageHeaders, kafka.Header{Key: k, Value: []byte(v)})
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Topic:   topic,
		Key:     []byte(key),
		Value:   body,
		Headers: messageHeaders,
	})
}

func (p *KafkaPublisher) Close() error {
	if p == nil || p.writer == nil {
		return nil
	}
	return p.writer.Close()
}
