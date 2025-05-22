package auth

import (
	"context"
	"errors"
	"fmt"
	standardLog "log"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid" // For generating user IDs
	"github.com/inventor7/p2p/internal/config"
	"github.com/inventor7/p2p/internal/db" // Your database package
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm" // Import GORM
)

// Service handles authentication business logic using the database
type Service struct {
	cfg    *config.Config
	db     *db.Database
	logger *zap.Logger
	// jwtSecret is derived from cfg.JWTSecret
}

// NewService creates a new auth service instance using the database
func NewService(cfg *config.Config, database *db.Database, logger *zap.Logger) *Service {
	if cfg == nil {
		standardLog.Fatal("auth.NewService: config cannot be nil")
	}
	if database == nil {
		if logger != nil {
			logger.Fatal("auth.NewService: database instance cannot be nil")
		} else {
			standardLog.Fatal("auth.NewService: database instance cannot be nil (logger also nil)")
		}
	}
	if logger == nil {
		standardLog.Fatal("auth.NewService: logger instance cannot be nil")
	}

	if cfg.JWTSecret == "" {
		logger.Warn("JWT_SECRET is not set in config, using default insecure key. THIS IS NOT FOR PRODUCTION.")
		// cfg.JWTSecret = "your-default-insecure-secret-key-for-dev-only" // Don't modify cfg, handle in usage
	}
	if cfg.JWTExpirationMinutes == 0 {
		logger.Warn("JWT_EXPIRATION_MINUTES not set or is 0, defaulting to 1440 (24 hours).")
		// cfg.JWTExpirationMinutes = 1440 // Don't modify cfg
	}

	return &Service{
		cfg:    cfg,
		db:     database,
		logger: logger,
	}
}

// Register creates a new user account in the database
func (s *Service) Register(ctx context.Context, username, password string /*, isSuper bool */) (string, error) {
	// Check if username already exists
	var existingUser db.User
	err := s.db.GetDB().Where("username = ?", username).First(&existingUser).Error
	if err == nil { // User found
		s.logger.Warn("Registration attempt for existing username", zap.String("username", username))
		return "", errors.New("username already exists")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) { // Other DB error
		s.logger.Error("DB error checking for existing username", zap.Error(err))
		return "", fmt.Errorf("database error: %w", err)
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		s.logger.Error("Failed to hash password during registration", zap.Error(err))
		return "", fmt.Errorf("could not process password: %w", err)
	}

	// Create user record for DB
	newUser := &db.User{
		ID:           uuid.New().String(), // Generate UUID for new user
		Username:     username,
		PasswordHash: string(hash), // You need to add PasswordHash to your db.User model
		IsSuper:      false,        // Default, or pass as param: isSuper
		LastSeen:     time.Now(),   // Set LastSeen on registration
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Save to database
	if err := s.db.GetDB().Create(newUser).Error; err != nil {
		s.logger.Error("Failed to register user in DB", zap.Error(err))
		return "", fmt.Errorf("failed to register user: %w", err)
	}
	s.logger.Info("User registered successfully in DB", zap.String("username", newUser.Username), zap.String("userID", newUser.ID))

	return s.generateToken(newUser.ID, newUser.IsSuper) // Pass IsSuper to token if needed
}

// Login authenticates a user against the database
func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	var user db.User
	err := s.db.GetDB().Where("username = ?", username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("Login attempt for non-existent user", zap.String("username", username))
			return "", errors.New("user not found or invalid credentials") // Generic message
		}
		s.logger.Error("DB error during login finding user", zap.Error(err))
		return "", fmt.Errorf("database error: %w", err)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.logger.Warn("Invalid password attempt for user", zap.String("username", username))
		return "", errors.New("user not found or invalid credentials") // Generic message
	}

	// Optionally update LastSeen
	go func() {
		updateErr := s.db.GetDB().Model(&db.User{}).Where("id = ?", user.ID).Update("last_seen", time.Now()).Error
		if updateErr != nil {
			s.logger.Error("Failed to update last_seen for user on login", zap.Error(updateErr), zap.String("userID", user.ID))
		}
	}()

	s.logger.Info("User logged in successfully from DB", zap.String("username", user.Username), zap.String("userID", user.ID))
	return s.generateToken(user.ID, user.IsSuper)
}

// Logout - JWTs are stateless. True revocation needs a blacklist.
func (s *Service) Logout(ctx context.Context, userID string) error {
	// For stateless JWTs, logout is primarily a client-side action (deleting the token).
	// If a token blacklist is implemented (e.g., in Redis or DB), add token to blacklist here.
	s.logger.Info("User logout processed", zap.String("userID", userID))
	return nil
}

// ValidateToken validates a JWT token and returns the user ID
func (s *Service) ValidateToken(ctx context.Context, tokenString string) (string, error) {
	jwtSecretKey := s.getJWTSecret()

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecretKey), nil
	})

	if err != nil {
		s.logger.Warn("Token parsing/validation error", zap.Error(err))
		// Check for specific errors like expired token
		if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				return "", errors.New("malformed token")
			} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
				return "", errors.New("token is expired or not yet valid")
			}
		}
		return "", fmt.Errorf("invalid token: %w", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID, okUserID := claims["user_id"].(string)
		// isSuper, _ := claims["is_super"].(bool) // Example if you add is_super to claims

		if !okUserID || userID == "" {
			s.logger.Warn("Token claims missing or invalid user_id")
			return "", errors.New("invalid token claims: user_id missing or not a string")
		}

		// Optionally, re-verify user exists in DB for extra security,
		// though if user is deleted after token issuance, token might still be valid until expiry.
		// var dbUser db.User
		// if errDb := s.db.GetDB().Select("id").First(&dbUser, "id = ?", userID).Error; errDb != nil {
		// 	s.logger.Warn("User ID from token not found in DB", zap.String("userID", userID), zap.Error(errDb))
		// 	return "", errors.New("user from token no longer exists")
		// }

		return userID, nil
	}

	s.logger.Warn("Token claims invalid or token is not valid")
	return "", errors.New("invalid token or claims")
}

// generateToken creates a new JWT token
func (s *Service) generateToken(userID string, isSuper bool) (string, error) {
	jwtSecretKey := s.getJWTSecret()
	expirationMinutes := s.getJWTExpiration()

	expirationTime := time.Now().Add(time.Duration(expirationMinutes) * time.Minute)
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     expirationTime.Unix(),
		// "is_super": isSuper, // Optionally add more claims
		// "iss": s.cfg.JWTIssuer, // Optional: Issuer from config
		// "aud": s.cfg.JWTAudience, // Optional: Audience from config
	}

	token := jwt.NewWithClaims(&jwt.SigningMethodHMAC{}, claims)
	signedToken, err := token.SignedString([]byte(jwtSecretKey))
	if err != nil {
		s.logger.Error("Failed to sign token", zap.Error(err))
		return "", fmt.Errorf("could not sign token: %w", err)
	}
	return signedToken, nil
}

func (s *Service) getJWTSecret() string {
	if s.cfg.JWTSecret == "" {
		return "your-default-insecure-secret-key-for-dev-only" // Fallback for safety, but should be logged in NewService
	}
	return s.cfg.JWTSecret
}

func (s *Service) getJWTExpiration() int {
	if s.cfg.JWTExpirationMinutes == 0 {
		return 24 * 60 // Default to 24 hours
	}
	return s.cfg.JWTExpirationMinutes
}
