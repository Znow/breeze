package breeze

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestRunOnRecordsLoopbackHost(t *testing.T) {
	port := wsTestPort(t)
	app := New(NewRouter(), NewEventLoopWorkerPool(1))

	done := make(chan error, 1)
	go func() { done <- app.RunOn("127.0.0.1", port, false) }()
	wsWaitForListener(t, port)

	if got := app.ListenHost(); got != "127.0.0.1" {
		t.Fatalf("ListenHost() = %q, want 127.0.0.1", got)
	}
	if got := app.ListenPort(); got != port {
		t.Fatalf("ListenPort() = %d, want %d", got, port)
	}

	if err := app.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() = %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunOn() = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("RunOn did not return after Stop")
	}
}

func TestRunPreservesEmptyHost(t *testing.T) {
	port := wsTestPort(t)
	app := New(NewRouter(), NewEventLoopWorkerPool(1))

	done := make(chan error, 1)
	go func() { done <- app.Run(port, false) }()
	wsWaitForListener(t, port)

	if got := app.ListenHost(); got != "" {
		t.Fatalf("ListenHost() = %q, want empty host", got)
	}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		t.Fatalf("dial to Run listener: %v", err)
	}
	_ = conn.Close()

	if err := app.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() = %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after Stop")
	}
}
