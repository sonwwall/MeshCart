package mq

import (
	"strings"

	"github.com/segmentio/kafka-go"
)

func NewKafkaReader(brokers []string, groupID, topic string) *kafka.Reader {
	cleaned := make([]string, 0, len(brokers))
	for _, broker := range brokers {
		broker = strings.TrimSpace(broker)
		if broker != "" {
			cleaned = append(cleaned, broker)
		}
	}
	if len(cleaned) == 0 || strings.TrimSpace(groupID) == "" || strings.TrimSpace(topic) == "" {
		return nil
	}

	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  cleaned,
		GroupID:  strings.TrimSpace(groupID),
		Topic:    strings.TrimSpace(topic),
		MinBytes: 1,
		MaxBytes: 10e6,
	})
}
