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
		// Auth routes: These might be repurposed for an *admin* login to the super-peer,
		// or removed if no such admin functionality is needed via these specific routes.
		auth := api.Group("/auth")
		{
			// auth.POST("/register", r.authHandler.Register) // If you need admin accounts
			// auth.POST("/connect", r.authHandler.Connect)   // If you need admin accounts
			auth.POST("/disconnect", r.authHandler.AuthMiddleware(), r.authHandler.Disconnect) // For admin, still requires auth
		}

		// P2P routes for peer interactions - Public or PeerID based
		p2p := api.Group("/p2p")
		{
			p2p.POST("/join", r.p2pHandler.JoinNetwork)            // Peer announces itself - Public
			p2p.POST("/leave", r.p2pHandler.LeaveNetwork)          // Peer announces departure - Needs PeerID (via header)
			p2p.POST("/files/share", r.p2pHandler.ShareFile)       // Peer shares file metadata - Needs PeerID (via header)
			p2p.GET("/peers", r.p2pHandler.GetPeers)               // List active peers - Public or PeerID based
			p2p.GET("/peers/:id/files", r.p2pHandler.GetPeerFiles) // Get files for a specific peer ID

			// These are likely for initiating direct P2P, so they might not be actual handlers
			// on the super-peer but more conceptual for the client.
			// p2p.POST("/peers/:id/connect", r.p2pHandler.ConnectToPeer)
			// p2p.POST("/peers/:id/disconnect", r.p2pHandler.DisconnectPeer)
		}

		// Index/Search routes
		// index := api.Group("/index")
		// {
		// 	index.GET("/search", r.indexHandler.SearchFiles) // Search files - Public or needs PeerID
		// }

		searchGroup := api.Group("/search")
		{
			searchGroup.GET("/files", r.indexHandler.SearchFiles) // This will be /api/search/files
		}

		// "Protected" routes using JWT AuthMiddleware would now be for specific
		// features that DO require user login (e.g., managing shared spaces), or admin functions.
		protected := api.Group("") // This group might be empty if all routes become public/peer-id based
		protected.Use(r.authHandler.AuthMiddleware())
		{
			// Index routes (for Shared Spaces - assuming these still require traditional user auth)
			spaces := protected.Group("/spaces") // Assuming this group uses r.authHandler.AuthMiddleware()
			{
				spaces.POST("/", r.indexHandler.CreateSpace)                       // Create a new space
				spaces.GET("/", r.indexHandler.ListSpaces)                         // List all spaces (user has access to)
				spaces.GET("/:id", r.indexHandler.GetSpace)                        // Get a specific space by ID
				spaces.POST("/:id/members", r.indexHandler.AddMember)              // Add a member to a space
				spaces.DELETE("/:id/members/:userId", r.indexHandler.RemoveMember) // Remove a member from a space
				spaces.POST("/:id/files", r.indexHandler.AddFile)                  // Add a file to a space
				spaces.DELETE("/:id/files/:fileId", r.indexHandler.RemoveFile)     // Remove a file from a space
				spaces.GET("/:id/files", r.indexHandler.GetFiles)                  // List files in a space
				spaces.GET("/:id/members", r.indexHandler.GetMembers)              // List members of a space
			}
		}
	}

	return router
}

// corsMiddleware handles CORS
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {

	// TODO: Implement a cors

	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Peer-ID")
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
