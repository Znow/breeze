package middleware

import (
	"github.com/nelthaarion/breeze"
)

// CORSOptions defines configuration for CORS
type CORSOptions struct {
	AllowOrigins     string // e.g., "*", or "https://example.com"
	AllowMethods     string // e.g., "GET,POST,PUT,DELETE"
	AllowHeaders     string // e.g., "Content-Type,Authorization"
	ExposeHeaders    string
	AllowCredentials string // "true" or "false"
	MaxAge           string // seconds
}

// CORSMiddleware returns a HandlerFunc to apply CORS headers
func CORSMiddleware(opts CORSOptions) breeze.HandlerFunc {
	return func(ctx *breeze.Context) {
		if opts.AllowOrigins != "" {
			ctx.SetHeader("Access-Control-Allow-Origin", opts.AllowOrigins)
		}
		if opts.AllowMethods != "" {
			ctx.SetHeader("Access-Control-Allow-Methods", opts.AllowMethods)
		}
		if opts.AllowHeaders != "" {
			ctx.SetHeader("Access-Control-Allow-Headers", opts.AllowHeaders)
		}
		if opts.ExposeHeaders != "" {
			ctx.SetHeader("Access-Control-Expose-Headers", opts.ExposeHeaders)
		}
		if opts.AllowCredentials != "" {
			ctx.SetHeader("Access-Control-Allow-Credentials", opts.AllowCredentials)
		}
		if opts.MaxAge != "" {
			ctx.SetHeader("Access-Control-Max-Age", opts.MaxAge)
		}

		// Handle preflight OPTIONS request
		if ctx.Req.Method == "OPTIONS" {
			ctx.Status(204)
			return
		}

		ctx.Next()
	}
}
