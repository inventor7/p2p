package main

import (
	"fmt"
	"log" // Standard log for initial critical errors before zap is up

	"github.com/inventor7/p2p/internal/api"
	"github.com/inventor7/p2p/internal/auth"
	appconfig "github.com/inventor7/p2p/internal/config"
	"github.com/inventor7/p2p/internal/db"
	"github.com/inventor7/p2p/internal/index"
	"github.com/inventor7/p2p/internal/p2p"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// --- Configure Zap Logger ---
	zapDevConfig := zap.NewDevelopmentConfig()

	// Make it more concise like standard log, but with color and levels
	zapDevConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05") // Just time, no date
	zapDevConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zapDevConfig.EncoderConfig.CallerKey = "" // Omit "caller" field for brevity
	// zapDevConfig.EncoderConfig.NameKey = ""      // Omit "logger" name field
	// zapDevConfig.EncoderConfig.StacktraceKey = "" // Omit stacktrace key from non-error logs

	// Customize which fields are present
	zapDevConfig.EncoderConfig.TimeKey = "T" // Shorter key for time
	zapDevConfig.EncoderConfig.LevelKey = "L"
	zapDevConfig.EncoderConfig.MessageKey = "M"
	// zapDevConfig.EncoderConfig.NameKey = "" // Remove logger name if not needed
	// zapDevConfig.EncoderConfig.CallerKey = "" // Remove caller if too verbose

	logger, err := zapDevConfig.Build(zap.AddStacktrace(zapcore.ErrorLevel)) // Add stacktrace only for Error and above
	if err != nil {
		log.Fatal("Failed to initialize zap logger:", err)
	}
	defer logger.Sync()

	// --- Initialize App Config ---
	cfg, err := appconfig.NewConfig(logger)
	if err != nil {
		logger.Fatal("Failed to initialize app config", zap.Error(err))
	}

	// --- Initialize Database ---
	database, err := db.NewDatabase(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	logger.Info("Database connected and migrations run successfully")

	// --- Initialize Services ---
	authSvc := auth.NewService(cfg, database, logger)
	p2pSvc := p2p.NewService(cfg, database, logger)
	indexSvc := index.NewService(cfg, database, logger)
	logger.Info("All services initialized")

	// --- Initialize Handlers ---
	authHandler := api.NewAuthHandler(logger, authSvc)
	indexHandler := api.NewIndexHandler(logger, indexSvc, p2pSvc)
	p2pHandler := api.NewP2PHandler(logger, p2pSvc)
	logger.Info("All handlers initialized")

	// --- Initialize Router ---
	router := api.NewRouter(cfg, logger, authHandler, indexHandler, p2pHandler)
	ginEngine := router.Setup()
	logger.Info("Router initialized and Gin engine setup complete")

	// --- Start Server ---
	serverAddr := fmt.Sprintf(":%d", cfg.ServerPort)
	logger.Info("Starting server", zap.String("address", serverAddr))
	if err := ginEngine.Run(serverAddr); err != nil {
		logger.Fatal("Failed to start server", zap.Error(err))
	}
}
