package middleware

import (
	"encoding/binary"
	"hash/fnv"
	"strconv"
	"sync"
	"time"

	"github.com/nelthaarion/breeze"
)

// cachedResponse holds the serialised ETag for a response body.
type cachedResponse struct {
	etag      string
	expiresAt time.Time
}

// ETagCache stores cached ETags keyed by method+path+query.
type ETagCache struct {
	mu    sync.RWMutex
	store map[string]*cachedResponse
	ttl   time.Duration
}

// NewETagCache creates a new ETag cache.
// ttl controls how long entries live before being evicted (0 = no expiry).
func NewETagCache(ttl time.Duration) *ETagCache {
	c := &ETagCache{
		store: make(map[string]*cachedResponse),
		ttl:   ttl,
	}
	if ttl > 0 {
		// Background goroutine evicts stale entries once per TTL period.
		go func() {
			ticker := time.NewTicker(ttl)
			defer ticker.Stop()
			for range ticker.C {
				c.evict()
			}
		}()
	}
	return c
}

func (c *ETagCache) evict() {
	now := time.Now()
	c.mu.Lock()
	for k, v := range c.store {
		if !v.expiresAt.IsZero() && now.After(v.expiresAt) {
			delete(c.store, k)
		}
	}
	c.mu.Unlock()
}

// cacheKey builds a map key that includes method, path, and query string
// so that different variants of the same path are cached independently.
func cacheKey(ctx *breeze.Context) string {
	q := ""
	if ctx.Req.Query != nil {
		q = ctx.Req.Query.Encode()
	}
	return string(ctx.Req.Method) + ctx.Req.Path + "?" + q
}

// bodyETag computes a fast, non-cryptographic ETag for the response body
// using FNV-1a (significantly faster than MD5 with no security requirement here).
func bodyETag(body []byte) string {
	h := fnv.New64a()
	h.Write(body)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], h.Sum64())
	// Encode as a short hex string — cheaper than hex.EncodeToString(md5.Sum)
	return strconv.FormatUint(h.Sum64(), 16)
}

// ETagMiddleware returns a Breeze middleware that:
//  1. Calls the handler chain first (ctx.Next).
//  2. Computes an ETag over the response body.
//  3. Caches the ETag and sets the ETag response header.
//  4. Returns 304 Not Modified when the client's If-None-Match matches.
func (c *ETagCache) ETagMiddleware() breeze.HandlerFunc {
	return func(ctx *breeze.Context) {
		// Run the handler first so ctx.Res is populated.
		ctx.Next()

		if ctx.Res == nil || len(ctx.Res.Body) == 0 {
			return
		}

		etag := bodyETag(ctx.Res.Body)
		key := cacheKey(ctx)

		entry := &cachedResponse{etag: etag}
		if c.ttl > 0 {
			entry.expiresAt = time.Now().Add(c.ttl)
		}

		c.mu.Lock()
		c.store[key] = entry
		c.mu.Unlock()

		ctx.SetHeader("ETag", etag)

		// Check If-None-Match *after* computing the ETag so we always set
		// the header even when returning 304.
		if inm := ctx.Req.Header["if-none-match"]; inm != "" && inm == etag {
			ctx.Status(304)
			ctx.Res.Body = nil
		}
	}
}
