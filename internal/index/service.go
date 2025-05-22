package index

import (
	"context"
	"fmt"
	standardLog "log" // Import standard log for use when custom logger might be nil
	"time"

	"github.com/google/uuid"
	"github.com/inventor7/p2p/internal/config" // Assuming this is your project's config package
	"github.com/inventor7/p2p/internal/db"     // Assuming this is your project's db package
	"go.uber.org/zap"
)

// Service handles shared space and file indexing functionality
type Service struct {
	cfg    *config.Config
	db     *db.Database
	logger *zap.Logger
}

// NewService creates a new index service instance
func NewService(cfg *config.Config, database *db.Database, logger *zap.Logger) *Service {
	// Robustness: Check for nil dependencies
	if cfg == nil {
		standardLog.Fatal("index.NewService: config cannot be nil")
	}
	if database == nil {
		// Use logger if available, otherwise standard log
		if logger != nil {
			logger.Fatal("index.NewService: database instance cannot be nil")
		} else {
			standardLog.Fatal("index.NewService: database instance cannot be nil (logger also nil)")
		}
	}
	if logger == nil {
		standardLog.Fatal("index.NewService: logger instance cannot be nil")
	}

	return &Service{
		cfg:    cfg,
		db:     database,
		logger: logger,
	}
}

