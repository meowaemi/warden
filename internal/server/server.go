// Package server runs an HTTP server with sane timeouts.
package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
	"warden/internal/logger"
)

// Server is an HTTP server.
type Server struct {
	srv *http.Server
}

// New returns a server for h on addr.
func New(addr string, h http.Handler) *Server {
	return &Server{srv: &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}}
}

// Start listens at once, so a busy port is an error here, then serves in the
// background; onFail is called if serving fails later.
func (s *Server) Start(onFail func()) error {
	listener, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return err
	}
	go func() {
		logger.Log.Info("http server listening", "addr", s.srv.Addr)
		if err := s.srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("http server failed", "err", err)
			onFail()
		}
	}()
	return nil
}

// Shutdown stops the server, waiting up to timeout for open requests.
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
