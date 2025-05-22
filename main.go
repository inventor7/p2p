package main

import (
	"fmt"
	"log"

	"github.com/inventor7/p2p/internal/api"
	"github.com/inventor7/p2p/internal/auth"
	appconfig "github.com/inventor7/p2p/internal/config"
	"github.com/inventor7/p2p/internal/index"
	"github.com/inventor7/p2p/internal/p2p"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Initialize logger with custom config for beautiful output
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.MessageKey = "message"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder

	logger, err := config.Build()
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	// Initialize config
	cfg, err := appconfig.NewConfig(logger)
	if err != nil {
		logger.Fatal("Failed to initialize config", zap.Error(err))
	}

	// Initialize services (these are placeholder - implement according to your needs)
	authService := &auth.Service{}   // Initialize your auth service
	indexService := &index.Service{} // Initialize your index service
	p2pService := &p2p.Service{}     // Initialize your p2p service

	// Initialize handlers
	authHandler := api.NewAuthHandler(logger, authService)
	indexHandler := api.NewIndexHandler(logger, indexService)
	p2pHandler := api.NewP2PHandler(logger, p2pService)

	// Initialize router
	router := api.NewRouter(cfg, logger, authHandler, indexHandler, p2pHandler)

	// Setup and start server
	ginEngine := router.Setup()

	logger.Info("Starting server", zap.Int("port", cfg.ServerPort))
	if err := ginEngine.Run(fmt.Sprintf(":%d", cfg.ServerPort)); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}
