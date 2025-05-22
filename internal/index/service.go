package index

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/inventor7/p2p/internal/config"
	"github.com/inventor7/p2p/internal/db"
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

	// Save shared space
	if err := tx.Create(space).Error; err != nil {
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
		tx.Rollback()
		return nil, fmt.Errorf("failed to add creator as member: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return space, nil
}

// AddMemberToSpace adds a user to a shared space
func (s *Service) AddMemberToSpace(ctx context.Context, spaceID, userID string) error {
	// Check if user is already a member
	var exists bool
	err := s.db.GetDB().Model(&db.SpaceMember{}).Select("count(*) > 0").Where(
		"space_id = ? AND user_id = ?", spaceID, userID,
	).Find(&exists).Error

	if err != nil {
		return fmt.Errorf("failed to check membership: %w", err)
	}

	if exists {
		return fmt.Errorf("user is already a member of this space")
	}

	// Add member
	member := &db.SpaceMember{
		SpaceID:  spaceID,
		UserID:   userID,
		JoinedAt: time.Now(),
	}

	if err := s.db.GetDB().Create(member).Error; err != nil {
		return fmt.Errorf("failed to add member to space: %w", err)
	}

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
		return fmt.Errorf("failed to add file to space: %w", err)
	}

	return nil
}

// GetSpaceFiles returns all files in a shared space
func (s *Service) GetSpaceFiles(ctx context.Context, spaceID string) ([]*db.File, error) {
	var files []*db.File
	err := s.db.GetDB().Model(&db.File{}).Joins(
		"JOIN space_files ON space_files.file_id = files.id",
	).Where(
		"space_files.space_id = ?", spaceID,
	).Find(&files).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get space files: %w", err)
	}

	return files, nil
}

// GetSpaceMembers returns all members of a shared space
func (s *Service) GetSpaceMembers(ctx context.Context, spaceID string) ([]*db.User, error) {
	var users []*db.User
	err := s.db.GetDB().Model(&db.User{}).Joins(
		"JOIN space_members ON space_members.user_id = users.id",
	).Where(
		"space_members.space_id = ?", spaceID,
	).Find(&users).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get space members: %w", err)
	}

	return users, nil
}

// RemoveFromSpace removes a member or file from a shared space
func (s *Service) RemoveFromSpace(ctx context.Context, spaceID string, itemID string, itemType string) error {
	tx := s.db.GetDB().Begin()

	switch itemType {
	case "member":
		// Check if member is the space creator
		var space db.SharedSpace
		if err := tx.First(&space, "id = ?", spaceID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to get space: %w", err)
		}

		if space.CreatedBy == itemID {
			tx.Rollback()
			return fmt.Errorf("cannot remove space creator")
		}

		// Remove member
		if err := tx.Delete(&db.SpaceMember{}, "space_id = ? AND user_id = ?", spaceID, itemID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to remove member: %w", err)
		}

	case "file":
		// Remove file
		if err := tx.Delete(&db.SpaceFile{}, "space_id = ? AND file_id = ?", spaceID, itemID).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to remove file: %w", err)
		}

	default:
		tx.Rollback()
		return fmt.Errorf("invalid item type: %s", itemType)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// SearchFiles searches for files across all spaces a user has access to
func (s *Service) SearchFiles(ctx context.Context, userID string, query string) ([]*db.File, error) {
	var files []*db.File

	// Get all spaces the user is a member of
	var spaceIDs []string
	err := s.db.GetDB().Model(&db.SpaceMember{}).Where(
		"user_id = ?", userID,
	).Pluck("space_id", &spaceIDs).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get user spaces: %w", err)
	}

	// Search for files in these spaces
	err = s.db.GetDB().Model(&db.File{}).Joins(
		"JOIN space_files ON space_files.file_id = files.id",
	).Where(
		"space_files.space_id IN ? AND (files.name ILIKE ? OR files.type ILIKE ?)",
		spaceIDs, "%"+query+"%", "%"+query+"%",
	).Find(&files).Error

	if err != nil {
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	return files, nil
}
