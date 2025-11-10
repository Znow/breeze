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

// RateLimiterOptions defines the configuration for the middleware
type RateLimiterOptions struct {
	Requests int           // allowed requests
	Per      time.Duration // per duration
	Message  string        // optional message on limit
}

type RateLimiter struct {
	options RateLimiterOptions
	clients map[string]*clientData
	mu      sync.Mutex
}

// NewRateLimiter returns a rate limiting middleware
func NewRateLimiter(opts RateLimiterOptions) breeze.HandlerFunc {
	rl := &RateLimiter{
		options: opts,
		clients: make(map[string]*clientData),
	}

	return func(ctx *breeze.Context) {
		// Use IP as key (Conn.RemoteAddr)
		clientIP := ctx.Conn.RemoteAddr().String()

		rl.mu.Lock()
		defer rl.mu.Unlock()

		now := time.Now()
		data, exists := rl.clients[clientIP]
		if !exists {
			data = &clientData{lastRequest: now, requests: 1}
			rl.clients[clientIP] = data
		} else {
			// Reset counter if duration has passed
			if now.Sub(data.lastRequest) > rl.options.Per {
				data.requests = 1
				data.lastRequest = now
			} else {
				data.requests++
			}
		}

		if data.requests > rl.options.Requests {
			// Rate limit exceeded
			ctx.Status(429)
			message := rl.options.Message
			if message == "" {
				message = fmt.Sprintf("Rate limit exceeded: max %d requests per %s", rl.options.Requests, rl.options.Per)
			}
			ctx.WriteString(message)
			return
		}

		ctx.Next()
	}
}
