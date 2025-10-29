package breeze

import (
	"fmt"
	"sync"

	"github.com/panjf2000/gnet/v2"
)

type Breeze struct {
	*gnet.BuiltinEventEngine
	Router          *Router
	Bufs            map[int][]byte
	Pool            *WorkerPool
	mu              sync.Mutex
	numberOfWorkers int
}

func New(router *Router, pool *WorkerPool) *Breeze {
	return &Breeze{
		BuiltinEventEngine: &gnet.BuiltinEventEngine{},
		Router:             router,
		Bufs:               make(map[int][]byte),
		Pool:               pool,
		numberOfWorkers:    pool.count,
	}
}

func (s *Breeze) OnTraffic(c gnet.Conn) gnet.Action {
	fd := c.Fd()
	data, _ := c.Next(-1)
	if len(data) == 0 {
		return gnet.None
	}

	s.mu.Lock()
	s.Bufs[fd] = append(s.Bufs[fd], data...)
	buf := s.Bufs[fd]
	s.mu.Unlock()

	for len(buf) > 0 {
		req, consumed, err := ParseHTTPRequest(buf)
		if err != nil {
			c.AsyncWrite([]byte("HTTP/1.1 400 Bad Request\r\nContent-Length: 11\r\n\r\nBad Request"), nil)
			buf = nil
			break
		}
		if req == nil {
			break
		}

		handler, middlewares, params := s.Router.Find(req)

		if handler == nil {
			// No route found — send 404
			c.AsyncWrite([]byte("HTTP/1.1 404 Not Found\r\nContent-Length: 9\r\n\r\nNot Found"), nil)
			// Consume the buffer and stop processing
			if consumed >= len(buf) {
				buf = nil
			} else {
				buf = buf[consumed:]
			}
			continue
		}

		ctx := &Context{
			Conn:        c,
			Req:         req,
			params:      params,
			middlewares: append(middlewares, handler), // append handler as final middleware
			index:       -1,
		}

		exec := func() {
			ctx.Next()
			if ctx.Res != nil {
				c.AsyncWrite(ctx.Res.Bytes(), nil)
			}
		}

		if s.Pool != nil {
			s.Pool.Submit(exec)
		} else {
			go exec()
		}

		if consumed >= len(buf) {
			buf = nil
			break
		} else {
			buf = buf[consumed:]
		}
	}

	s.mu.Lock()
	s.Bufs[fd] = buf
	s.mu.Unlock()

	return gnet.None
}

func (s *Breeze) Run(port int, multiCore bool) error {
	return gnet.Run(
		s,
		fmt.Sprintf("tcp://:%d", port),
		gnet.WithTCPNoDelay(gnet.TCPNoDelay),
		gnet.WithMulticore(multiCore),
		gnet.WithLoadBalancing(gnet.RoundRobin),
	)
}
