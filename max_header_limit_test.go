package breeze

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nelthaarion/gnet/v2"
)

func TestMaxHeaderBytesConfiguration(t *testing.T) {
	app := New(NewRouter(), NewEventLoopWorkerPool(1))
	if got := app.MaxHeaderBytes(); got != defaultMaxHeaderBytes {
		t.Fatalf("default MaxHeaderBytes() = %d, want %d", got, defaultMaxHeaderBytes)
	}

	app.SetMaxHeaderBytes(1024)
	if got := app.MaxHeaderBytes(); got != 1024 {
		t.Fatalf("MaxHeaderBytes() = %d, want 1024", got)
	}

	app.SetMaxHeaderBytes(-1)
	if got := app.MaxHeaderBytes(); got != 0 {
		t.Fatalf("negative SetMaxHeaderBytes left %d, want 0", got)
	}
}

// TestHeaderLimitTerminatedBlock rejects a request whose complete header block
// is larger than the cap, before the handler runs.
func TestHeaderLimitTerminatedBlock(t *testing.T) {
	app := New(NewRouter(), NewEventLoopWorkerPool(1))
	app.SetMaxHeaderBytes(128)

	called := false
	app.Router.Handle(GET, "/", func(*Context) error {
		called = true
		return nil
	})

	pad := strings.Repeat("a", 256)
	raw := "GET / HTTP/1.1\r\nHost: test\r\nX-Pad: " + pad + "\r\n\r\n"
	c := &bodyLimitConn{inbound: []byte(raw)}
	if action := app.OnTraffic(c); action != gnet.Close {
		t.Fatalf("action = %v, want Close", action)
	}
	if called {
		t.Fatal("handler was invoked for an oversized header block")
	}
	if got := c.written.String(); !strings.HasPrefix(got, "HTTP/1.1 431 Request Header Fields Too Large") {
		t.Fatalf("response = %q, want 431", got)
	}
}

// TestHeaderLimitUnterminatedGrowth is the DoS case: a client streams header
// bytes without ever sending the CRLFCRLF terminator. Without a cap the server
// buffers them unbounded; with one it rejects once the buffer exceeds the cap.
func TestHeaderLimitUnterminatedGrowth(t *testing.T) {
	app := New(NewRouter(), NewEventLoopWorkerPool(1))
	app.SetMaxHeaderBytes(128)

	// A valid request line followed by an unterminated, oversized header value.
	raw := "GET / HTTP/1.1\r\nX-Pad: " + strings.Repeat("a", 256)
	c := &bodyLimitConn{inbound: []byte(raw)}
	if action := app.OnTraffic(c); action != gnet.Close {
		t.Fatalf("action = %v, want Close", action)
	}
	if got := c.written.String(); !strings.HasPrefix(got, "HTTP/1.1 431 Request Header Fields Too Large") {
		t.Fatalf("response = %q, want 431", got)
	}
	// The connection context must not retain the oversized buffer as leftover.
	if c.ctx != nil {
		t.Fatalf("connection retained leftover buffer of %d bytes after rejection", len(c.ctx.([]byte)))
	}
}

// TestHeaderLimitUnderCapPasses confirms a normal request within the cap still
// works.
func TestHeaderLimitUnderCapPasses(t *testing.T) {
	app := New(NewRouter(), NewEventLoopWorkerPool(1))
	app.SetMaxHeaderBytes(1024)

	called := false
	app.Router.Handle(GET, "/", func(*Context) error {
		called = true
		return nil
	})

	raw := "GET / HTTP/1.1\r\nHost: test\r\nX-Pad: " + strings.Repeat("a", 64) + "\r\n\r\n"
	c := &bodyLimitConn{inbound: []byte(raw)}
	if action := app.OnTraffic(c); action != gnet.None {
		t.Fatalf("action = %v, want None", action)
	}
	if !called {
		t.Fatal("handler was not invoked for a header block within the cap")
	}
}

func TestParseHTTPRequestHeaderLimit(t *testing.T) {
	pad := strings.Repeat("a", 256)
	raw := []byte("GET / HTTP/1.1\r\nX-Pad: " + pad + "\r\n\r\n")

	if _, _, err := parsePooledRequest(raw, true, 0, 128); err != ErrHeaderTooLarge {
		t.Fatalf("err = %v, want ErrHeaderTooLarge", err)
	}

	req, _, err := parsePooledRequest(raw, true, 0, 4096)
	if err != nil {
		t.Fatalf("within-cap parse failed: %v", err)
	}
	if req == nil {
		t.Fatal("within-cap parse returned nil request")
	}
	releaseRequest(req)
}

// Guard against a regression where an oversized declared body and a small
// header cap interact: the header check must not swallow a body-limit case.
func TestHeaderAndBodyLimitsIndependent(t *testing.T) {
	raw := []byte("POST / HTTP/1.1\r\nHost: test\r\nContent-Length: 6\r\n\r\n123456")
	if _, _, err := parsePooledRequest(raw, true, 5, 4096); err != ErrBodyTooLarge {
		t.Fatalf("err = %v, want ErrBodyTooLarge", err)
	}
	if !bytes.Contains(raw, []byte("123456")) {
		t.Fatal("raw request unexpectedly mutated")
	}
}
