package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
	"warden/internal/logger"
)

type Server struct {
	srv *http.Server
}

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
func (s *Server) Shutdown(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.srv.Shutdown(ctx)
}
