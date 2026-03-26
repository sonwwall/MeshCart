package mq

import (
	"context"
	"errors"
	"strings"
	"time"
)

type Publisher interface {
	Publish(ctx context.Context, topic, key string, body []byte, headers map[string]string) error
}

type Store interface {
	ClaimPending(ctx context.Context, now time.Time, limit int) ([]OutboxMessage, error)
	MarkPublished(ctx context.Context, id int64, publishedAt time.Time) error
	MarkFailed(ctx context.Context, id int64, retryCount int, nextRetryAt time.Time, errMsg string) error
}

type Dispatcher struct {
	store        Store
	publisher    Publisher
	nowFunc      func() time.Time
	batchSize    int
	retryBackoff time.Duration
}

func NewDispatcher(store Store, publisher Publisher, batchSize int, retryBackoff time.Duration) *Dispatcher {
	if batchSize <= 0 {
		batchSize = 100
	}
	if retryBackoff <= 0 {
		retryBackoff = 5 * time.Second
	}
	return &Dispatcher{
		store:        store,
		publisher:    publisher,
		nowFunc:      time.Now,
		batchSize:    batchSize,
		retryBackoff: retryBackoff,
	}
}

func (d *Dispatcher) RunOnce(ctx context.Context) (int, error) {
	if d == nil || d.store == nil || d.publisher == nil {
		return 0, errors.New("mq dispatcher is not initialized")
	}

	now := d.nowFunc()
	messages, err := d.store.ClaimPending(ctx, now, d.batchSize)
	if err != nil {
		return 0, err
	}

	processed := 0
	for _, message := range messages {
		if err := d.publisher.Publish(ctx, message.Topic, message.EventKey, message.Body, message.Headers); err != nil {
			processed++
			nextRetryAt := now.Add(d.retryBackoff)
			if markErr := d.store.MarkFailed(ctx, message.ID, message.RetryCount+1, nextRetryAt, truncateError(err)); markErr != nil {
				return processed, markErr
			}
			continue
		}
		processed++
		if err := d.store.MarkPublished(ctx, message.ID, now); err != nil {
			return processed, err
		}
	}

	return processed, nil
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	message := strings.TrimSpace(err.Error())
	if len(message) <= 255 {
		return message
	}
	return message[:255]
}
