package db

import (
	"fmt"
	"time"

	"github.com/inventor7/p2p/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Models
type User struct {
	ID        string    `gorm:"primaryKey;type:uuid" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	IsSuper   bool      `gorm:"default:false" json:"is_super"`
	LastSeen  time.Time `json:"last_seen"`
	IPAddress string    `json:"ip_address,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type File struct {
	ID           string    `gorm:"primaryKey;type:uuid" json:"id"`
	Name         string    `gorm:"not null" json:"name"`
	Type         string    `json:"type"`
	Size         int64     `json:"size"`
	OwnerID      string    `gorm:"type:uuid" json:"owner_id"`
	Path         string    `json:"path"`
	Hash         string    `gorm:"index" json:"hash"`
	PreviewURL   string    `json:"preview_url,omitempty"`
	LastModified time.Time `json:"last_modified"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SharedSpace struct {
	ID          string    `gorm:"primaryKey;type:uuid" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `gorm:"type:uuid" json:"created_by"`
	Color       string    `json:"color"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SpaceMember struct {
	SpaceID  string    `gorm:"primaryKey;type:uuid" json:"space_id"`
	UserID   string    `gorm:"primaryKey;type:uuid" json:"user_id"`
	JoinedAt time.Time `json:"joined_at"`
}

type SpaceFile struct {
	SpaceID string    `gorm:"primaryKey;type:uuid" json:"space_id"`
	FileID  string    `gorm:"primaryKey;type:uuid" json:"file_id"`
	AddedAt time.Time `json:"added_at"`
}

// Database represents the database connection and operations
type Database struct {
	db *gorm.DB
}

// NewDatabase creates a new database connection
func NewDatabase(cfg *config.Config) (*Database, error) {
	db, err := gorm.Open(postgres.Open(cfg.GetDSN()), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto migrate the schema
	if err := db.AutoMigrate(
		&User{},
		&File{},
		&SharedSpace{},
		&SpaceMember{},
		&SpaceFile{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return &Database{db: db}, nil
}

// GetDB returns the underlying database connection
func (d *Database) GetDB() *gorm.DB {
	return d.db
}
