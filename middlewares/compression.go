package middleware

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/nelthaarion/breeze"
)

// Writer pools avoid allocating a new compressor struct on every request.
// gzip.NewWriter and brotli.NewWriter are expensive — pooling them saves
// significant heap pressure under load.

var gzipPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression)
		return w
	},
}

var brotliPool = sync.Pool{
	New: func() any {
		return brotli.NewWriter(nil)
	},
}

var deflatePool = sync.Pool{
	New: func() any {
		w, _ := flate.NewWriter(nil, flate.DefaultCompression)
		return w
	},
}

// minCompressSize is the minimum response body size (bytes) worth compressing.
// Compressing tiny payloads adds CPU cost and can actually increase size.
const minCompressSize = 512

// CompressionMiddleware compresses responses using the best algorithm the
// client advertises in Accept-Encoding (br > gzip > deflate).
//
// Execution order: handler runs first (ctx.Next), then the response body is
// compressed in place before it is written to the connection.
func CompressionMiddleware() breeze.HandlerFunc {
	return func(ctx *breeze.Context) {
		// Run the full handler chain first so ctx.Res is populated.
		ctx.Next()

		if ctx.Res == nil || len(ctx.Res.Body) < minCompressSize {
			return
		}

		accept := ctx.Req.Header["accept-encoding"]
		if accept == "" {
			return
		}

		var buf bytes.Buffer
		buf.Grow(len(ctx.Res.Body) / 2) // optimistic pre-size

		switch {
		case strings.Contains(accept, "br"):
			w := brotliPool.Get().(*brotli.Writer)
			w.Reset(&buf)
			_, err := w.Write(ctx.Res.Body)
			closeErr := w.Close()
			brotliPool.Put(w)
			if err == nil && closeErr == nil {
				ctx.Res.Body = buf.Bytes()
				ctx.SetHeader("Content-Encoding", "br")
				ctx.SetHeader("Vary", "Accept-Encoding")
			}

		case strings.Contains(accept, "gzip"):
			w := gzipPool.Get().(*gzip.Writer)
			w.Reset(&buf)
			_, err := w.Write(ctx.Res.Body)
			closeErr := w.Close()
			gzipPool.Put(w)
			if err == nil && closeErr == nil {
				ctx.Res.Body = buf.Bytes()
				ctx.SetHeader("Content-Encoding", "gzip")
				ctx.SetHeader("Vary", "Accept-Encoding")
			}

		case strings.Contains(accept, "deflate"):
			w := deflatePool.Get().(*flate.Writer)
			w.Reset(&buf)
			_, err := w.Write(ctx.Res.Body)
			closeErr := w.Close()
			deflatePool.Put(w)
			if err == nil && closeErr == nil {
				ctx.Res.Body = buf.Bytes()
				ctx.SetHeader("Content-Encoding", "deflate")
				ctx.SetHeader("Vary", "Accept-Encoding")
			}
		}
	}
}
