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
	IPAddress  string // IP address of the peer
	ListenPort int    // Port the peer is listening on for P2P connections
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
func (s *Service) RegisterPeer(ctx context.Context, peerName string, ipAddress string, listenPort int, isSuper bool) (*db.User, error) {
	// Create new user record
	user := &db.User{
		ID:       uuid.New().String(),
		Username: peerName, // Use peerName for Username
		IsSuper:  isSuper,
		LastSeen: time.Now(),
		// IPAddress and ListenPort are not part of db.User by default.
		// If they need to be persisted in db.User, that model needs an update.
		// For now, they are stored in PeerConnection.
	}

	// Save to database
	if err := s.db.GetDB().Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to register peer: %w", err)
	}

	// Initialize peer connection
	conn := &PeerConnection{
		User:       user,
		IPAddress:  ipAddress,  // Store IP
		ListenPort: listenPort, // Store Port
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

	return user, nil // Return the created user object (which includes the ID)
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

// FileSearchResult combines file details with the peer's contact information.
type FileSearchResult struct {
	db.File
	PeerIPAddress  string `json:"peer_ip_address"`
	PeerListenPort int    `json:"peer_listen_port"`
}

// SearchSharedFiles searches for globally shared files and returns them with peer contact info.
func (s *Service) SearchSharedFiles(ctx context.Context, query string) ([]*FileSearchResult, error) {
	var dbFiles []*db.File

	// For MySQL, LIKE is often case-insensitive by default depending on collation.
	// If specific case-insensitivity is needed and collation doesn't ensure it:
	// err := s.db.GetDB().Where("LOWER(name) LIKE LOWER(?)", "%"+query+"%").Find(&dbFiles).Error
	// For simplicity, assuming default MySQL behavior or case-insensitive collation:
	searchTerm := "%" + query + "%"
	if err := s.db.GetDB().Where("name LIKE ?", searchTerm).Find(&dbFiles).Error; err != nil { // <--- CHANGED ILIKE to LIKE
		s.logger.Error("Failed to search shared files in DB", zap.Error(err), zap.String("query", query))
		return nil, fmt.Errorf("failed to search shared files: %w", err)
	}

	var results []*FileSearchResult
	s.mu.RLock() // Read lock for accessing peers maps
	defer s.mu.RUnlock()

	for _, file := range dbFiles {
		var conn *PeerConnection
		var found bool

		if p, ok := s.peers[file.OwnerID]; ok && p.IsActive {
			conn = p
			found = true
		} else if sp, ok := s.superPeers[file.OwnerID]; ok && sp.IsActive {
			conn = sp
			found = true
		}

		if found {
			results = append(results, &FileSearchResult{
				File:           *file,
				PeerIPAddress:  conn.IPAddress,
				PeerListenPort: conn.ListenPort,
			})
		} else {
			s.logger.Debug("File found in DB but owner peer is not active or not found in memory", zap.String("fileID", file.ID), zap.String("ownerID", file.OwnerID))
		}
	}

	s.logger.Info("Searched shared files", zap.String("query", query), zap.Int("db_matches", len(dbFiles)), zap.Int("active_results", len(results)))
	return results, nil
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

// FileSearchResult combines file details with the peer's contact information.
// type FileSearchResult struct {
// 	db.File
// 	PeerIPAddress  string `json:"peer_ip_address"`
// 	PeerListenPort int    `json:"peer_listen_port"`
// }

type GetActivePeersDTO struct {
	ID            string    `json:"id"`
	Username      string    `json:"name"`          // Match frontend 'name'
	IsSuperClient bool      `json:"isSuperClient"` // Match frontend
	IPAddress     string    `json:"ipAddress"`
	ListenPort    int       `json:"listenPort"`
	LastSeen      time.Time `json:"lastSeen"`
	// Add other fields your frontend Peer type expects, like sharedFilesCount
}

// GetActivePeers retrieves a list of currently active peers.
func (s *Service) GetActivePeers(ctx context.Context) ([]GetActivePeersDTO, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activePeers := make([]GetActivePeersDTO, 0, len(s.peers)+len(s.superPeers))

	for _, conn := range s.peers {
		if conn.IsActive {
			activePeers = append(activePeers, GetActivePeersDTO{
				ID:            conn.User.ID,
				Username:      conn.User.Username,
				IsSuperClient: conn.User.IsSuper,
				IPAddress:     conn.IPAddress,
				ListenPort:    conn.ListenPort,
				LastSeen:      conn.LastPing, // or conn.User.LastSeen if that's more accurate
			})
		}
	}
	for _, conn := range s.superPeers {
		if conn.IsActive {
			activePeers = append(activePeers, GetActivePeersDTO{
				ID:            conn.User.ID,
				Username:      conn.User.Username,
				IsSuperClient: conn.User.IsSuper,
				IPAddress:     conn.IPAddress,
				ListenPort:    conn.ListenPort,
				LastSeen:      conn.LastPing,
			})
		}
	}
	s.logger.Info("Retrieved active peers", zap.Int("count", len(activePeers)))
	return activePeers, nil
}
