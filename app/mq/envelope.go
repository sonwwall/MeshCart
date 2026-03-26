package mq

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidEnvelope = errors.New("invalid mq envelope")

type Envelope struct {
	ID         string            `json:"id"`
	EventName  string            `json:"event_name"`
	Topic      string            `json:"topic"`
	Key        string            `json:"key"`
	Producer   string            `json:"producer"`
	Version    int               `json:"version"`
	OccurredAt time.Time         `json:"occurred_at"`
	Headers    map[string]string `json:"headers,omitempty"`
	Payload    json.RawMessage   `json:"payload"`
}

func (e Envelope) Validate() error {
	if strings.TrimSpace(e.ID) == "" ||
		strings.TrimSpace(e.EventName) == "" ||
		strings.TrimSpace(e.Topic) == "" ||
		strings.TrimSpace(e.Producer) == "" ||
		e.Version <= 0 ||
		e.OccurredAt.IsZero() ||
		len(e.Payload) == 0 {
		return ErrInvalidEnvelope
	}
	return nil
}

func MarshalEnvelope(e Envelope) ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(e)
}
