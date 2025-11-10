package breeze

import (
	"os"
	"path/filepath"
	"strings"
)

type HandlerFunc func(*Context)

type route struct {
	method       Method
	pattern      string
	segments     []string
	handler      HandlerFunc
	hasWildcard  bool
	wildcardName string
}

type Router struct {
	routes        []*route
	middlewares   []HandlerFunc
	staticDir     string
	autoServeRoot bool
}

func NewRouter() *Router {
	return &Router{
		staticDir:     "./public",
		autoServeRoot: true,
	}
}

func (r *Router) Use(mw ...HandlerFunc) {
	r.middlewares = append(r.middlewares, mw...)
}

// Optionally change static dir
func (r *Router) SetStaticDir(dir string) {
	r.staticDir = dir
}

func (r *Router) Handle(method Method, pattern string, handler HandlerFunc, middlewares ...HandlerFunc) {
	if pattern == "" || pattern[0] != '/' {
		panic("invalid route pattern: must start with '/'")
	}

	trimmed := strings.Trim(pattern, "/")
	var segments []string
	hasWildcard := false
	wildcardName := ""

	if trimmed == "" {
		segments = []string{} // root route
	} else {
		segments = strings.Split(trimmed, "/")
		last := segments[len(segments)-1]

		if strings.HasPrefix(last, "*") {
			hasWildcard = true
			if len(last) > 1 {
				wildcardName = last[1:]
			} else {
				wildcardName = "wildcard"
			}
			segments = segments[:len(segments)-1]
		}
	}

	// Combine middlewares + final handler
	finalHandler := func(ctx *Context) {
		ctx.middlewares = append(middlewares, handler)
		ctx.index = -1 // will start at 0 in ctx.Next
		ctx.Next()
	}

	r.routes = append(r.routes, &route{
		method:       method,
		pattern:      pattern,
		segments:     segments,
		handler:      finalHandler,
		hasWildcard:  hasWildcard,
		wildcardName: wildcardName,
	})
}

func (r *Router) Find(req *HTTPRequest) (HandlerFunc, []HandlerFunc, map[string]string) {
	path := strings.Trim(req.Path, "/")
	reqSegments := []string{}
	if path != "" {
		reqSegments = strings.Split(path, "/")
	}

	for _, rt := range r.routes {
		if rt.method != req.Method {
			continue
		}

		params := map[string]string{}

		// Wildcard route
		if rt.hasWildcard {
			if len(reqSegments) < len(rt.segments) {
				continue
			}
			match := true
			for i := 0; i < len(rt.segments); i++ {
				rseg := rt.segments[i]
				pseg := reqSegments[i]
				if strings.HasPrefix(rseg, ":") {
					params[rseg[1:]] = pseg
				} else if rseg != pseg {
					match = false
					break
				}
			}
			if match {
				rest := strings.Join(reqSegments[len(rt.segments):], "/")
				params[rt.wildcardName] = rest
				return rt.handler, r.middlewares, params
			}
			continue
		}

		// Normal route
		if len(rt.segments) != len(reqSegments) {
			continue
		}
		match := true
		for i := range rt.segments {
			rseg := rt.segments[i]
			pseg := reqSegments[i]
			if strings.HasPrefix(rseg, ":") {
				params[rseg[1:]] = pseg
			} else if rseg != pseg {
				match = false
				break
			}
		}
		if match {
			return rt.handler, r.middlewares, params
		}
	}

	// ✅ Auto serve index.html for "/"
	if r.autoServeRoot && (req.Path == "/" || req.Path == "") && req.Method == GET {
		indexPath := filepath.Join(r.staticDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			return func(ctx *Context) {
				data, err := os.ReadFile(indexPath)
				if err != nil {
					ctx.WriteString("Error reading index.html")
					return
				}

				ctx.HTML(data)
			}, r.middlewares, nil
		}
	}

	return nil, nil, nil
}
