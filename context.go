package breeze

import (
	"github.com/goccy/go-json"
	"github.com/panjf2000/gnet/v2"
)

type Context struct {
	Conn        gnet.Conn
	Req         *HTTPRequest
	Res         *HTTPResponse
	params      map[string]string
	middlewares []HandlerFunc
	index       int
}

func (ctx *Context) WriteString(s string) {
	ctx.Res = &HTTPResponse{
		Status:  200,
		Headers: map[string]string{"Content-Type": "text/plain"},
		Body:    []byte(s),
	}
}

func (ctx *Context) JSON(data any) {
	d, err := json.Marshal(data)
	if err != nil {
		ctx.Res = &HTTPResponse{
			Status:  400,
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    []byte(`{"message":"error parsing json"}`),
		}
		return
	}
	ctx.Res = &HTTPResponse{
		Status:  200,
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    d,
	}
}

func (ctx *Context) HTML(data []byte) {
	ctx.Res = &HTTPResponse{
		Status:  200,
		Headers: map[string]string{"Content-Type": "text/html; charset=utf-8"},
		Body:    data,
	}
}

func (ctx *Context) Status(code int) {
	if ctx.Res == nil {
		ctx.Res = &HTTPResponse{Headers: make(map[string]string)}
	}
	ctx.Res.Status = code
}

// --- Params helpers ---

func (ctx *Context) Param(key string) string {
	if ctx.params == nil {
		return ""
	}
	return ctx.params[key]
}

// SetParam sets a single key/value pair in context params
func (ctx *Context) SetParam(key, value string) {
	if ctx.params == nil {
		ctx.params = make(map[string]string)
	}
	ctx.params[key] = value
}

// SetParams replaces all params with the provided map
func (ctx *Context) SetParams(p map[string]string) {
	if p == nil {
		ctx.params = make(map[string]string)
	} else {
		ctx.params = p
	}
}

func (ctx *Context) Query(key string) string {
	if ctx.Req == nil || ctx.Req.Query == nil {
		return ""
	}
	return ctx.Req.Query.Get(key)
}

// --- Middleware chain control ---

func (ctx *Context) Next() {
	ctx.index++

	// Stop if we've run all middlewares (including the handler)
	if ctx.index >= len(ctx.middlewares) {
		return
	}

	// Execute the current middleware or handler
	fn := ctx.middlewares[ctx.index]
	if fn != nil {
		fn(ctx)
	}
}

func (ctx *Context) Abort() {
	ctx.index = len(ctx.middlewares)
}

// --- New: SetHeader method for security and custom headers ---
func (ctx *Context) SetHeader(key, value string) {
	if ctx.Res == nil {
		ctx.Res = &HTTPResponse{Headers: make(map[string]string)}
	}
	if ctx.Res.Headers == nil {
		ctx.Res.Headers = make(map[string]string)
	}
	ctx.Res.Headers[key] = value
}
func (ctx *Context) GetParams() map[string]string {
	if ctx.params == nil {
		return map[string]string{}
	}

	// return a copy to prevent external modification
	cpy := make(map[string]string, len(ctx.params))
	for k, v := range ctx.params {
		cpy[k] = v
	}
	return cpy
}
func (ctx *Context) GetParam(key string) string {
	if ctx.params == nil {
		return ""
	}
	return ctx.params[key]
}
