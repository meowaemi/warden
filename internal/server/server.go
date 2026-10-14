// Package server runs an HTTP server and stops it gracefully.
package server

import (
	"context"
	"errors"
	"net/http"
	"time"
	"warden/internal/logger"
)

// Server is an HTTP server that can be started and gracefully stopped.
type Server struct {
	srv *http.Server
}

// New returns a server that will listen on addr and serve h.
func New(addr string, h http.Handler) *Server {
	return &Server{srv: &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}}
}

// Start runs the server in its own goroutine and returns immediately.
// If the server fails, the error is logged and onFail is called, so the
// caller can stop the rest of the program.
func (s *Server) Start(onFail func()) {
	go func() {
		logger.Log.Info("http server listening", "addr", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Error("http server failed", "err", err)
			onFail()
		}
	}()
}

// Shutdown stops accepting new connections and waits up to timeout for
// running requests to finish.
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
