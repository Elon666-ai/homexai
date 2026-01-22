package service

import (
	"errors"
	"fmt"
	"time"

	"homexai/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the JWT claims
type JWTClaims struct {
	UserID     uint   `json:"user_id"`
	Email      string `json:"email"`
	PropertyID uint   `json:"property_id,omitempty"`
	Role       string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken generates a JWT token for a user
// If rememberMe is true, token will have extended expiration (30 days)
func GenerateToken(userID uint, email string, role string, propertyID uint) (string, error) {
	return GenerateTokenWithRemember(userID, email, role, propertyID, false)
}

// GenerateTokenWithRemember generates a JWT token with optional extended expiration
func GenerateTokenWithRemember(userID uint, email string, role string, propertyID uint, rememberMe bool) (string, error) {
	var expirationTime time.Time
	if rememberMe {
		// 30 days for "Remember Me"
		expirationTime = time.Now().Add(7 * 24 * time.Hour)
	} else {
		// Default expiration from config
		expirationTime = time.Now().Add(time.Duration(config.Yaml.JWT.ExpireHours) * time.Hour)
	}

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
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(config.Yaml.JWT.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// GenerateRefreshToken generates a refresh token
func GenerateRefreshToken(userID uint) (string, error) {
	jwtConf := config.Yaml

	expirationTime := time.Now().Add(time.Duration(jwtConf.JWT.RefreshExpireHours) * time.Hour)

	claims := &JWTClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "homex-api",
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtConf.JWT.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func ValidateToken(tokenString string) (*JWTClaims, error) {
	jwtConf := config.Yaml

	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtConf.JWT.Secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// RefreshAccessToken generates a new access token from a refresh token
func RefreshAccessToken(refreshToken string) (string, error) {
	claims, err := ValidateToken(refreshToken)
	if err != nil {
		return "", err
	}

	// Generate new access token
	return GenerateToken(claims.UserID, claims.Email, claims.Role, claims.PropertyID)
}

// ExtractUserIDFromToken extracts user ID from token
func ExtractUserIDFromToken(tokenString string) (uint, error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}
