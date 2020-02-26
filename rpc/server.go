package rpc

import (
	"sync"

	"gitlab.com/makeos/mosdef/config"
	"gitlab.com/makeos/mosdef/pkgs/logger"
	"gitlab.com/makeos/mosdef/util"
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
	sync.RWMutex

	// addr is the address to bind the server to
	addr string

	// cfg is the engine config
	cfg *config.AppConfig

	// log is the logger
	log logger.Logger

	// rpc is the JSONRPC 2.0 server
	rpc *JSONRPC

	// started indicates the start state of the server
	started bool

	interrupt *util.Interrupt
}

// NewServer creates a new RPC server
func NewServer(cfg *config.AppConfig, log logger.Logger,
	interrupt *util.Interrupt) *Server {
	srv := &Server{
		addr:      cfg.RPC.Address,
		log:       log,
		cfg:       cfg,
		rpc:       New(cfg.RPC.Address, cfg, log),
		interrupt: interrupt,
	}
	return srv
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
			s.interrupt.Wait()
			s.Stop()
		}
	}()

	s.Lock()
	s.started = true
	s.Unlock()

	s.log.Info("RPC service started", "Address", s.addr)
	s.rpc.Serve()
}

// IsRunning returns the start state
func (s *Server) IsRunning() bool {
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
func (s *Server) AddAPI(apis ...APISet) {
	s.Lock()
	defer s.Unlock()
	s.rpc.MergeAPISet(apis...)
}

// GetMethods returns all registered RPC methods
func (s *Server) GetMethods() []MethodInfo {
	return s.rpc.Methods()
}
