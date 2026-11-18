package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/UncleKweks/E-Commerce/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// RequestIDHeader is sent back on every response. If a user reports a
// problem, this ID finds the exact log lines for their request.
const RequestIDHeader = "X-Request-ID"

// RequestLogger gives each request an ID, puts a logger tagged with that ID
// into the request context, and logs one line when the request finishes.
// It never logs headers or bodies, which can contain passwords and tokens.
func RequestLogger(base *zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		id := newRequestID()
		c.Header(RequestIDHeader, id)

		log := base.With().Str("request_id", id).Logger()
		c.Request = c.Request.WithContext(log.WithContext(c.Request.Context()))

		c.Next()

		status := c.Writer.Status()
		event := log.Info()
		if status >= 500 {
			event = log.Error()
		} else if status >= 400 {
			event = log.Warn()
		}
		event.
			Str("method", c.Request.Method).
			Str("route", c.FullPath()).
			Int("status", status).
			Dur("duration_ms", time.Since(start)).
			Str("ip", c.ClientIP()).
			Msg("request")
	}
}

// Recovery turns a panic in any handler into a logged error and a 500,
// instead of crashing the server or leaking a stack trace to the client.
func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		zerolog.Ctx(c.Request.Context()).Error().
			Interface("panic", recovered).
			Str("route", c.FullPath()).
			Msg("panic recovered")
		response.InternalError(c)
	})
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
