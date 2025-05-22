package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/inventor7/p2p/internal/auth"
	"go.uber.org/zap"
)

// AuthHandler handles authentication related HTTP requests
type AuthHandler struct {
	logger  *zap.Logger
	service *auth.Service
}

// NewAuthHandler creates a new auth handler instance
func NewAuthHandler(logger *zap.Logger, service *auth.Service) *AuthHandler {
	return &AuthHandler{
		logger:  logger,
		service: service,
	}
}

// Register handles user registration
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	token, err := h.service.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		h.logger.Error("Failed to register user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// Connect handles user login
func (h *AuthHandler) Connect(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	token, err := h.service.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		h.logger.Error("Failed to login user", zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
}

// Disconnect handles user logout
func (h *AuthHandler) Disconnect(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	if err := h.service.Logout(c.Request.Context(), userID.(string)); err != nil {
		h.logger.Error("Failed to logout user", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
}

// AuthMiddleware handles authentication for protected routes
func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "No authorization header"})
			return
		}

		userID, err := h.service.ValidateToken(c.Request.Context(), auth)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}
