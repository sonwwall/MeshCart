package middleware

import (
	"context"
	"strconv"
	"time"

	"meshcart/app/common"
	tracex "meshcart/app/trace"
	"meshcart/gateway/config"

	"github.com/cloudwego/hertz/pkg/app"
	jwtmw "github.com/hertz-contrib/jwt"
)

const (
	ClaimUserID   = "user_id"
	ClaimUsername = "username"
	ClaimRole     = "role"
	ClaimIssuer   = "iss"
)

type AuthIdentity struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func NewJWT(cfg config.JWTConfig) (*jwtmw.HertzJWTMiddleware, error) {
	return jwtmw.New(&jwtmw.HertzJWTMiddleware{
		Realm:            "meshcart",
		SigningAlgorithm: "HS256",
		Key:              []byte(cfg.Secret),
		Timeout:          time.Duration(cfg.TimeoutMinutes) * time.Minute,
		MaxRefresh:       time.Duration(cfg.MaxRefreshMinutes) * time.Minute,
		IdentityKey:      ClaimUsername,
		TokenLookup:      "header: Authorization, query: token",
		TokenHeadName:    "Bearer",
		PayloadFunc: func(data interface{}) jwtmw.MapClaims {
			identity, ok := data.(*AuthIdentity)
			if !ok || identity == nil {
				return jwtmw.MapClaims{}
			}
			return jwtmw.MapClaims{
				ClaimUserID:   identity.UserID,
				ClaimUsername: identity.Username,
				ClaimRole:     identity.Role,
				ClaimIssuer:   cfg.Issuer,
			}
		},
		IdentityHandler: func(ctx context.Context, c *app.RequestContext) interface{} {
			identity, _ := IdentityFromRequest(ctx, c)
			return identity
		},
		Unauthorized: func(ctx context.Context, c *app.RequestContext, _ int, _ string) {
			traceID := TraceIDFromRequest(c)
			if traceID == "" {
				traceID = tracex.TraceID(ctx)
			}
			c.JSON(200, common.Fail(common.ErrUnauthorized, traceID))
		},
		RefreshResponse: func(ctx context.Context, c *app.RequestContext, _ int, token string, expire time.Time) {
			traceID := TraceIDFromRequest(c)
			if traceID == "" {
				traceID = tracex.TraceID(ctx)
			}
			c.JSON(200, common.Success(map[string]interface{}{
				"token":     FormatBearerToken(token),
				"expire_at": expire.Format(time.RFC3339),
			}, traceID))
		},
		HTTPStatusMessageFunc: func(err error, _ context.Context, _ *app.RequestContext) string {
			if err != nil {
				return err.Error()
			}
			return common.ErrUnauthorized.Msg
		},
		TimeFunc: time.Now,
	})
}

func FormatBearerToken(token string) string {
	if token == "" {
		return ""
	}
	return "Bearer " + token
}

func IdentityFromRequest(ctx context.Context, c *app.RequestContext) (*AuthIdentity, bool) {
	claims := jwtmw.ExtractClaims(ctx, c)
	username, ok := claims[ClaimUsername].(string)
	if !ok || username == "" {
		return nil, false
	}
	return &AuthIdentity{
		UserID:   claimInt64(claims[ClaimUserID]),
		Username: username,
		Role:     claimString(claims[ClaimRole]),
	}, true
}

func OptionalIdentityFromRequest(ctx context.Context, c *app.RequestContext, jwtMiddleware *jwtmw.HertzJWTMiddleware) (*AuthIdentity, bool) {
	if jwtMiddleware == nil {
		return nil, false
	}

	token, err := jwtMiddleware.ParseToken(ctx, c)
	if err != nil || token == nil || !token.Valid {
		return nil, false
	}

	c.Set("JWT_PAYLOAD", jwtmw.ExtractClaimsFromToken(token))
	return IdentityFromRequest(ctx, c)
}

func claimInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func claimString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	}
	return ""
}
