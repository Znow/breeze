package middleware

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/nelthaarion/breeze"
)

type clientData struct {
	windowStart time.Time
	requests    int
}

// RateLimiterOptions defines the configuration for the middleware.
type RateLimiterOptions struct {
	Requests int           // max allowed requests per Per window
	Per      time.Duration // window duration
	Message  string        // optional custom 429 message
}

type RateLimiter struct {
	options  RateLimiterOptions
	clients  map[string]*clientData
	mu       sync.Mutex
	limitMsg string // pre-formatted to avoid fmt.Sprintf per request
	done     chan struct{}
}

// NewRateLimiter returns a rate-limiting middleware.
// A background goroutine sweeps expired entries every Per interval so the
// client map does not grow unbounded under traffic from many distinct IPs.
// Call the returned stop() function to shut down the background goroutine
// (e.g. on server shutdown or during tests).
func NewRateLimiter(opts RateLimiterOptions) (handler breeze.HandlerFunc, stop func()) {
	if opts.Per <= 0 {
		panic("breeze/middleware: RateLimiterOptions.Per must be a positive duration (e.g. time.Minute)")
	}
	if opts.Requests <= 0 {
		panic("breeze/middleware: RateLimiterOptions.Requests must be a positive integer")
	}

	msg := opts.Message
	if msg == "" {
		msg = fmt.Sprintf("Rate limit exceeded: max %d requests per %s", opts.Requests, opts.Per)
	}

	rl := &RateLimiter{
		options:  opts,
		clients:  make(map[string]*clientData),
		limitMsg: msg,
		done:     make(chan struct{}),
	}

	// Background eviction: remove entries whose window has fully expired.
	// Exits cleanly when stop() is called.
	go func() {
		ticker := time.NewTicker(opts.Per)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.mu.Lock()
				cutoff := time.Now().Add(-opts.Per)
				for ip, d := range rl.clients {
					if d.windowStart.Before(cutoff) {
						delete(rl.clients, ip)
					}
				}
				rl.mu.Unlock()
			case <-rl.done:
				return
			}
		}
	}()

	handler = func(ctx *breeze.Context) {
		// Bug fix #2: strip the ephemeral port so all connections from the
		// same host map to the same bucket. RemoteAddr() returns "host:port";
		// without this every new TCP connection was treated as a new client.
		addr := ctx.Conn.RemoteAddr().String()
		clientIP, _, err := net.SplitHostPort(addr)
		if err != nil {
			// Fallback: use the raw address if parsing fails (e.g. Unix sockets).
			clientIP = addr
		}

		rl.mu.Lock()
		now := time.Now()
		data, exists := rl.clients[clientIP]
		if !exists {
			rl.clients[clientIP] = &clientData{windowStart: now, requests: 1}
			rl.mu.Unlock()
			ctx.Next()
			return
		}

		// Bug fix #3: use >= so a request arriving exactly at the window
		// boundary correctly opens a fresh window rather than staying in the old one.
		if now.Sub(data.windowStart) >= rl.options.Per {
			data.requests = 1
			data.windowStart = now
		} else {
			data.requests++
		}
		exceeded := data.requests > rl.options.Requests
		rl.mu.Unlock()

		if exceeded {
			// Bug fix #1: WriteString resets ctx.Res to a new HTTPResponse with
			// Status 200, so Status(429) must be called AFTER WriteString, not before.
			ctx.WriteString(rl.limitMsg)
			ctx.Status(429)
			return
		}

		ctx.Next()
	}

	stop = func() { close(rl.done) }
	return handler, stop
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
			ctx.WriteString(msg)
			ctx.Status(413)
			return
		}
		ctx.Next()
	}
}
