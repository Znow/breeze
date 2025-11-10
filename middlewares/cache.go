package middleware

import (
	"crypto/md5"
	"encoding/hex"
	"sync"

	"github.com/nelthaarion/breeze"
)

// ETagCache stores cached responses per route or URL
type ETagCache struct {
	mu    sync.RWMutex
	store map[string]*cachedResponse
}

type cachedResponse struct {
	body []byte
	etag string
}

// NewETagCache creates a new ETag cache
func NewETagCache() *ETagCache {
	return &ETagCache{
		store: make(map[string]*cachedResponse),
	}
}

// ETagMiddleware returns a Breeze middleware that sets ETag headers
func (c *ETagCache) ETagMiddleware() breeze.HandlerFunc {
	return func(ctx *breeze.Context) {
		if ctx.Res == nil || len(ctx.Res.Body) == 0 {
			ctx.Next()
			return
		}

		// Generate ETag based on body
		hash := md5.Sum(ctx.Res.Body)
		etag := hex.EncodeToString(hash[:])

		url := ctx.Req.Path // simple key, could include query

		c.mu.Lock()
		c.store[url] = &cachedResponse{
			body: ctx.Res.Body,
			etag: etag,
		}
		c.mu.Unlock()

		// Set ETag header
		ctx.SetHeader("ETag", etag)

		// Check If-None-Match
		if inm := ctx.Req.Header["If-None-Match"]; inm != "" {
			if inm == etag {
				ctx.Status(304)
				ctx.Res.Body = nil
				return
			}
		}

		ctx.Next()
	}
}
