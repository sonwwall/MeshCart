package mq

import "time"

const (
	OutboxStatusPending    = "pending"
	OutboxStatusPublishing = "publishing"
	OutboxStatusPublished  = "published"
	OutboxStatusFailed     = "failed"
)

type OutboxMessage struct {
	ID         int64
	Topic      string
	EventName  string
	EventKey   string
	Producer   string
	Headers    map[string]string
	Body       []byte
	RetryCount int
	MaxRetries int
}

type OutboxRecord struct {
	ID          int64
	Topic       string
	EventName   string
	EventKey    string
	Producer    string
	HeadersJSON []byte
	Body        []byte
	Status      string
	RetryCount  int
	MaxRetries  int
	NextRetryAt *time.Time
	LastError   string
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
