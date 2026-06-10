package ipc

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type noopLogger struct{}

func (n *noopLogger) Info(msg string)           {}
func (n *noopLogger) Warn(msg string)           {}
func (n *noopLogger) Error(msg, e string)       {}

func setupServer(t *testing.T) (*Server, string, chan struct{}) {
	sockPath := filepath.Join(t.TempDir(), "test.sock")
	shutdownCh := make(chan struct{})
	server := NewServer(sockPath, shutdownCh, &noopLogger{})
	
	err := server.Start()
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	t.Cleanup(func() {
		defer func() { recover() }()
		close(shutdownCh)
		time.Sleep(50 * time.Millisecond)
	})
	
	return server, sockPath, shutdownCh
}

func TestServerClient_PingHandler(t *testing.T) {
	server, sockPath, _ := setupServer(t)

	server.Handle(TypePing, func(req *Request) *Response {
		data, _ := NewOKResponse(PingData{PID: os.Getpid(), Version: "test"})
		return data
	})

	client := NewClient(sockPath)
	data, err := client.Ping()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if data.PID != os.Getpid() {
		t.Errorf("expected PID %d, got %d", os.Getpid(), data.PID)
	}
	if data.Version != "test" {
		t.Errorf("expected Version 'test', got '%s'", data.Version)
	}
}

func TestServerClient_UnknownType(t *testing.T) {
	_, sockPath, _ := setupServer(t)

	client := NewClient(sockPath)
	req, _ := NewRequest("unknown_type", PingPayload{})
	
	resp, err := client.Send(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != StatusError {
		t.Fatalf("expected StatusError, got %s", resp.Status)
	}

	errData, _ := ParseErrorData(resp)
	if errData.Code != "ERR_UNKNOWN_TYPE" {
		t.Errorf("expected ERR_UNKNOWN_TYPE code, got %s", errData.Code)
	}
}

func TestServerClient_InvalidJSON(t *testing.T) {
	_, sockPath, _ := setupServer(t)

	conn, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte("this is not json\n"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp, err := DecodeResponse(conn)
	if err != nil {
		t.Fatalf("unexpected error decoding response: %v", err)
	}

	if resp.Status != StatusError {
		t.Fatalf("expected error status, got %s", resp.Status)
	}
}

func TestServerClient_ConcurrentRequests(t *testing.T) {
	server, sockPath, _ := setupServer(t)

	server.Handle(TypePing, func(req *Request) *Response {
		data, _ := NewOKResponse(PingData{PID: os.Getpid(), Version: "test"})
		return data
	})

	var wg sync.WaitGroup
	errs := make(chan error, 50)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := NewClient(sockPath)
			for j := 0; j < 5; j++ {
				_, err := client.Ping()
				if err != nil {
					errs <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("concurrent request error: %v", err)
		}
	}
}

func TestServerClient_Timeout(t *testing.T) {
	server, sockPath, _ := setupServer(t)

	server.Handle("slow", func(req *Request) *Response {
		time.Sleep(10 * time.Second)
		return nil
	})

	client := &Client{sockPath: sockPath, timeout: 1 * time.Second}
	req, _ := NewRequest("slow", PingPayload{})

	_, err := client.Send(req)
	if err == nil {
		t.Fatalf("expected error due to timeout, got nil")
	}
}

func TestClient_DaemonNotRunning(t *testing.T) {
	client := NewClient(filepath.Join(t.TempDir(), "nonexistent.sock"))
	
	if client.IsDaemonReachable() {
		t.Errorf("expected IsDaemonReachable to be false")
	}

	_, err := client.Ping()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestServerShutdown_ClosesSocket(t *testing.T) {
	server, sockPath, shutdownCh := setupServer(t)

	server.Handle(TypePing, func(req *Request) *Response {
		data, _ := NewOKResponse(PingData{})
		return data
	})

	client := NewClient(sockPath)
	if !client.IsDaemonReachable() {
		t.Fatalf("expected daemon to be reachable")
	}

	close(shutdownCh)
	time.Sleep(100 * time.Millisecond)

	if client.IsDaemonReachable() {
		t.Errorf("expected IsDaemonReachable to be false after shutdown")
	}
}
