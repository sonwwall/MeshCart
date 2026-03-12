package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"meshcart/app/common"
	tracex "meshcart/app/trace"
	"meshcart/gateway/config"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

type RateLimitRule struct {
	RatePerSecond float64
	Burst         float64
}

func (r RateLimitRule) Enabled() bool {
	return r.RatePerSecond > 0 && r.Burst > 0
}

type RateLimitKeyFunc func(context.Context, *app.RequestContext) (string, bool)

type RateLimitStore struct {
	mu              sync.Mutex
	limiters        map[string]*rateLimitEntry
	entryTTL        time.Duration
	cleanupInterval time.Duration
	lastCleanup     time.Time
	now             func() time.Time
}

type rateLimitEntry struct {
	limiter  *tokenBucket
	lastSeen time.Time
}

type tokenBucket struct {
	mu         sync.Mutex
	rate       float64
	capacity   float64
	tokens     float64
	lastRefill time.Time
}

func NewRateLimitStore(cfg config.RateLimitConfig) *RateLimitStore {
	now := time.Now
	return &RateLimitStore{
		limiters:        make(map[string]*rateLimitEntry),
		entryTTL:        cfg.EntryTTL,
		cleanupInterval: cfg.CleanupInterval,
		lastCleanup:     now(),
		now:             now,
	}
}

func NewRule(cfg config.RateLimitRuleConfig) RateLimitRule {
	return RateLimitRule{
		RatePerSecond: float64(cfg.RatePerSecond),
		Burst:         float64(cfg.Burst),
	}
}

func RateLimit(store *RateLimitStore, rule RateLimitRule, keyFunc RateLimitKeyFunc) app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if store == nil || !rule.Enabled() || keyFunc == nil {
			c.Next(ctx)
			return
		}

		key, ok := keyFunc(ctx, c)
		if !ok || key == "" {
			c.Next(ctx)
			return
		}

		if store.Allow(key, rule) {
			c.Next(ctx)
			return
		}

		traceID := TraceIDFromRequest(c)
		if traceID == "" {
			traceID = tracex.TraceID(ctx)
		}

		c.Abort()
		c.JSON(consts.StatusOK, common.Fail(common.ErrTooManyRequests, traceID))
	}
}

func IPRouteKey(_ context.Context, c *app.RequestContext) (string, bool) {
	ip := c.ClientIP()
	if ip == "" {
		return "", false
	}
	return fmt.Sprintf("ip:%s:%s:%s", ip, string(c.Method()), routePattern(c)), true
}

func IPKey(_ context.Context, c *app.RequestContext) (string, bool) {
	ip := c.ClientIP()
	if ip == "" {
		return "", false
	}
	return fmt.Sprintf("ip:%s", ip), true
}

func UserRouteKey(ctx context.Context, c *app.RequestContext) (string, bool) {
	if identity, ok := IdentityFromRequest(ctx, c); ok && identity.UserID > 0 {
		return fmt.Sprintf("user:%d:%s:%s", identity.UserID, string(c.Method()), routePattern(c)), true
	}

	ip := c.ClientIP()
	if ip == "" {
		return "", false
	}
	return fmt.Sprintf("ip:%s:%s:%s", ip, string(c.Method()), routePattern(c)), true
}

func RouteKey(_ context.Context, c *app.RequestContext) (string, bool) {
	return fmt.Sprintf("route:%s:%s", string(c.Method()), routePattern(c)), true
}

func routePattern(c *app.RequestContext) string {
	if fullPath := c.FullPath(); fullPath != "" {
		return fullPath
	}
	return string(c.Path())
}

func (s *RateLimitStore) Allow(key string, rule RateLimitRule) bool {
	now := s.now()

	s.mu.Lock()
	s.cleanupExpired(now)

	entry, ok := s.limiters[key]
	if !ok {
		entry = &rateLimitEntry{
			limiter:  newTokenBucket(rule, now),
			lastSeen: now,
		}
		s.limiters[key] = entry
	} else {
		entry.lastSeen = now
	}
	s.mu.Unlock()

	return entry.limiter.Allow(now)
}

func (s *RateLimitStore) cleanupExpired(now time.Time) {
	if s.entryTTL <= 0 || s.cleanupInterval <= 0 {
		return
	}
	if now.Sub(s.lastCleanup) < s.cleanupInterval {
		return
	}

	for key, entry := range s.limiters {
		if now.Sub(entry.lastSeen) > s.entryTTL {
			delete(s.limiters, key)
		}
	}
	s.lastCleanup = now
}

func newTokenBucket(rule RateLimitRule, now time.Time) *tokenBucket {
	return &tokenBucket{
		rate:       rule.RatePerSecond,
		capacity:   rule.Burst,
		tokens:     rule.Burst,
		lastRefill: now,
	}
}

func (b *tokenBucket) Allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * b.rate
		if b.tokens > b.capacity {
			b.tokens = b.capacity
		}
		b.lastRefill = now
	}

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}
