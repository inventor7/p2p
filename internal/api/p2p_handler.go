package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/inventor7/p2p/internal/db" // Assuming db.File will be used for FileShareRequest
	"github.com/inventor7/p2p/internal/p2p"
	"go.uber.org/zap"
)

// JoinNetworkRequest defines the structure for the join network request
type JoinNetworkRequest struct {
	PeerName        string `json:"peer_name" binding:"required"`
	ListenPort      int    `json:"listen_port" binding:"required"`
	is_super_client bool   `json:"is_super"`
	// IPAddress might be inferred by the server or provided if complex network
}

// FileShareRequest defines the structure for sharing file metadata
type FileShareRequest struct {
	FileName string `json:"file_name" binding:"required"`
	FileSize int64  `json:"file_size" binding:"required"`
	FileHash string `json:"file_hash" binding:"required"`
	// Potentially other metadata like OwnerID (which would be the peerID)
}

// P2PHandler handles peer-to-peer HTTP requests
type P2PHandler struct {
	logger  *zap.Logger
	service *p2p.Service
}

// NewP2PHandler creates a new P2P handler instance
func NewP2PHandler(logger *zap.Logger, service *p2p.Service) *P2PHandler {
	return &P2PHandler{
		logger:  logger,
		service: service,
	}
}

// JoinNetwork handles a peer joining the network
func (h *P2PHandler) JoinNetwork(c *gin.Context) {
	var req JoinNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	peerIP := c.ClientIP() // Get client's IP as seen by the server

	// Call the p2p service to register the peer
	// Assuming isSuper is false for regular peers joining via this endpoint.
	// If super-peer registration needs a different flow/params, that would be separate.
	user, err := h.service.RegisterPeer(c.Request.Context(), req.PeerName, peerIP, req.ListenPort, req.is_super_client)
	if err != nil {
		h.logger.Error("Failed to register peer in service", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join network: " + err.Error()})
		return
	}

	h.logger.Info("Peer joined network",
		zap.String("peerID", user.ID),
		zap.String("peerName", req.PeerName),
		zap.String("peerIP", peerIP),
		zap.Int("listenPort", req.ListenPort),
	)

	c.JSON(http.StatusOK, gin.H{
		"message":   "Successfully joined network",
		"peer_id":   user.ID, // Return the peer_id assigned by the service
		"your_ip":   peerIP,
		"your_port": req.ListenPort,
		"is_super":  req.is_super_client,
	})
}

// ShareFile handles a peer sharing file metadata with the super-peer
func (h *P2PHandler) ShareFile(c *gin.Context) {
	peerID := c.GetHeader("X-Peer-ID")
	if peerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing X-Peer-ID header. Join network first."})
		return
	}

	var req FileShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}

	// Create a db.File object from the request
	file := &db.File{
		ID:      uuid.New().String(), // Generate new file ID
		Name:    req.FileName,
		Size:    req.FileSize,
		Hash:    req.FileHash,
		OwnerID: peerID, // Associate file with the peer
		// Type and Path might need to be handled differently or omitted for metadata-only sharing
	}

	if err := h.service.ShareFile(c.Request.Context(), peerID, file); err != nil {
		h.logger.Error("Failed to share file", zap.Error(err), zap.String("peerID", peerID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to share file: " + err.Error()})
		return
	}

	h.logger.Info("Peer shared file metadata",
		zap.String("peerID", peerID),
		zap.String("fileName", req.FileName),
		zap.String("fileID", file.ID),
	)

	c.JSON(http.StatusOK, gin.H{"message": "File metadata shared successfully", "file_id": file.ID})
}

// LeaveNetwork handles a peer announcing its departure.
func (h *P2PHandler) LeaveNetwork(c *gin.Context) {
	peerID := c.GetHeader("X-Peer-ID") // Or get from request body/param
	if peerID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing X-Peer-ID header or peer identifier."})
		return
	}

	if err := h.service.DisconnectPeer(c.Request.Context(), peerID); err != nil {
		h.logger.Error("Failed to disconnect peer", zap.Error(err), zap.String("peerID", peerID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave network: " + err.Error()})
		return
	}

	h.logger.Info("Peer left network", zap.String("peerID", peerID))
	c.JSON(http.StatusOK, gin.H{"message": "Successfully left network"})
}

func (h *P2PHandler) GetPeers(c *gin.Context) {
	if h.service == nil {
		h.logger.Error("P2P Handler has a nil service instance in GetPeers")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: service not initialized"})
		return
	}

	activePeers, err := h.service.GetActivePeers(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get active peers from service", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve peers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		// "message": "Successfully retrieved active peers", // Optional message
		"peers": activePeers, // Ensure this key matches what frontend expects
	})
}

// GetPeerFiles handles getting files shared by a specific peer (metadata from p2p service)
func (h *P2PHandler) GetPeerFiles(c *gin.Context) {
	peerID := c.Param("id")
	if peerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Peer ID is required"})
		return
	}

	files, err := h.service.GetPeerFiles(c.Request.Context(), peerID)
	if err != nil {
		h.logger.Error("Failed to get peer files", zap.Error(err), zap.String("peerID", peerID))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve peer files: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"peer_id": peerID, "files": files})
}

// ConnectToPeer and DisconnectPeer might not be actual super-peer handlers
// if connections are direct P2P. They are listed for conceptual completeness from the prompt.
// If they were to be proxied or managed by the super-peer, their implementation would go here.

// ConnectToPeer handles connecting to a peer (conceptual, likely not a super-peer endpoint)
func (h *P2PHandler) ConnectToPeer(c *gin.Context) {
	peerID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"message": "ConnectToPeer (conceptual) for peer " + peerID})
}

// DisconnectPeer handles disconnecting from a peer (conceptual)
func (h *P2PHandler) DisconnectPeer(c *gin.Context) {
	peerID := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"message": "DisconnectPeer (conceptual) for peer " + peerID})
}
