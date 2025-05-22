package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inventor7/p2p/internal/config"
	"go.uber.org/zap"
)

// Router handles HTTP routing and middleware
type Router struct {
	cfg          *config.Config
	logger       *zap.Logger
	authHandler  *AuthHandler
	indexHandler *IndexHandler
	p2pHandler   *P2PHandler
}

// NewRouter creates a new router instance
func NewRouter(
	cfg *config.Config,
	logger *zap.Logger,
	authHandler *AuthHandler,
	indexHandler *IndexHandler,
	p2pHandler *P2PHandler,
) *Router {
	return &Router{
		cfg:          cfg,
		logger:       logger,
		authHandler:  authHandler,
		indexHandler: indexHandler,
		p2pHandler:   p2pHandler,
	}
}

// Setup configures the HTTP router
func (r *Router) Setup() *gin.Engine {
	// Create router
	router := gin.New()

	// Middleware
	router.Use(gin.Recovery())
	router.Use(corsMiddleware(r.cfg.AllowedOrigins))
	router.Use(loggerMiddleware(r.logger))

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// API routes
	api := router.Group("/api")
	{
		// Auth routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", r.authHandler.Register)
			auth.POST("/connect", r.authHandler.Connect)
			auth.POST("/disconnect", r.authHandler.AuthMiddleware(), r.authHandler.Disconnect)
		}

		// Protected routes
		protected := api.Group("/")
		protected.Use(r.authHandler.AuthMiddleware())
		{
			// P2P routes
			p2p := protected.Group("/p2p")
			{
				p2p.GET("/peers", r.p2pHandler.GetPeers)
				p2p.POST("/peers/:id/connect", r.p2pHandler.ConnectToPeer)
				p2p.POST("/peers/:id/disconnect", r.p2pHandler.DisconnectPeer)
				p2p.GET("/peers/:id/files", r.p2pHandler.GetPeerFiles)
				p2p.POST("/files/share", r.p2pHandler.ShareFile)
				p2p.POST("/network/join", r.p2pHandler.JoinNetwork)
			}

			// Index routes
			spaces := protected.Group("/spaces")
			{
				spaces.POST("/", r.indexHandler.CreateSpace)
				spaces.GET("/", r.indexHandler.ListSpaces)
				spaces.GET("/:id", r.indexHandler.GetSpace)
				spaces.POST("/:id/members", r.indexHandler.AddMember)
				spaces.DELETE("/:id/members/:userId", r.indexHandler.RemoveMember)
				spaces.POST("/:id/files", r.indexHandler.AddFile)
				spaces.DELETE("/:id/files/:fileId", r.indexHandler.RemoveFile)
				spaces.GET("/:id/files", r.indexHandler.GetFiles)
				spaces.GET("/:id/members", r.indexHandler.GetMembers)
			}

			// Search routes
			search := protected.Group("/search")
			{
				search.GET("/files", r.indexHandler.SearchFiles)
			}
		}
	}

	return router
}

// corsMiddleware handles CORS
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// loggerMiddleware handles request logging
func loggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Start timer
		start := time.Now()

		// Process request
		c.Next()

		// Log request
		latency := time.Since(start)
		logger.Info("HTTP Request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}
