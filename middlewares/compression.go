package middleware

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/nelthaarion/breeze"
)

// CompressionMiddleware compresses responses using supported algorithms
func CompressionMiddleware() breeze.HandlerFunc {
	return func(ctx *breeze.Context) {
		if ctx.Res == nil || len(ctx.Res.Body) == 0 {
			ctx.Next()
			return
		}

		accept := ctx.Req.Header["Accept-Encoding"]
		var buf bytes.Buffer

		switch {
		case strings.Contains(accept, "br"):
			// Brotli compression
			brWriter := brotli.NewWriter(&buf)
			_, err := brWriter.Write(ctx.Res.Body)
			if err == nil {
				brWriter.Close()
				ctx.Res.Body = buf.Bytes()
				ctx.SetHeader("Content-Encoding", "br")
			} else {
				brWriter.Close()
			}
		case strings.Contains(accept, "gzip"):
			// Gzip compression
			gzWriter := gzip.NewWriter(&buf)
			_, err := gzWriter.Write(ctx.Res.Body)
			if err == nil {
				gzWriter.Close()
				ctx.Res.Body = buf.Bytes()
				ctx.SetHeader("Content-Encoding", "gzip")
			} else {
				gzWriter.Close()
			}
		case strings.Contains(accept, "deflate"):
			// Deflate compression
			defWriter, _ := flate.NewWriter(&buf, flate.DefaultCompression)
			_, err := defWriter.Write(ctx.Res.Body)
			if err == nil {
				defWriter.Close()
				ctx.Res.Body = buf.Bytes()
				ctx.SetHeader("Content-Encoding", "deflate")
			} else {
				defWriter.Close()
			}
		default:
			// No supported compression
		}

		ctx.Next()
	}
}
