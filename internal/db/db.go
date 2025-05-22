package db

import (
	"fmt"
	"time"

	"github.com/inventor7/p2p/internal/config"
	"gorm.io/driver/mysql" // <--- CHANGE: Import MySQL driver
	"gorm.io/gorm"
)

// Models
// UUIDs will typically be stored as VARCHAR(36) or CHAR(36) in MySQL.
// GORM's MySQL driver handles string-based UUIDs.
type User struct {
	ID           string    `gorm:"primaryKey;type:varchar(36)" json:"id"` // Explicit for MySQL if `type:uuid` isn't ideal
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	IsSuper      bool      `gorm:"default:false" json:"is_super"`
	PasswordHash string    `gorm:"not null" json:"-"`
	LastSeen     time.Time `json:"last_seen"`
	IPAddress    string    `json:"ip_address,omitempty"` // Consider if this should be in User table
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type File struct {
	ID           string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name         string    `gorm:"not null" json:"name"`
	Type         string    `json:"type"`
	Size         int64     `json:"size"`
	OwnerID      string    `gorm:"type:varchar(36);index" json:"owner_id"` // Added index for faster lookups
	Path         string    `json:"path"`
	Hash         string    `gorm:"index" json:"hash"`
	PreviewURL   string    `json:"preview_url,omitempty"`
	LastModified time.Time `json:"last_modified"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type SharedSpace struct {
	ID          string    `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description"`
	CreatedBy   string    `gorm:"type:varchar(36)" json:"created_by"`
	Color       string    `json:"color"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SpaceMember struct {
	// Using composite primary key is fine.
	// Alternatively, an auto-incrementing ID for the join table itself.
	SpaceID  string    `gorm:"primaryKey;type:varchar(36)" json:"space_id"`
	UserID   string    `gorm:"primaryKey;type:varchar(36)" json:"user_id"`
	JoinedAt time.Time `json:"joined_at"`
}

type SpaceFile struct {
	SpaceID string    `gorm:"primaryKey;type:varchar(36)" json:"space_id"`
	FileID  string    `gorm:"primaryKey;type:varchar(36)" json:"file_id"`
	AddedAt time.Time `json:"added_at"`
}

// Database represents the database connection and operations
type Database struct {
	db *gorm.DB
}

// NewDatabase creates a new database connection using MySQL
func NewDatabase(cfg *config.Config) (*Database, error) {
	// DSN format for MySQL: "username:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	// Ensure cfg.GetDSN() returns a DSN in this format.
	dsn := cfg.GetDSN()
	if dsn == "" {
		return nil, fmt.Errorf("database DSN is not configured")
	}

	gormDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{ // <--- CHANGE: Use mysql.Open
		// Optionally add GORM logger configuration here
		// Logger: logger.Default.LogMode(logger.Silent), // Example for GORM logger
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL database: %w", err)
	}

	// Auto migrate the schema
	// GORM will create tables based on these structs.
	// For MySQL, it's good practice to define explicit string lengths for indexed fields
	// if not using TEXT types, but GORM often handles reasonable defaults.
	if err := gormDB.AutoMigrate(
		&User{},
		&File{},
		&SharedSpace{},
		&SpaceMember{},
		&SpaceFile{},
	); err != nil {
		return nil, fmt.Errorf("failed to migrate MySQL database: %w", err)
	}

	return &Database{db: gormDB}, nil
}

// GetDB returns the underlying database connection
func (d *Database) GetDB() *gorm.DB {
	if d == nil || d.db == nil {
		// This should ideally not happen if NewDatabase is always used and checked.
		// Consider panicking or returning an error if the DB is uninitialized.
		panic("Database accessed before initialization or GetDB called on nil *Database receiver")
	}
	return d.db
}
