package ipc

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ServerLogger interface {
	Info(msg string)
	Warn(msg string)
	Error(msg, errStr string)
}

type HandlerFunc func(req *Request) *Response

type Server struct {
	sockPath   string
	listener   net.Listener
	handlers   map[string]HandlerFunc
	mu         sync.RWMutex
	connLimit  chan struct{}
	shutdownCh <-chan struct{}
	log        ServerLogger
}

func NewServer(sockPath string, shutdownCh <-chan struct{}, log ServerLogger) *Server {
	return &Server{
		sockPath:   sockPath,
		handlers:   make(map[string]HandlerFunc),
		connLimit:  make(chan struct{}, 32),
		shutdownCh: shutdownCh,
		log:        log,
	}
}

func (s *Server) Handle(msgType string, fn HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[msgType] = fn
}

func (s *Server) Start() error {
	_ = os.Remove(s.sockPath)
	_ = os.MkdirAll(filepath.Dir(s.sockPath), 0755)

	listener, err := net.Listen("unix", s.sockPath)
	if err != nil {
		return fmt.Errorf("failed to listen on socket %s: %w", s.sockPath, err)
	}
	s.listener = listener

	_ = os.Chmod(s.sockPath, 0600)

	go s.acceptLoop()
	go s.watchShutdown()

	s.log.Info("IPC server listening on " + s.sockPath)
	return nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.shutdownCh:
				return
			default:
				s.log.Error("accept error", err.Error())
				return
			}
		}

		select {
		case s.connLimit <- struct{}{}:
			go s.handleConnection(conn)
		default:
			s.log.Warn("connection limit reached, rejecting connection")
			_ = conn.Close()
		}
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	defer func() { <-s.connLimit }()

	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	req, err := DecodeRequest(conn)
	if err != nil {
		resp := NewErrorResponse("invalid request", "ERR_DECODE")
		_ = EncodeResponse(conn, resp)
		return
	}

	s.mu.RLock()
	handler, ok := s.handlers[req.Type]
	s.mu.RUnlock()

	if !ok {
		resp := NewErrorResponse("unknown request type: "+req.Type, "ERR_UNKNOWN_TYPE")
		_ = EncodeResponse(conn, resp)
		return
	}

	resp := handler(req)
	if resp == nil {
		resp = NewErrorResponse("handler returned nil", "ERR_NIL_RESPONSE")
	}

	_ = EncodeResponse(conn, resp)
}

func (s *Server) watchShutdown() {
	<-s.shutdownCh
	s.Stop()
}

func (s *Server) Stop() {
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.log.Info("IPC server stopped")
}
