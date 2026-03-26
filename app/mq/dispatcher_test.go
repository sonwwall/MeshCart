package mq

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	messages     []OutboxMessage
	claimed      bool
	publishedIDs []int64
	failedCalls  []struct {
		id         int64
		retryCount int
	}
}

func (s *fakeStore) ClaimPending(context.Context, time.Time, int) ([]OutboxMessage, error) {
	s.claimed = true
	return s.messages, nil
}

func (s *fakeStore) MarkPublished(_ context.Context, id int64, _ time.Time) error {
	s.publishedIDs = append(s.publishedIDs, id)
	return nil
}

func (s *fakeStore) MarkFailed(_ context.Context, id int64, retryCount int, _ time.Time, _ string) error {
	s.failedCalls = append(s.failedCalls, struct {
		id         int64
		retryCount int
	}{id: id, retryCount: retryCount})
	return nil
}

type fakePublisher struct {
	fail map[int64]error
}

func (p *fakePublisher) Publish(_ context.Context, _ string, key string, _ []byte, _ map[string]string) error {
	if p.fail == nil {
		return nil
	}
	for id, err := range p.fail {
		if key == string(rune(id)) {
			return err
		}
	}
	return nil
}

func TestMarshalEnvelope(t *testing.T) {
	body, err := MarshalEnvelope(Envelope{
		ID:         "evt-1",
		EventName:  "order.created",
		Topic:      "order.events",
		Key:        "order:1",
		Producer:   "order-service",
		Version:    1,
		OccurredAt: time.Unix(1700000000, 0),
		Payload:    []byte(`{"order_id":1}`),
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected envelope body")
	}
}

func TestDispatcherRunOnce(t *testing.T) {
	store := &fakeStore{
		messages: []OutboxMessage{
			{ID: 1, Topic: "order.events", EventKey: string(rune(1)), Body: []byte(`{}`), RetryCount: 0, MaxRetries: 3},
			{ID: 2, Topic: "order.events", EventKey: string(rune(2)), Body: []byte(`{}`), RetryCount: 1, MaxRetries: 3},
		},
	}
	publisher := &fakePublisher{fail: map[int64]error{2: errors.New("publish failed")}}
	dispatcher := NewDispatcher(store, publisher, 10, time.Second)

	processed, err := dispatcher.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run dispatcher once: %v", err)
	}
	if processed != 2 {
		t.Fatalf("expected 2 processed messages, got %d", processed)
	}
	if len(store.publishedIDs) != 1 || store.publishedIDs[0] != 1 {
		t.Fatalf("unexpected published ids: %+v", store.publishedIDs)
	}
	if len(store.failedCalls) != 1 || store.failedCalls[0].id != 2 || store.failedCalls[0].retryCount != 2 {
		t.Fatalf("unexpected failed calls: %+v", store.failedCalls)
	}
}
