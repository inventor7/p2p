package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/inventor7/p2p/internal/config"
	"github.com/inventor7/p2p/internal/db"
	"go.uber.org/zap"
)

// Service handles P2P networking and file transfer functionality
type Service struct {
	cfg    *config.Config
	db     *db.Database
	logger *zap.Logger

	// In-memory state
	peers      map[string]*PeerConnection
	superPeers map[string]*PeerConnection
	mu         sync.RWMutex
}

// PeerConnection represents an active peer connection
type PeerConnection struct {
	User       *db.User
	LastPing   time.Time
	Files      map[string]*db.File // Local cache of shared files
	IsActive   bool
	Disconnect chan struct{}
}

// NewService creates a new P2P service instance
func NewService(cfg *config.Config, database *db.Database, logger *zap.Logger) *Service {
	return &Service{
		cfg:        cfg,
		db:         database,
		logger:     logger,
		peers:      make(map[string]*PeerConnection),
		superPeers: make(map[string]*PeerConnection),
	}
}

// RegisterPeer registers a new peer in the network
func (s *Service) RegisterPeer(ctx context.Context, username string, isSuper bool) (*db.User, error) {
	// Create new user record
	user := &db.User{
		ID:       uuid.New().String(),
		Username: username,
		IsSuper:  isSuper,
		LastSeen: time.Now(),
	}

	// Save to database
	if err := s.db.GetDB().Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to register peer: %w", err)
	}

	// Initialize peer connection
	conn := &PeerConnection{
		User:       user,
		LastPing:   time.Now(),
		Files:      make(map[string]*db.File),
		IsActive:   true,
		Disconnect: make(chan struct{}),
	}

	// Add to appropriate peer map
	s.mu.Lock()
	if isSuper {
		s.superPeers[user.ID] = conn
	} else {
		s.peers[user.ID] = conn
	}
	s.mu.Unlock()

	// Start heartbeat monitoring
	go s.monitorPeerConnection(conn)

	return user, nil
}

// ShareFile makes a file available for sharing
func (s *Service) ShareFile(ctx context.Context, userID string, file *db.File) error {
	// Validate file size and type
	if file.Size > s.cfg.MaxFileSize {
		return fmt.Errorf("file size exceeds maximum allowed size of %d bytes", s.cfg.MaxFileSize)
	}

	// Save file metadata to database
	if err := s.db.GetDB().Create(file).Error; err != nil {
		return fmt.Errorf("failed to save file metadata: %w", err)
	}

	// Update peer's shared files cache
	s.mu.Lock()
	if peer, exists := s.peers[userID]; exists {
		peer.Files[file.ID] = file
	}
	s.mu.Unlock()

	return nil
}

// GetPeerFiles returns the files shared by a specific peer
func (s *Service) GetPeerFiles(ctx context.Context, peerID string) ([]*db.File, error) {
	var files []*db.File
	if err := s.db.GetDB().Where("owner_id = ?", peerID).Find(&files).Error; err != nil {
		return nil, fmt.Errorf("failed to get peer files: %w", err)
	}
	return files, nil
}

// DisconnectPeer handles peer disconnection
func (s *Service) DisconnectPeer(ctx context.Context, peerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check super peers first
	if peer, exists := s.superPeers[peerID]; exists {
		close(peer.Disconnect)
		delete(s.superPeers, peerID)
		return nil
	}

	// Check regular peers
	if peer, exists := s.peers[peerID]; exists {
		close(peer.Disconnect)
		delete(s.peers, peerID)
		return nil
	}

	return fmt.Errorf("peer not found")
}

// monitorPeerConnection monitors peer connection health
func (s *Service) monitorPeerConnection(peer *PeerConnection) {
	ticker := time.NewTicker(time.Duration(s.cfg.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Check if peer has exceeded timeout
			if time.Since(peer.LastPing) > time.Duration(s.cfg.ConnectionTimeout)*time.Second {
				s.logger.Info("Peer connection timed out",
					zap.String("peer_id", peer.User.ID),
					zap.String("username", peer.User.Username))

				// Disconnect peer
				s.DisconnectPeer(context.Background(), peer.User.ID)
				return
			}
		case <-peer.Disconnect:
			return
		}
	}
}

// UpdatePeerStatus updates a peer's last seen timestamp
func (s *Service) UpdatePeerStatus(ctx context.Context, peerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update in-memory state
	if peer, exists := s.peers[peerID]; exists {
		peer.LastPing = time.Now()
	} else if peer, exists := s.superPeers[peerID]; exists {
		peer.LastPing = time.Now()
	} else {
		return fmt.Errorf("peer not found")
	}

	// Update database
	if err := s.db.GetDB().Model(&db.User{}).Where("id = ?", peerID).Update("last_seen", time.Now()).Error; err != nil {
		return fmt.Errorf("failed to update peer status: %w", err)
	}

	return nil
}
