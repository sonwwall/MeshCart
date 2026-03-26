package mq

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type GormStore struct {
	db        *gorm.DB
	tableName string
}

func NewGormStore(db *gorm.DB, tableName string) *GormStore {
	return &GormStore{db: db, tableName: tableName}
}

func (s *GormStore) ClaimPending(ctx context.Context, now time.Time, limit int) ([]OutboxMessage, error) {
	if s == nil || s.db == nil || s.tableName == "" || limit <= 0 {
		return nil, nil
	}

	var messages []OutboxMessage
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		rows := make([]struct {
			ID          int64
			Topic       string
			EventName   string
			EventKey    string
			Producer    string
			HeadersJSON []byte
			Body        []byte
			RetryCount  int
			MaxRetries  int
		}, 0, limit)

		query := fmt.Sprintf(`
SELECT id, topic, event_name, event_key, producer, headers_json, body, retry_count, max_retries
FROM %s
WHERE status IN (?, ?)
  AND retry_count < max_retries
  AND (next_retry_at IS NULL OR next_retry_at <= ?)
ORDER BY id ASC
LIMIT ?
FOR UPDATE SKIP LOCKED`, s.tableName)
		if err := tx.Raw(query, OutboxStatusPending, OutboxStatusFailed, now, limit).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}

		ids := make([]int64, 0, len(rows))
		messages = make([]OutboxMessage, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
			headers := make(map[string]string)
			if len(row.HeadersJSON) > 0 {
				if err := json.Unmarshal(row.HeadersJSON, &headers); err != nil {
					return err
				}
			}
			messages = append(messages, OutboxMessage{
				ID:         row.ID,
				Topic:      row.Topic,
				EventName:  row.EventName,
				EventKey:   row.EventKey,
				Producer:   row.Producer,
				Headers:    headers,
				Body:       row.Body,
				RetryCount: row.RetryCount,
				MaxRetries: row.MaxRetries,
			})
		}

		return tx.Table(s.tableName).
			Where("id IN ?", ids).
			Updates(map[string]any{
				"status":        OutboxStatusPublishing,
				"last_error":    "",
				"next_retry_at": sql.NullTime{},
			}).Error
	})
	return messages, err
}

func (s *GormStore) MarkPublished(ctx context.Context, id int64, publishedAt time.Time) error {
	if s == nil || s.db == nil || s.tableName == "" || id <= 0 {
		return nil
	}
	return s.db.WithContext(ctx).Table(s.tableName).Where("id = ?", id).Updates(map[string]any{
		"status":       OutboxStatusPublished,
		"published_at": publishedAt,
		"last_error":   "",
	}).Error
}

func (s *GormStore) MarkFailed(ctx context.Context, id int64, retryCount int, nextRetryAt time.Time, errMsg string) error {
	if s == nil || s.db == nil || s.tableName == "" || id <= 0 {
		return nil
	}
	return s.db.WithContext(ctx).Table(s.tableName).Where("id = ?", id).Updates(map[string]any{
		"status":        OutboxStatusFailed,
		"retry_count":   retryCount,
		"next_retry_at": nextRetryAt,
		"last_error":    errMsg,
	}).Error
}