// CreateSharedSpace creates a new shared space
func (s *Service) CreateSharedSpace(ctx context.Context, name, description string, creatorID string) (*db.SharedSpace, error) {
	// Create shared space
	space := &db.SharedSpace{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		CreatedBy:   creatorID,
		Color:       "blue", // Default color
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Start transaction
	tx := s.db.GetDB().Begin()
	if tx.Error != nil {
		s.logger.Error("Failed to begin transaction for CreateSharedSpace", zap.Error(tx.Error))
		return nil, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Save shared space
	if err := tx.Create(space).Error; err != nil {
		s.logger.Error("Failed to create shared space in DB", zap.Error(err), zap.String("spaceName", name))
		tx.Rollback()
		return nil, fmt.Errorf("failed to create shared space: %w", err)
	}

	// Add creator as member
	member := &db.SpaceMember{
		SpaceID:  space.ID,
		UserID:   creatorID,
		JoinedAt: time.Now(),
	}

	if err := tx.Create(member).Error; err != nil {
		s.logger.Error("Failed to add creator as member to space", zap.Error(err), zap.String("spaceID", space.ID), zap.String("creatorID", creatorID))
		tx.Rollback()
		return nil, fmt.Errorf("failed to add creator as member: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		s.logger.Error("Failed to commit transaction for CreateSharedSpace", zap.Error(err))
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Shared space created successfully", zap.String("spaceID", space.ID), zap.String("spaceName", space.Name))
	return space, nil
}

// AddMemberToSpace adds a user to a shared space
func (s *Service) AddMemberToSpace(ctx context.Context, spaceID, userID string) error {
	// Using a subquery or a direct count for potentially better performance and clarity
	var count int64
	err := s.db.GetDB().Model(&db.SpaceMember{}).Where(
		"space_id = ? AND user_id = ?", spaceID, userID,
	).Count(&count).Error

	if err != nil {
		s.logger.Error("Failed to check membership", zap.Error(err), zap.String("spaceID", spaceID), zap.String("userID", userID))
		return fmt.Errorf("failed to check membership: %w", err)
	}

	if count > 0 { // If count is greater than 0, member exists
		s.logger.Warn("User already a member of this space", zap.String("spaceID", spaceID), zap.String("userID", userID))
		return fmt.Errorf("user is already a member of this space") // Potentially return a specific error type
	}

	// Add member
	member := &db.SpaceMember{
		SpaceID:  spaceID,
		UserID:   userID,
		JoinedAt: time.Now(),
	}

	if err := s.db.GetDB().Create(member).Error; err != nil {
		s.logger.Error("Failed to add member to space", zap.Error(err), zap.String("spaceID", spaceID), zap.String("userID", userID))
		return fmt.Errorf("failed to add member to space: %w", err)
	}

	s.logger.Info("Member added to space successfully", zap.String("spaceID", spaceID), zap.String("userID", userID))
	return nil
}

// AddFileToSpace adds a file to a shared space
func (s *Service) AddFileToSpace(ctx context.Context, spaceID, fileID string) error {
	// Add file to space
	spaceFile := &db.SpaceFile{
		SpaceID: spaceID,
		FileID:  fileID,
		AddedAt: time.Now(),
	}

	if err := s.db.GetDB().Create(spaceFile).Error; err != nil {
		s.logger.Error("Failed to add file to space", zap.Error(err), zap.String("spaceID", spaceID), zap.String("fileID", fileID))
		return fmt.Errorf("failed to add file to space: %w", err)
	}

	s.logger.Info("File added to space successfully", zap.String("spaceID", spaceID), zap.String("fileID", fileID))
	return nil
}

// GetSpaceFiles returns all files in a shared space
func (s *Service) GetSpaceFiles(ctx context.Context, spaceID string) ([]*db.File, error) {
	var files []*db.File
	// Ensure 'files' is the correct table name if GORM doesn't infer it correctly from db.File struct
	err := s.db.GetDB().Model(&db.File{}).
		Joins("JOIN space_files ON space_files.file_id = files.id"). // 'files.id' assumes table name is 'files'
		Where("space_files.space_id = ?", spaceID).
		Find(&files).Error

	if err != nil {
		s.logger.Error("Failed to get space files", zap.Error(err), zap.String("spaceID", spaceID))
		return nil, fmt.Errorf("failed to get space files: %w", err)
	}

	s.logger.Debug("Fetched space files", zap.String("spaceID", spaceID), zap.Int("count", len(files)))
	return files, nil
}

// GetSpaceByID returns a shared space by its ID
func (s *Service) GetSpaceByID(ctx context.Context, spaceID string) (*db.SharedSpace, error) {
	var space db.SharedSpace
	err := s.db.GetDB().First(&space, "id =?", spaceID).Error

	if err != nil {
		s.logger.Error("Failed to get space by ID", zap.Error(err), zap.String("spaceID", spaceID))
		return nil, fmt.Errorf("failed to get space: %w", err)

	}
	s.logger.Debug("Fetched space by ID", zap.String("spaceID", spaceID), zap.String("spaceName", space.Name))
	return &space, nil
}

// GetSpaceMembers returns all members of a shared space
func (s *Service) GetSpaceMembers(ctx context.Context, spaceID string) ([]*db.User, error) {
	var users []*db.User
	// Ensure 'users' is the correct table name if GORM doesn't infer it correctly from db.User struct
	err := s.db.GetDB().Model(&db.User{}).
		Joins("JOIN space_members ON space_members.user_id = users.id"). // 'users.id' assumes table name is 'users'
		Where("space_members.space_id = ?", spaceID).
		Find(&users).Error

	if err != nil {
		s.logger.Error("Failed to get space members", zap.Error(err), zap.String("spaceID", spaceID))
		return nil, fmt.Errorf("failed to get space members: %w", err)
	}

	s.logger.Debug("Fetched space members", zap.String("spaceID", spaceID), zap.Int("count", len(users)))
	return users, nil
}

// RemoveFromSpace removes a member or file from a shared space
func (s *Service) RemoveFromSpace(ctx context.Context, spaceID string, itemID string, itemType string) error {
	tx := s.db.GetDB().Begin()
	if tx.Error != nil {
		s.logger.Error("Failed to begin transaction for RemoveFromSpace", zap.Error(tx.Error))
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	switch itemType {
	case "member":
		// Check if member is the space creator
		var space db.SharedSpace
		if err := tx.First(&space, "id = ?", spaceID).Error; err != nil {
			s.logger.Error("Failed to get space details for creator check", zap.Error(err), zap.String("spaceID", spaceID))
			tx.Rollback()
			return fmt.Errorf("failed to get space: %w", err)
		}

		if space.CreatedBy == itemID {
			s.logger.Warn("Attempt to remove space creator", zap.String("spaceID", spaceID), zap.String("creatorID", itemID))
			tx.Rollback()
			return fmt.Errorf("cannot remove space creator")
		}

		// Remove member
		result := tx.Delete(&db.SpaceMember{}, "space_id = ? AND user_id = ?", spaceID, itemID)
		if result.Error != nil {
			s.logger.Error("Failed to remove member from space", zap.Error(result.Error), zap.String("spaceID", spaceID), zap.String("memberID", itemID))
			tx.Rollback()
			return fmt.Errorf("failed to remove member: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			s.logger.Warn("No member found to remove or already removed", zap.String("spaceID", spaceID), zap.String("memberID", itemID))
			// Optionally, you might not want to rollback here if it's not a critical failure, or return a specific "not found" error.
			// For now, let's assume it's okay if no rows were affected.
		}

	case "file":
		// Remove file
		result := tx.Delete(&db.SpaceFile{}, "space_id = ? AND file_id = ?", spaceID, itemID)
		if result.Error != nil {
			s.logger.Error("Failed to remove file from space", zap.Error(result.Error), zap.String("spaceID", spaceID), zap.String("fileID", itemID))
			tx.Rollback()
			return fmt.Errorf("failed to remove file: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			s.logger.Warn("No file found to remove or already removed from space", zap.String("spaceID", spaceID), zap.String("fileID", itemID))
		}

	default:
		s.logger.Warn("Invalid item type for removal from space", zap.String("itemType", itemType))
		tx.Rollback()
		return fmt.Errorf("invalid item type: %s", itemType)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		s.logger.Error("Failed to commit transaction for RemoveFromSpace", zap.Error(err))
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Item removed from space successfully", zap.String("spaceID", spaceID), zap.String("itemID", itemID), zap.String("itemType", itemType))
	return nil
}

func (s *Service) SearchFiles(ctx context.Context, userID string, query string) ([]*db.File, error) {
	var files []*db.File

	// Get all spaces the user is a member of
	var spaceIDs []string
	err := s.db.GetDB().Model(&db.SpaceMember{}).Where(
		"user_id = ?", userID,
	).Pluck("space_id", &spaceIDs).Error

	if err != nil {
		s.logger.Error("Failed to get user's spaces for search", zap.Error(err), zap.String("userID", userID))
		return nil, fmt.Errorf("failed to get user spaces: %w", err)
	}

	if len(spaceIDs) == 0 {
		s.logger.Debug("User is not a member of any spaces, search will yield no results.", zap.String("userID", userID))
		return files, nil // Return empty slice, not an error
	}

	// Search for files in these spaces
	searchTerm := "%" + query + "%"
	// For MySQL, LIKE is often case-insensitive. If specific case-insensitivity is required:
	// .Where("space_files.space_id IN ? AND (LOWER(files.name) LIKE LOWER(?) OR LOWER(files.type) LIKE LOWER(?))",
	// spaceIDs, searchTerm, searchTerm).
	err = s.db.GetDB().Model(&db.File{}).
		Joins("JOIN space_files ON space_files.file_id = files.id").
		Where("space_files.space_id IN ?", spaceIDs).
		Where("files.name LIKE ? OR files.type LIKE ?", searchTerm, searchTerm). // <--- CHANGED ILIKE to LIKE
		Find(&files).Error

	if err != nil {
		s.logger.Error("Failed to search files in user's spaces", zap.Error(err), zap.String("userID", userID), zap.String("query", query))
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	s.logger.Info("Searched files for user", zap.String("userID", userID), zap.String("query", query), zap.Int("count", len(files)))
	return files, nil
}

func (s *Service) ListSharedSpaces(ctx context.Context) ([]db.SharedSpace, error) {
	var spaces []db.SharedSpace
	if err := s.db.GetDB().Order("created_at desc").Find(&spaces).Error; err != nil { // Added Order for consistency
		s.logger.Error("Failed to list shared spaces from DB", zap.Error(err))
		return nil, fmt.Errorf("failed to list shared spaces: %w", err)
	}
	s.logger.Info("Retrieved shared spaces", zap.Int("count", len(spaces)))
	return spaces, nil
}
