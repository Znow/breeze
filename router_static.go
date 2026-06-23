package breeze

import (
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ServeStatic registers handlers to serve files under `root` at URL prefix `prefix`.
// Example: ServeStatic("/static", "./public") will serve ./public/* at /static/*
func (r *Router) ServeStatic(prefix, root string) {
	// Resolve root once at registration time so every request just does a
	// strings.HasPrefix check — no extra syscall on the hot path.
	absRoot, err := filepath.Abs(root)
	if err != nil {
		panic("ServeStatic: cannot resolve root path: " + err.Error())
	}
	// Ensure the jail boundary always ends with a separator so that a directory
	// named e.g. "/public-evil" cannot match a root of "/public".
	jail := absRoot + string(os.PathSeparator)

	cleanPrefix := strings.TrimSuffix(prefix, "/")
	pattern := cleanPrefix + "/*filepath"

	r.Handle(GET, pattern, func(ctx *Context) {
		fp := ctx.Param("filepath")
		if fp == "" || fp == "/" {
			fp = "index.html"
		}

		// Sanitize: clean the path and make it relative.
		fp = filepath.Clean("/"+fp)[1:]

		full := filepath.Join(absRoot, fp)

		// ── Path traversal guard ───────────────────────────────────────────
		// filepath.Join already resolves "..", but we verify the result is
		// still inside the jail to handle any edge cases (symlinks aside).
		if !strings.HasPrefix(full+string(os.PathSeparator), jail) {
			ctx.Status(403)
			ctx.WriteString("Forbidden")
			return
		}

		f, err := os.Open(full)
		if err != nil {
			ctx.Status(404)
			ctx.WriteString("File not found")
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil || info.IsDir() {
			ctx.Status(404)
			ctx.WriteString("File not found")
			return
		}

		// Pre-size the read buffer to the exact file size to avoid
		// io.ReadAll's internal growth loop for files whose size is known.
		data := make([]byte, info.Size())
		if _, err := io.ReadFull(f, data); err != nil {
			ctx.Status(500)
			ctx.WriteString("Error reading file")
			return
		}

		ctype := mime.TypeByExtension(filepath.Ext(full))
		if ctype == "" {
			ctype = http.DetectContentType(data)
		}

		ctx.Res = &HTTPResponse{
			Status:  200,
			Headers: map[string]string{"Content-Type": ctype},
			Body:    data,
		}
	})
}
