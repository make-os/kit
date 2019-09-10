package rpc

import (
	"sync"

	"github.com/makeos/mosdef/config"
	"github.com/makeos/mosdef/rpc/jsonrpc"
	"github.com/makeos/mosdef/util/logger"
)

// Result represent a response to a service method call
type Result struct {
	Error   string
	ErrCode int
	Status  int
	Data    map[string]interface{}
}

// Server represents a rpc server
type Server struct {
	interrupt <-chan struct{}
	sync.RWMutex

	// addr is the address to bind the server to
	addr string

	// cfg is the engine config
	cfg *config.EngineConfig

	// log is the logger
	log logger.Logger

	// rpc is the JSONRPC 2.0 server
	rpc *jsonrpc.JSONRPC

	// started indicates the start state
	// of the server
	started bool
}

// NewServer creates a new RPC server
func NewServer(addr string, cfg *config.EngineConfig, log logger.Logger,
	interrupt <-chan struct{}) *Server {
	return &Server{
		addr:      addr,
		log:       log,
		cfg:       cfg,
		rpc:       jsonrpc.New(log, addr),
		interrupt: interrupt,
	}
}

// GetAddr gets the address
func (s *Server) GetAddr() string {
	s.RLock()
	defer s.RUnlock()
	return s.addr
}

// Serve starts the server
func (s *Server) Serve() {
	go func() {
		if s.interrupt != nil {
			<-s.interrupt
			s.Stop()
		}
	}()

	s.AddAPI(s.APIs())
	s.Lock()
	s.started = true
	s.Unlock()
	s.log.Info("RPC service started", "Address", s.addr)
	s.rpc.Serve()
}

// IsStarted returns the start state
func (s *Server) IsStarted() bool {
	s.RLock()
	defer s.RUnlock()
	return s.started
}

// Stop stops the server and frees resources
func (s *Server) Stop() {
	s.Lock()
	defer s.Unlock()
	if !s.started {
		return
	}
	s.rpc.Stop()
	s.started = false
	s.log.Info("RPC service has stopped")
}

// AddAPI adds one or more API sets
func (s *Server) AddAPI(apis ...jsonrpc.APISet) {
	s.Lock()
	defer s.Unlock()
	s.rpc.MergeAPISet(apis...)
}
