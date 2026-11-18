package middleware

import (
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/UncleKweks/E-Commerce/internal/response"
	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// CORS lets browser code from the listed websites call this API.
// Browsers block JavaScript on other sites from reading our responses unless
// we send these headers, so only trusted frontends get them.
func CORS(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && slices.Contains(allowedOrigins, origin) {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Idempotency-Key")
			h.Set("Access-Control-Expose-Headers", RequestIDHeader)
			h.Set("Access-Control-Max-Age", "600")
			h.Add("Vary", "Origin")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// BodyLimit rejects request bodies larger than maxBytes, so nobody can make
// the server read a multi-gigabyte upload into memory.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// RateLimit allows each client IP about `every` one request, with short bursts
// of up to burst requests. It slows down password guessing.
//
// Limits are kept in this process's memory, so with several servers each
// one counts separately. Production setups usually move this into Redis
// or the load balancer.
func RateLimit(every time.Duration, burst int) gin.HandlerFunc {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}
	var (
		mu          sync.Mutex
		clients     = map[string]*client{}
		lastCleanup = time.Now()
	)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		// Forget clients idle for 10 minutes so the map can't grow forever.
		if now.Sub(lastCleanup) > time.Minute {
			for k, v := range clients {
				if now.Sub(v.lastSeen) > 10*time.Minute {
					delete(clients, k)
				}
			}
			lastCleanup = now
		}
		cl, ok := clients[ip]
		if !ok {
			cl = &client{limiter: rate.NewLimiter(rate.Every(every), burst)}
			clients[ip] = cl
		}
		cl.lastSeen = now
		allowed := cl.limiter.Allow()
		mu.Unlock()

		if !allowed {
			c.Header("Retry-After", "60")
			response.Error(c, http.StatusTooManyRequests, "rate_limited", "Too many attempts. Please wait a minute and try again.")
			return
		}
		c.Next()
	}
}
