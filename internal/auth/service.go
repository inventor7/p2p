package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
)

// Service handles authentication business logic
type Service struct {
	jwtSecret []byte
	users     map[string]User // In-memory user store for demo
	tokens    map[string]bool // Active tokens
}

// User represents a registered user
type User struct {
	ID           string
	Username     string
	PasswordHash string
}

// NewService creates a new auth service instance
func NewService() *Service {
	return &Service{
		jwtSecret: []byte("your-secret-key"), // In production, use environment variable
		users:     make(map[string]User),
		tokens:    make(map[string]bool),
	}
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, username, password string) (string, error) {
	// Check if username already exists
	for _, user := range s.users {
		if user.Username == username {
			return "", errors.New("username already exists")
		}
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	// Create user
	user := User{
		ID:           username, // For simplicity, using username as ID
		Username:     username,
		PasswordHash: string(hash),
	}

	s.users[user.ID] = user

	// Generate token
	return s.generateToken(user.ID)
}

// Login authenticates a user
func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	// Find user
	var user User
	for _, u := range s.users {
		if u.Username == username {
			user = u
			break
		}
	}

	if user.ID == "" {
		return "", errors.New("user not found")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid password")
	}

	// Generate token
	return s.generateToken(user.ID)
}

// Logout invalidates a user's token
func (s *Service) Logout(ctx context.Context, userID string) error {
	// In a real implementation, you would invalidate the token
	// For this demo, we'll just return nil
	return nil
}

// ValidateToken validates a JWT token and returns the user ID
func (s *Service) ValidateToken(ctx context.Context, tokenString string) (string, error) {
	// Parse token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return "", err
	}

	// Validate claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID := claims["user_id"].(string)
		if _, exists := s.users[userID]; !exists {
			return "", errors.New("user not found")
		}
		return userID, nil
	}

	return "", errors.New("invalid token")
}

// generateToken creates a new JWT token
func (s *Service) generateToken(userID string) (string, error) {
	// Create claims
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token
	return token.SignedString(s.jwtSecret)
}
