package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/nelthaarion/breeze"
)

type clientData struct {
	lastRequest time.Time
	requests    int
}

// RateLimiterOptions defines the configuration for the middleware.
type RateLimiterOptions struct {
	Requests int           // max allowed requests per Per window
	Per      time.Duration // window duration
	Message  string        // optional custom 429 message
}

type RateLimiter struct {
	options    RateLimiterOptions
	clients    map[string]*clientData
	mu         sync.Mutex
	limitMsg   string // pre-formatted to avoid fmt.Sprintf per request
}

// NewRateLimiter returns a rate-limiting middleware.
// A background goroutine sweeps expired entries every Per interval so the
// client map does not grow unbounded under traffic from many distinct IPs.
func NewRateLimiter(opts RateLimiterOptions) breeze.HandlerFunc {
	msg := opts.Message
	if msg == "" {
		msg = fmt.Sprintf("Rate limit exceeded: max %d requests per %s", opts.Requests, opts.Per)
	}

	rl := &RateLimiter{
		options:  opts,
		clients:  make(map[string]*clientData),
		limitMsg: msg,
	}

	// Background eviction: remove entries whose window has fully expired.
	go func() {
		ticker := time.NewTicker(opts.Per)
		defer ticker.Stop()
		for range ticker.C {
			rl.mu.Lock()
			cutoff := time.Now().Add(-opts.Per)
			for ip, d := range rl.clients {
				if d.lastRequest.Before(cutoff) {
					delete(rl.clients, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()

	return func(ctx *breeze.Context) {
		clientIP := ctx.Conn.RemoteAddr().String()

		rl.mu.Lock()
		now := time.Now()
		data, exists := rl.clients[clientIP]
		if !exists {
			rl.clients[clientIP] = &clientData{lastRequest: now, requests: 1}
			rl.mu.Unlock()
			ctx.Next()
			return
		}

		if now.Sub(data.lastRequest) > rl.options.Per {
			data.requests = 1
			data.lastRequest = now
		} else {
			data.requests++
		}
		exceeded := data.requests > rl.options.Requests
		rl.mu.Unlock()

		if exceeded {
			ctx.Status(429)
			ctx.WriteString(rl.limitMsg)
			return
		}

		ctx.Next()
	}
}

// ── Body size limit middleware ─────────────────────────────────────────────

// BodySizeLimitMiddleware rejects requests whose Content-Length exceeds
// maxBytes with a 413 Content Too Large response.
// This is a middleware companion to the global breeze.MaxBodySize; use it
// when you need per-route limits (e.g. a file-upload route can allow more
// than the default API limit).
func BodySizeLimitMiddleware(maxBytes int64) breeze.HandlerFunc {
	msg := fmt.Sprintf("Request body exceeds the %d byte limit", maxBytes)
	return func(ctx *breeze.Context) {
		if int64(len(ctx.Req.Body)) > maxBytes {
			ctx.Status(413)
			ctx.WriteString(msg)
			return
		}
		ctx.Next()
	}
}
