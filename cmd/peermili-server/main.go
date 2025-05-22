package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/peermili/backend/internal/api"
	"github.com/peermili/backend/internal/auth"
	"github.com/peermili/backend/internal/config"
	"github.com/peermili/backend/internal/db"
	"github.com/peermili/backend/internal/index"
	"github.com/peermili/backend/internal/p2p"
	"github.com/peermili/backend/pkg/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// Initialize application with dependency injection
	app := fx.New(
		// Provide core dependencies
		fx.Provide(
			context.Background,
			config.NewConfig,
			logger.NewLogger,
			db.NewDatabase,
		),

		// Provide service modules
		fx.Provide(
			auth.NewService,
			index.NewService,
			p2p.NewService,
		),

		// Provide API handlers
		fx.Provide(
			api.NewRouter,
			api.NewAuthHandler,
			api.NewIndexHandler,
			api.NewP2PHandler,
		),

		// Register lifecycle hooks
		fx.Invoke(func(*zap.Logger) {}), // Ensure logger is initialized
		fx.Invoke(func(*api.Router) {}), // Start HTTP server
	)

	// Start the application
	if err := app.Start(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Wait for interrupt signal
	<-app.Done()

	// Graceful shutdown
	if err := app.Stop(context.Background()); err != nil {
		log.Fatal(err)
	}

	os.Exit(0)
}
