package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lk/zoek/backend/internal/errs"
	"github.com/lk/zoek/backend/internal/logger"
	"go.uber.org/zap"
)

// JWTManager handles token creation and verification.
type JWTManager struct {
	Secret      string
	Issuer      string
	TokenExpiry int // hours, 0 = no expiry
}

// Claims is the JWT payload.
type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

// NewJWTManager creates a new JWT manager.
func NewJWTManager(secret string, tokenExpiry int) *JWTManager {
	return &JWTManager{Secret: secret, Issuer: "zoek", TokenExpiry: tokenExpiry}
}

// GenerateToken creates a signed JWT for a user.
func (m *JWTManager) GenerateToken(userID int64) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer: m.Issuer,
		},
	}
	if m.TokenExpiry > 0 {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Duration(m.TokenExpiry) * time.Hour))
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.Secret))
}

// ParseToken verifies a JWT and returns the claims.
func (m *JWTManager) ParseToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return []byte(m.Secret), nil
	})
	if err != nil || !token.Valid {
		return nil, errs.ErrAuthExpired
	}
	return claims, nil
}

// Auth middleware verifies the JWT and sets the user ID in context.
func (m *JWTManager) Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "AUTH_REQUIRED",
				"message": "请先登录",
				"action":  "RELOGIN",
			})
			return
		}
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "AUTH_REQUIRED",
				"message": "请先登录",
				"action":  "RELOGIN",
			})
			return
		}
		claims, err := m.ParseToken(parts[1])
		if err != nil {
			bizErr, ok := err.(*errs.BizError)
			if ok {
				c.AbortWithStatusJSON(http.StatusUnauthorized, bizErr)
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, errs.ErrAuthExpired)
			return
		}
		c.Set("user_id", claims.UserID)
		c.Next()
	}
}

// RequestID middleware generates a UUID request ID if not provided by the client.
// The ID is stored in context and set on the response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = c.Query("request_id")
		}
		if reqID == "" {
			reqID = uuid.New().String()
		}
		c.Set("request_id", reqID)
		c.Header("X-Request-ID", reqID)
		c.Next()
	}
}

// Logging is a structured request logger that logs method, path, status,
// latency, and request ID for each request.
func Logging(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		fields := []zap.Field{
			logger.String("method", method),
			logger.String("path", path),
			logger.Int("status", status),
			logger.String("latency", latency.String()),
			logger.String("request_id", GetRequestID(c)),
			logger.String("client_ip", c.ClientIP()),
		}

		// Attach user ID if available
		if uid := GetUserID(c); uid > 0 {
			fields = append(fields, logger.Int64("user_id", uid))
		}

		switch {
		case status >= 500:
			log.Error("request completed", fields...)
		case status >= 400:
			log.Warn("request completed", fields...)
		default:
			log.Info("request completed", fields...)
		}
	}
}

// CORS returns a middleware that sets CORS headers based on the allow origins.
func CORS(allowOrigins []string) gin.HandlerFunc {
	allowAll := false
	for _, o := range allowOrigins {
		if o == "*" {
			allowAll = true
			break
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}

		if allowAll {
			c.Header("Access-Control-Allow-Origin", "*")
		} else {
			for _, o := range allowOrigins {
				if o == origin {
					c.Header("Access-Control-Allow-Origin", origin)
					break
				}
			}
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// ErrorHandler is a recovery middleware that catches panics, logs them, and
// returns a uniform JSON error.
func ErrorHandler(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Error("panic recovered",
					logger.Any("panic", rec),
					logger.String("path", c.Request.URL.Path),
					logger.String("request_id", GetRequestID(c)),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":    "INTERNAL",
					"message": "服务器内部错误",
					"action":  "RETRY",
				})
			}
		}()
		c.Next()
	}
}

// GetUserID extracts the user ID from the Gin context (set by Auth middleware).
func GetUserID(c *gin.Context) int64 {
	v, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	return v.(int64)
}

// GetRequestID extracts the request ID from the context.
func GetRequestID(c *gin.Context) string {
	v, exists := c.Get("request_id")
	if !exists {
		return ""
	}
	return v.(string)
}
