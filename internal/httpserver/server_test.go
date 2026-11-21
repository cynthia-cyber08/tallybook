package httpserver

import (
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestNew_RejectsDisabledTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeouts Timeouts
	}{
		{"read header", Timeouts{Read: time.Second, Write: time.Second, Idle: time.Second}},
		{"read", Timeouts{ReadHeader: time.Second, Write: time.Second, Idle: time.Second}},
		{"write", Timeouts{ReadHeader: time.Second, Read: time.Second, Idle: time.Second}},
		{"idle", Timeouts{ReadHeader: time.Second, Read: time.Second, Write: time.Second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(":0", http.NotFoundHandler(), tt.timeouts); err == nil {
				t.Fatal("New accepted a disabled timeout")
			}
		})
	}
}

func TestReadHeaderTimeoutClosesPartialRequest(t *testing.T) {
	server, err := New("", http.NotFoundHandler(), Timeouts{
		ReadHeader: 50 * time.Millisecond,
		Read:       time.Second,
		Write:      time.Second,
		Idle:       time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	t.Cleanup(func() {
		if err := server.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("close server: %v", err)
		}
		<-serveErr
	})

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: example.test\r\nX-Slow:"); err != nil {
		t.Fatal(err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, conn); err != nil {
		t.Fatalf("read response after server closed partial request: %v", err)
	}
}
