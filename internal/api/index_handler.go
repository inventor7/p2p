package api

import (
	"errors" // For gorm.ErrRecordNotFound
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/inventor7/p2p/internal/db"    // Your database models
	"github.com/inventor7/p2p/internal/index" // Your index service
	"github.com/inventor7/p2p/internal/p2p"   // Your p2p service for global search
	"go.uber.org/zap"
	"gorm.io/gorm" // For gorm.ErrRecordNotFound
)

// IndexHandler handles index-related HTTP requests
type IndexHandler struct {
	logger       *zap.Logger
	indexService *index.Service
	p2pService   *p2p.Service // For global file search
}

// NewIndexHandler creates a new index handler instance
func NewIndexHandler(logger *zap.Logger, indexService *index.Service, p2pService *p2p.Service) *IndexHandler {
	if logger == nil || indexService == nil || p2pService == nil {
		// Or handle more gracefully
		panic("NewIndexHandler: received nil dependency")
	}
	return &IndexHandler{
		logger:       logger,
		indexService: indexService,
		p2pService:   p2pService,
	}
}

// --- Shared Space Management ---

// CreateSpace handles POST /api/spaces
// Requires AuthMiddleware to get creator's UserID
func (h *IndexHandler) CreateSpace(c *gin.Context) {
	userID, exists := c.Get("userID") // From AuthMiddleware
	if !exists {
		h.logger.Warn("CreateSpace called without userID in context (AuthMiddleware missing or failed?)")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	creatorID := userID.(string)

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		// Color    string `json:"color"` // Optional: allow client to suggest color
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Call service method (you'll need to implement CreateSharedSpace in index.Service)
	space, err := h.indexService.CreateSharedSpace(c.Request.Context(), req.Name, req.Description, creatorID)
	if err != nil {
		h.logger.Error("Failed to create space via service", zap.Error(err), zap.String("name", req.Name), zap.String("creatorID", creatorID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create space: " + err.Error()})
		return
	}

	h.logger.Info("Space created successfully", zap.String("spaceID", space.ID), zap.String("name", space.Name))
	c.JSON(http.StatusCreated, space)
}

// ListSpaces handles GET /api/spaces
// Optionally, could be filtered by userID if spaces are not public.
// Assuming AuthMiddleware might be used to get userID for filtering if needed.
func (h *IndexHandler) ListSpaces(c *gin.Context) {
	// userID, _ := c.Get("userID") // Get userID if spaces are user-specific

	// Call service method (you'll need to implement ListSharedSpaces in index.Service)
	// If spaces are user-specific: spaces, err := h.indexService.ListUserSpaces(c.Request.Context(), userID.(string))
	spaces, err := h.indexService.ListSharedSpaces(c.Request.Context()) // Assuming lists all or public spaces
	if err != nil {
		h.logger.Error("Failed to list spaces via service", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list spaces: " + err.Error()})
		return
	}

	if spaces == nil {
		spaces = []db.SharedSpace{} // Ensure empty array instead of null
	}

	c.JSON(http.StatusOK, spaces)
}

// GetSpace handles GET /api/spaces/:id
func (h *IndexHandler) GetSpace(c *gin.Context) {
	spaceID := c.Param("id")

	// Call service method (you'll need to implement GetSpaceByID in index.Service)
	space, err := h.indexService.GetSpaceByID(c.Request.Context(), spaceID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.logger.Warn("Space not found", zap.String("spaceID", spaceID))
			c.JSON(http.StatusNotFound, gin.H{"error": "Space not found"})
			return
		}
		h.logger.Error("Failed to get space via service", zap.Error(err), zap.String("spaceID", spaceID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get space: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, space)
}

// AddMember handles POST /api/spaces/:id/members
// Requires AuthMiddleware to get the user adding the member.
// The member to be added is specified in the request body.
func (h *IndexHandler) AddMember(c *gin.Context) {
	spaceID := c.Param("id")

	var req struct {
		UserID string `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Call service method (you'll need to implement AddMemberToSpace in index.Service)
	if err := h.indexService.AddMemberToSpace(c.Request.Context(), spaceID, req.UserID); err != nil {
		// Check for specific errors like already a member or space not found
		h.logger.Error("Failed to add member to space via service", zap.Error(err), zap.String("spaceID", spaceID), zap.String("userID", req.UserID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add member: " + err.Error()})
		return
	}

	h.logger.Info("Member added to space successfully", zap.String("spaceID", spaceID), zap.String("userID", req.UserID))
	c.JSON(http.StatusOK, gin.H{"message": "Member added successfully"})
}

// RemoveMember handles DELETE /api/spaces/:id/members/:userId
// Requires AuthMiddleware to get the user performing the removal.
func (h *IndexHandler) RemoveMember(c *gin.Context) {
	spaceID := c.Param("id")
	userIDToRemove := c.Param("userId")

	// Optional: Get the user ID performing the action from AuthMiddleware
	// actionUserID, exists := c.Get("userID")
	// if !exists { ... handle unauthorized ... }
	// actionUserIDStr := actionUserID.(string)

	// Call service method (you'll need to implement RemoveFromSpace in index.Service)
	// You might pass actionUserIDStr to the service to check permissions
	if err := h.indexService.RemoveFromSpace(c.Request.Context(), spaceID, userIDToRemove, "member"); err != nil {
		// Check for specific errors like member not found, space not found, or permission denied
		h.logger.Error("Failed to remove member from space via service", zap.Error(err), zap.String("spaceID", spaceID), zap.String("userID", userIDToRemove))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member: " + err.Error()})
		return
	}

	h.logger.Info("Member removed from space successfully", zap.String("spaceID", spaceID), zap.String("userID", userIDToRemove))
	c.JSON(http.StatusOK, gin.H{"message": "Member removed successfully"})
}

// AddFile handles POST /api/spaces/:id/files
// Requires AuthMiddleware to get the user adding the file.
// The file to be added is specified in the request body (likely by FileID).
func (h *IndexHandler) AddFile(c *gin.Context) {
	spaceID := c.Param("id")

	var req struct {
		FileID string `json:"file_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Call service method (you'll need to implement AddFileToSpace in index.Service)
	if err := h.indexService.AddFileToSpace(c.Request.Context(), spaceID, req.FileID); err != nil {
		// Check for specific errors like file already in space, space not found, or file not found
		h.logger.Error("Failed to add file to space via service", zap.Error(err), zap.String("spaceID", spaceID), zap.String("fileID", req.FileID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add file: " + err.Error()})
		return
	}

	h.logger.Info("File added to space successfully", zap.String("spaceID", spaceID), zap.String("fileID", req.FileID))
	c.JSON(http.StatusOK, gin.H{"message": "File added successfully"})
}

// RemoveFile handles DELETE /api/spaces/:id/files/:fileId
// Requires AuthMiddleware to get the user performing the removal.
func (h *IndexHandler) RemoveFile(c *gin.Context) {
	spaceID := c.Param("id")
	fileIDToRemove := c.Param("fileId")

	// Optional: Get the user ID performing the action from AuthMiddleware
	// actionUserID, exists := c.Get("userID")
	// if !exists { ... handle unauthorized ... }
	// actionUserIDStr := actionUserID.(string)

	// Call service method (you'll need to implement RemoveFromSpace in index.Service)
	// You might pass actionUserIDStr to the service to check permissions
	if err := h.indexService.RemoveFromSpace(c.Request.Context(), spaceID, fileIDToRemove, "file"); err != nil {
		// Check for specific errors like file not found in space, space not found, or permission denied
		h.logger.Error("Failed to remove file from space via service", zap.Error(err), zap.String("spaceID", spaceID), zap.String("fileID", fileIDToRemove))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove file: " + err.Error()})
		return
	}

	h.logger.Info("File removed from space successfully", zap.String("spaceID", spaceID), zap.String("fileID", fileIDToRemove))
	c.JSON(http.StatusOK, gin.H{"message": "File removed successfully"})
}

// GetFiles handles GET /api/spaces/:id/files
func (h *IndexHandler) GetFiles(c *gin.Context) {
	spaceID := c.Param("id")

	// Call service method (you'll need to implement GetSpaceFiles in index.Service)
	files, err := h.indexService.GetSpaceFiles(c.Request.Context(), spaceID)
	if err != nil {
		h.logger.Error("Failed to get files for space via service", zap.Error(err), zap.String("spaceID", spaceID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get files: " + err.Error()})
		return
	}

	if files == nil {
		files = []*db.File{} // Ensure empty array instead of null
	}

	h.logger.Info("Fetched files for space", zap.String("spaceID", spaceID), zap.Int("count", len(files)))
	c.JSON(http.StatusOK, gin.H{"files": files})
}

// GetMembers handles GET /api/spaces/:id/members
func (h *IndexHandler) GetMembers(c *gin.Context) {
	spaceID := c.Param("id")

	// Call service method (you'll need to implement GetSpaceMembers in index.Service)
	members, err := h.indexService.GetSpaceMembers(c.Request.Context(), spaceID)
	if err != nil {
		h.logger.Error("Failed to get members for space via service", zap.Error(err), zap.String("spaceID", spaceID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get members: " + err.Error()})
		return
	}

	if members == nil {
		members = []*db.User{} // Ensure empty array instead of null
	}

	h.logger.Info("Fetched members for space", zap.String("spaceID", spaceID), zap.Int("count", len(members)))
	c.JSON(http.StatusOK, gin.H{"members": members})
}

// SearchFiles handles file search across the P2P network
func (h *IndexHandler) SearchFiles(c *gin.Context) {
	query := c.Query("q") // Get search query from query parameter "q"
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Search query 'q' is required"})
		return
	}

	// Use p2pService for global file search
	files, err := h.p2pService.SearchSharedFiles(c.Request.Context(), query)
	if err != nil {
		h.logger.Error("Failed to search files via P2P service", zap.Error(err), zap.String("query", query))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search files: " + err.Error()})
		return
	}

	if files == nil {
		files = []*p2p.FileSearchResult{} // Return empty array instead of null if no files found
	}

	h.logger.Info("File search performed", zap.String("query", query), zap.Int("results_count", len(files)))
	c.JSON(http.StatusOK, gin.H{"files": files})
}
