package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"meshcart/gateway/config"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("auth session not found")
var ErrSessionConflict = errors.New("auth session conflict")

type Session struct {
	SessionID        string    `json:"session_id"`
	UserID           int64     `json:"user_id"`
	Username         string    `json:"username"`
	Role             string    `json:"role"`
	RefreshTokenHash string    `json:"refresh_token_hash"`
	ExpiresAt        time.Time `json:"expires_at"`
	DeviceID         string    `json:"device_id,omitempty"`
	UserAgent        string    `json:"user_agent,omitempty"`
	IP               string    `json:"ip,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type SessionStore interface {
	Save(ctx context.Context, session *Session) error
	Get(ctx context.Context, sessionID string) (*Session, error)
	GetByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (*Session, error)
	Rotate(ctx context.Context, sessionID, currentRefreshTokenHash, nextRefreshTokenHash string, expiresAt, updatedAt time.Time, username, role string) (*Session, error)
	Delete(ctx context.Context, sessionID string) error
}

func NewSessionID() string {
	return uuid.NewString()
}

func NewRefreshToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type RedisSessionStore struct {
	client    *redis.Client
	keyPrefix string
}

func NewRedisSessionStore(client *redis.Client, cfg config.AuthSessionConfig) *RedisSessionStore {
	return &RedisSessionStore{
		client:    client,
		keyPrefix: cfg.KeyPrefix,
	}
}

func (s *RedisSessionStore) Save(ctx context.Context, session *Session) error {
	if session == nil {
		return errors.New("auth session is nil")
	}
	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return errors.New("auth session already expired")
	}
	_, err = s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, s.sessionKey(session.SessionID), payload, ttl)
		pipe.Set(ctx, s.refreshKey(session.RefreshTokenHash), session.SessionID, ttl)
		return nil
	})
	return err
}

func (s *RedisSessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	value, err := s.client.Get(ctx, s.sessionKey(sessionID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	var session Session
	if err := json.Unmarshal([]byte(value), &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *RedisSessionStore) GetByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (*Session, error) {
	sessionID, err := s.client.Get(ctx, s.refreshKey(refreshTokenHash)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return s.Get(ctx, sessionID)
}

func (s *RedisSessionStore) Rotate(ctx context.Context, sessionID, currentRefreshTokenHash, nextRefreshTokenHash string, expiresAt, updatedAt time.Time, username, role string) (*Session, error) {
	sessionKey := s.sessionKey(sessionID)
	currentRefreshKey := s.refreshKey(currentRefreshTokenHash)
	nextRefreshKey := s.refreshKey(nextRefreshTokenHash)

	var rotated *Session
	err := s.client.Watch(ctx, func(tx *redis.Tx) error {
		value, err := tx.Get(ctx, sessionKey).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return ErrSessionNotFound
			}
			return err
		}

		var session Session
		if err := json.Unmarshal([]byte(value), &session); err != nil {
			return err
		}
		if session.RefreshTokenHash != currentRefreshTokenHash {
			return ErrSessionConflict
		}

		session.RefreshTokenHash = nextRefreshTokenHash
		session.ExpiresAt = expiresAt
		session.UpdatedAt = updatedAt
		if username != "" {
			session.Username = username
		}
		if role != "" {
			session.Role = role
		}

		payload, err := json.Marshal(&session)
		if err != nil {
			return err
		}
		ttl := time.Until(session.ExpiresAt)
		if ttl <= 0 {
			return ErrSessionConflict
		}

		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Set(ctx, sessionKey, payload, ttl)
			pipe.Del(ctx, currentRefreshKey)
			pipe.Set(ctx, nextRefreshKey, session.SessionID, ttl)
			return nil
		})
		if err != nil {
			return err
		}
		rotated = &session
		return nil
	}, sessionKey, currentRefreshKey)
	if errors.Is(err, redis.TxFailedErr) {
		return nil, ErrSessionConflict
	}
	if err != nil {
		return nil, err
	}
	return rotated, nil
}

func (s *RedisSessionStore) Delete(ctx context.Context, sessionID string) error {
	session, err := s.Get(ctx, sessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil
		}
		return err
	}
	_, err = s.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, s.sessionKey(sessionID))
		if session.RefreshTokenHash != "" {
			pipe.Del(ctx, s.refreshKey(session.RefreshTokenHash))
		}
		return nil
	})
	return err
}

func (s *RedisSessionStore) sessionKey(sessionID string) string {
	return fmt.Sprintf("%s:%s", s.keyPrefix, sessionID)
}

func (s *RedisSessionStore) refreshKey(refreshTokenHash string) string {
	return fmt.Sprintf("%s:refresh:%s", s.keyPrefix, refreshTokenHash)
}

type MemorySessionStore struct {
	mu           sync.RWMutex
	sessions     map[string]*Session
	refreshIndex map[string]string
}

func NewMemorySessionStore() *MemorySessionStore {
	return &MemorySessionStore{
		sessions:     make(map[string]*Session),
		refreshIndex: make(map[string]string),
	}
}

func (s *MemorySessionStore) Save(_ context.Context, session *Session) error {
	if session == nil {
		return errors.New("auth session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := *session
	s.sessions[session.SessionID] = &cloned
	if session.RefreshTokenHash != "" {
		s.refreshIndex[session.RefreshTokenHash] = session.SessionID
	}
	return nil
}

func (s *MemorySessionStore) Get(_ context.Context, sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	cloned := *session
	return &cloned, nil
}

func (s *MemorySessionStore) Delete(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[sessionID]; ok && session.RefreshTokenHash != "" {
		delete(s.refreshIndex, session.RefreshTokenHash)
	}
	delete(s.sessions, sessionID)
	return nil
}

func (s *MemorySessionStore) GetByRefreshTokenHash(_ context.Context, refreshTokenHash string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessionID, ok := s.refreshIndex[refreshTokenHash]
	if !ok {
		return nil, ErrSessionNotFound
	}
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	cloned := *session
	return &cloned, nil
}

func (s *MemorySessionStore) Rotate(_ context.Context, sessionID, currentRefreshTokenHash, nextRefreshTokenHash string, expiresAt, updatedAt time.Time, username, role string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	if session.RefreshTokenHash != currentRefreshTokenHash {
		return nil, ErrSessionConflict
	}

	delete(s.refreshIndex, currentRefreshTokenHash)
	session.RefreshTokenHash = nextRefreshTokenHash
	session.ExpiresAt = expiresAt
	session.UpdatedAt = updatedAt
	if username != "" {
		session.Username = username
	}
	if role != "" {
		session.Role = role
	}
	s.refreshIndex[nextRefreshTokenHash] = sessionID

	cloned := *session
	return &cloned, nil
}
