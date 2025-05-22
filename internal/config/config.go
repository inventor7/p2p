package config

import (
	"fmt"
	"os"
	"strconv"

	"go.uber.org/zap"
)

type Config struct {
	// Server configuration
	ServerPort     int
	ServerHost     string
	Environment    string
	AllowedOrigins []string

	// Database configuration
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// P2P configuration
	MaxPeers            int
	MaxSuperPeers       int
	HeartbeatInterval   int // seconds
	ConnectionTimeout   int // seconds
	MaxFileSize         int64
	AllowedFileTypes    []string
	DefaultDownloadPath string

	// JWT configuration
	JWTExpiration int // hours

	JWTSecret            string `env:"JWT_SECRET"`
	JWTExpirationMinutes int    `env:"JWT_EXPIRATION_MINUTES" envDefault:"1440"`

	// Logger
	Logger *zap.Logger
}

// NewConfig creates a new configuration instance
func NewConfig(logger *zap.Logger) (*Config, error) {
	port, _ := strconv.Atoi(getEnvOrDefault("SERVER_PORT", "8080"))
	dbPort, _ := strconv.Atoi(getEnvOrDefault("DB_PORT", "3306"))
	maxPeers, _ := strconv.Atoi(getEnvOrDefault("MAX_PEERS", "100"))
	maxSuperPeers, _ := strconv.Atoi(getEnvOrDefault("MAX_SUPER_PEERS", "10"))
	heartbeat, _ := strconv.Atoi(getEnvOrDefault("HEARTBEAT_INTERVAL", "30"))
	timeout, _ := strconv.Atoi(getEnvOrDefault("CONNECTION_TIMEOUT", "60"))
	maxFileSize, _ := strconv.ParseInt(getEnvOrDefault("MAX_FILE_SIZE", "1073741824"), 10, 64) // 1GB default
	jwtExp, _ := strconv.Atoi(getEnvOrDefault("JWT_EXPIRATION", "24"))

	config := &Config{
		ServerPort:  port,
		ServerHost:  getEnvOrDefault("SERVER_HOST", "localhost"),
		Environment: getEnvOrDefault("ENVIRONMENT", "development"),
		AllowedOrigins: []string{
			"http://localhost:3000",
			"http://localhost:5173",
			"http://localhost:8081",
		},

		DBHost:     getEnvOrDefault("DB_HOST", "localhost"),
		DBPort:     dbPort,
		DBUser:     getEnvOrDefault("DB_USER", "root"),
		DBPassword: getEnvOrDefault("DB_PASSWORD", "aybinv7"),
		DBName:     getEnvOrDefault("DB_NAME", "p2p"),
		DBSSLMode:  getEnvOrDefault("DB_SSLMODE", "disable"),

		MaxPeers:            maxPeers,
		MaxSuperPeers:       maxSuperPeers,
		HeartbeatInterval:   heartbeat,
		ConnectionTimeout:   timeout,
		MaxFileSize:         maxFileSize,
		DefaultDownloadPath: getEnvOrDefault("DEFAULT_DOWNLOAD_PATH", "./downloads"),
		AllowedFileTypes: []string{
			"image/*",
			"video/*",
			"audio/*",
			"text/*",
			"application/pdf",
			"application/zip",
			"application/x-rar-compressed",
			"application/x-7z-compressed",
		},

		JWTSecret:     getEnvOrDefault("JWT_SECRET", "your-secret-key"),
		JWTExpiration: jwtExp,
		Logger:        logger,
	}

	return config, nil
}

// GetDSN returns the database connection string
// GetDSN constructs the Data Source Name for the database.
// THIS METHOD NOW NEEDS TO GENERATE A MYSQL DSN.
func (c *Config) GetDSN() string {
	// MySQL DSN: "username:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	if c.DBUser == "" || c.DBName == "" { // DBPassword can be empty for local dev
		// c.logger.Warn("DB_USER or DB_NAME not configured, DSN will be empty.") // If logger is available
		return ""
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.DBUser,
		c.DBPassword,
		c.DBHost,
		c.DBPort,
		c.DBName,
	)
}

// Helper function to get environment variable with default value
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
