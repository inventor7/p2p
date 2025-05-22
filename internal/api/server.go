package api

import (
	"context"
	"net/http"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Server represents the HTTP server
type Server struct {
	logger *zap.Logger
	router *Router
}

// NewServer creates a new server instance
func NewServer(logger *zap.Logger, router *Router) *Server {
	return &Server{
		logger: logger,
		router: router,
	}
}

// Start initializes and starts the HTTP server
func (s *Server) Start(lc fx.Lifecycle) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			gin := s.router.Setup()
			server := &http.Server{
				Addr:    ":8080",
				Handler: gin,
			}

			go func() {
				s.logger.Info("Starting server on :8080")
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					s.logger.Error("Failed to start server", zap.Error(err))
				}
			}()

			return nil
		},
		OnStop: func(ctx context.Context) error {
			s.logger.Info("Shutting down server")
			return nil
		},
	})
}
