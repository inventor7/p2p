package main

import (
	"context"
	"log"
	"os"

	"github.com/inventor7/p2p/internal/api"
	"github.com/inventor7/p2p/internal/auth"
	"github.com/inventor7/p2p/internal/config"
	"github.com/inventor7/p2p/internal/db"
	"github.com/inventor7/p2p/internal/index"
	"github.com/inventor7/p2p/internal/p2p"
	"github.com/joho/godotenv"
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
			db.NewDatabase,
		),

		// Provide service modules
		fx.Provide(
			auth.NewService,
			index.NewService,
			p2p.NewService,
		),

		// Provide API handlers and server
		fx.Provide(
			api.NewAuthHandler,
			api.NewIndexHandler,
			api.NewP2PHandler,
			api.NewRouter,
			api.NewServer,
		),

		// Invoke server start
		fx.Invoke(func(server *api.Server, lc fx.Lifecycle) {
			server.Start(lc)
		}),

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
