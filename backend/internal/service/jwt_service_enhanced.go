package service

import (
	"context"
	"fmt"
	"time"

	"homexai/internal/config"
	"homexai/internal/database"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenPair represents access and refresh token pair
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // in seconds
	TokenType    string `json:"token_type"` // "Bearer"
}

// GenerateTokenPair generates both access token and refresh token
func GenerateTokenPair(userID uint, email string, role string, propertyID uint) (*TokenPair, error) {
	// Generate access token (short-lived: 1 hour)
	accessExpiration := time.Duration(config.Yaml.JWT.ExpireHours) * time.Hour
	accessToken, err := generateTokenWithType(userID, email, role, propertyID, "access", accessExpiration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token (long-lived: 7 days)
	refreshExpiration := time.Duration(config.Yaml.JWT.RefreshExpireHours) * time.Hour
	refreshToken, err := generateTokenWithType(userID, email, role, propertyID, "refresh", refreshExpiration)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Store refresh token in Redis
	ctx := context.Background()
	refreshKey := fmt.Sprintf("refresh_token:%d", userID)
	if err := database.GetRedisClient().Set(ctx, refreshKey, refreshToken, refreshExpiration).Err(); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessExpiration.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

// generateTokenWithType generates a JWT token with specific type
func generateTokenWithType(userID uint, email string, role string, propertyID uint, tokenType string, expiration time.Duration) (string, error) {
	// Generate unique JTI (JWT ID) for token tracking and revocation
	jti := uuid.New().String()

	expirationTime := time.Now().Add(expiration)

	claims := &JWTClaims{
		UserID:     userID,
		Email:      email,
		PropertyID: propertyID,
		Role:       role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "homex-api",
			Subject:   fmt.Sprintf("%d", userID),
			ID:        jti, // JWT ID for revocation
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.Yaml.JWT.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// RefreshAccessTokenEnhanced generates a new access token from a refresh token
// This is the enhanced version with token revocation check
func RefreshAccessTokenEnhanced(refreshToken string) (*TokenPair, error) {
	// Validate refresh token
	claims, err := ValidateToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// Check if refresh token is revoked
	if IsTokenRevoked(claims.ID) {
		return nil, fmt.Errorf("refresh token has been revoked")
	}

	// Verify it's stored in Redis (valid refresh token)
	ctx := context.Background()
	refreshKey := fmt.Sprintf("refresh_token:%d", claims.UserID)
	storedToken, err := database.GetRedisClient().Get(ctx, refreshKey).Result()
	if err != nil {
		return nil, fmt.Errorf("refresh token not found or expired")
	}

	if storedToken != refreshToken {
		return nil, fmt.Errorf("refresh token mismatch")
	}

	// Generate new token pair
	return GenerateTokenPair(claims.UserID, claims.Email, claims.Role, claims.PropertyID)
}

// RevokeToken revokes a token by adding it to blacklist
// The token will be blacklisted until its expiration time
func RevokeToken(tokenString string) error {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}

	// Calculate TTL: time until token expires
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		// Token already expired, no need to blacklist
		return nil
	}

	// Add token to blacklist
	ctx := context.Background()
	blacklistKey := fmt.Sprintf("token:blacklist:%s", claims.ID)
	if err := database.GetRedisClient().Set(ctx, blacklistKey, "revoked", ttl).Err(); err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	return nil
}

// IsTokenRevoked checks if a token is in the blacklist
func IsTokenRevoked(jti string) bool {
	if jti == "" {
		return false
	}

	ctx := context.Background()
	blacklistKey := fmt.Sprintf("token:blacklist:%s", jti)
	val, err := database.GetRedisClient().Get(ctx, blacklistKey).Result()
	return err == nil && val == "revoked"
}

// RevokeAllUserTokens revokes all tokens for a specific user
// Useful for logout from all devices or security breach
func RevokeAllUserTokens(userID uint) error {
	ctx := context.Background()

	// Remove refresh token
	refreshKey := fmt.Sprintf("refresh_token:%d", userID)
	if err := database.GetRedisClient().Del(ctx, refreshKey).Err(); err != nil {
		return fmt.Errorf("failed to revoke user tokens: %w", err)
	}

	// Set a flag that all tokens issued before now are invalid
	// This is checked during token validation
	revokeAllKey := fmt.Sprintf("user:revoke_all:%d", userID)
	if err := database.GetRedisClient().Set(ctx, revokeAllKey, time.Now().Unix(), 24*time.Hour).Err(); err != nil {
		return fmt.Errorf("failed to set revoke_all flag: %w", err)
	}

	return nil
}

// ValidateTokenEnhanced validates a JWT token with revocation check
func ValidateTokenEnhanced(tokenString string) (*JWTClaims, error) {
	// First validate token structure and signature
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Check if token is specifically blacklisted
	if IsTokenRevoked(claims.ID) {
		return nil, fmt.Errorf("token has been revoked")
	}

	// Check if all user tokens have been revoked
	ctx := context.Background()
	revokeAllKey := fmt.Sprintf("user:revoke_all:%d", claims.UserID)
	revokeTimestamp, err := database.GetRedisClient().Get(ctx, revokeAllKey).Int64()
	if err == nil {
		// Revoke_all flag exists, check if token was issued before revocation
		if claims.IssuedAt.Time.Unix() < revokeTimestamp {
			return nil, fmt.Errorf("token was revoked (logout from all devices)")
		}
	}

	return claims, nil
}
