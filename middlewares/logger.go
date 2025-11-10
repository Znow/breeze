package middleware

import (
	"fmt"
	"time"

	"github.com/nelthaarion/breeze"
)

func LoggingMiddleware() breeze.HandlerFunc {
	return func(ctx *breeze.Context) {
		start := time.Now()
		method := ctx.Req.Method
		path := ctx.Req.Path

		ctx.Next() // continue chain

		status := 0
		if ctx.Res != nil {
			status = ctx.Res.Status
		}
		fmt.Printf("[Breeze][%s] %s %s -> %d (%v)\n", start.Format(time.RFC3339), method, path, status, time.Since(start))
	}
}
