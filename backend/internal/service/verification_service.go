package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/go-redis/redis/v8"
)

// VerificationService handles verification code operations using Redis
type VerificationService struct {
	redis *redis.Client
}

// NewVerificationService creates a new verification service
func NewVerificationService(redis *redis.Client) *VerificationService {
	return &VerificationService{redis: redis}
}

// Code type constants
const (
	CodeTypeLogin         = "login"
	CodeTypeRegister      = "register"
	CodeTypeResetPassword = "reset_password"
	CodeTypeVerifyEmail   = "verify_email"
	CodeTypeVerifyPhone   = "verify_phone"
	CodeTypeInvitation    = "invitation"
)

const (
	// Default code expiration time (5 minutes)
	defaultExpiration = 5 * time.Minute
	// Invitation code expiration time (24 hours)
	invitationExpiration = 24 * time.Hour
	// Default code length
	defaultCodeLength = 6
	// Maximum attempts allowed
	maxAttempts = 5
	// Resend cooldown (60 seconds)
	resendCooldown = 60 * time.Second
)

// GenerateCode generates and stores a verification code in Redis
// Returns the generated code or error
func (s *VerificationService) GenerateCode(
	ctx context.Context,
	identifier string, // email or phone
	codeType string, // login, register, reset_password, etc.
) (string, error) {
	if s.redis == nil {
		return "", fmt.Errorf("redis client not initialized")
	}
	return s.GenerateCodeWithLength(ctx, identifier, codeType, defaultCodeLength)
}

// GenerateCodeWithLength generates a code with custom length
func (s *VerificationService) GenerateCodeWithLength(
	ctx context.Context,
	identifier string,
	codeType string,
	length int,
) (string, error) {
	if s.redis == nil {
		return "", fmt.Errorf("redis client not initialized")
	}
	
	// Ensure context is not nil
	if ctx == nil {
		ctx = context.Background()
	}
	
	if length <= 0 {
		length = defaultCodeLength
	}

	// Generate random numeric code
	code := generateNumericCode(length)

	// Store code in Redis with TTL
	key := getVerificationKey(codeType, identifier)
	if err := s.redis.Set(ctx, key, code, defaultExpiration).Err(); err != nil {
		return "", fmt.Errorf("failed to store verification code: %w", err)
	}

	// Initialize attempts counter
	attemptsKey := getAttemptsKey(codeType, identifier)
	if err := s.redis.Set(ctx, attemptsKey, 0, defaultExpiration).Err(); err != nil {
		// Non-critical error, log but don't fail
		fmt.Printf("Warning: failed to initialize attempts counter: %v\n", err)
	}

	return code, nil
}

// GenerateInvitationCode generates an invitation code with 24-hour expiration
func (s *VerificationService) GenerateInvitationCode(
	ctx context.Context,
	email string,
) (string, error) {
	if s.redis == nil {
		return "", fmt.Errorf("redis client not initialized")
	}
	
	// Ensure context is not nil
	if ctx == nil {
		ctx = context.Background()
	}
	
	code := generateNumericCode(defaultCodeLength)

	key := getVerificationKey(CodeTypeInvitation, email)
	if err := s.redis.Set(ctx, key, code, invitationExpiration).Err(); err != nil {
		return "", fmt.Errorf("failed to store invitation code: %w", err)
	}

	attemptsKey := getAttemptsKey(CodeTypeInvitation, email)
	if err := s.redis.Set(ctx, attemptsKey, 0, invitationExpiration).Err(); err != nil {
		fmt.Printf("Warning: failed to initialize attempts counter: %v\n", err)
	}

	return code, nil
}

// VerifyCode verifies a verification code
// Returns error if code is invalid, expired, or too many attempts
func (s *VerificationService) VerifyCode(
	ctx context.Context,
	identifier string,
	codeType string,
	code string,
) error {
	if s.redis == nil {
		return fmt.Errorf("redis client not initialized")
	}
	
	// Check attempts
	attempts, err := s.GetAttempts(ctx, identifier, codeType)
	if err != nil {
		return err
	}

	if attempts >= maxAttempts {
		return fmt.Errorf("too many attempts, please request a new code")
	}

	// Get stored code
	key := getVerificationKey(codeType, identifier)
	storedCode, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return fmt.Errorf("verification code expired or not found")
	}
	if err != nil {
		return fmt.Errorf("failed to get verification code: %w", err)
	}

	// Verify code
	if storedCode != code {
		// Increment attempts on failure
		if err := s.IncrementAttempts(ctx, identifier, codeType); err != nil {
			fmt.Printf("Warning: failed to increment attempts: %v\n", err)
		}
		return fmt.Errorf("invalid verification code")
	}

	// Code is valid - delete it and attempts counter
	pipe := s.redis.Pipeline()
	pipe.Del(ctx, key)
	pipe.Del(ctx, getAttemptsKey(codeType, identifier))
	if _, err := pipe.Exec(ctx); err != nil {
		fmt.Printf("Warning: failed to cleanup after verification: %v\n", err)
	}

	return nil
}

// IncrementAttempts increments the failed attempts counter
func (s *VerificationService) IncrementAttempts(
	ctx context.Context,
	identifier string,
	codeType string,
) error {
	if s.redis == nil {
		return fmt.Errorf("redis client not initialized")
	}
	
	// Ensure context is not nil
	if ctx == nil {
		ctx = context.Background()
	}
	
	attemptsKey := getAttemptsKey(codeType, identifier)
	return s.redis.Incr(ctx, attemptsKey).Err()
}

// GetAttempts gets the number of failed attempts
func (s *VerificationService) GetAttempts(
	ctx context.Context,
	identifier string,
	codeType string,
) (int, error) {
	if s.redis == nil {
		return 0, fmt.Errorf("redis client not initialized")
	}
	
	// Ensure context is not nil
	if ctx == nil {
		ctx = context.Background()
	}
	
	attemptsKey := getAttemptsKey(codeType, identifier)
	val, err := s.redis.Get(ctx, attemptsKey).Int()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// ResendCode generates a new code with cooldown protection
// Returns the new code or error if cooldown is active
func (s *VerificationService) ResendCode(
	ctx context.Context,
	identifier string,
	codeType string,
) (string, error) {
	if s.redis == nil {
		return "", fmt.Errorf("redis client not initialized")
	}
	
	// Ensure context is not nil
	if ctx == nil {
		ctx = context.Background()
	}
	
	// Check cooldown
	cooldownKey := getCooldownKey(codeType, identifier)
	exists, err := s.redis.Exists(ctx, cooldownKey).Result()
	if err != nil {
		return "", err
	}

	if exists > 0 {
		ttl, _ := s.redis.TTL(ctx, cooldownKey).Result()
		return "", fmt.Errorf("please wait %d seconds before requesting a new code", int(ttl.Seconds()))
	}

	// Generate new code
	code, err := s.GenerateCode(ctx, identifier, codeType)
	if err != nil {
		return "", err
	}

	// Set cooldown
	if err := s.redis.Set(ctx, cooldownKey, "1", resendCooldown).Err(); err != nil {
		// Non-critical error, log but don't fail
		fmt.Printf("Warning: failed to set cooldown: %v\n", err)
	}

	return code, nil
}

// DeleteCode deletes a verification code (for cleanup or cancel operations)
func (s *VerificationService) DeleteCode(
	ctx context.Context,
	identifier string,
	codeType string,
) error {
	if s.redis == nil {
		return fmt.Errorf("redis client not initialized")
	}
	
	// Ensure context is not nil
	if ctx == nil {
		ctx = context.Background()
	}
	
	key := getVerificationKey(codeType, identifier)
	return s.redis.Del(ctx, key).Err()
}

// GetCodeTTL gets the remaining TTL of a verification code
func (s *VerificationService) GetCodeTTL(
	ctx context.Context,
	identifier string,
	codeType string,
) (time.Duration, error) {
	if s.redis == nil {
		return 0, fmt.Errorf("redis client not initialized")
	}
	
	// Ensure context is not nil
	if ctx == nil {
		ctx = context.Background()
	}
	
	key := getVerificationKey(codeType, identifier)
	return s.redis.TTL(ctx, key).Result()
}

// Helper functions

// getVerificationKey generates Redis key for verification code
// Format: verification:{type}:{identifier}
func getVerificationKey(codeType, identifier string) string {
	return fmt.Sprintf("verification:%s:%s", codeType, identifier)
}

// getAttemptsKey generates Redis key for attempts counter
// Format: verification:attempts:{type}:{identifier}
func getAttemptsKey(codeType, identifier string) string {
	return fmt.Sprintf("verification:attempts:%s:%s", codeType, identifier)
}

// getCooldownKey generates Redis key for resend cooldown
// Format: verification:cooldown:{type}:{identifier}
func getCooldownKey(codeType, identifier string) string {
	return fmt.Sprintf("verification:cooldown:%s:%s", codeType, identifier)
}

// generateNumericCode generates a random numeric code of specified length
func generateNumericCode(length int) string {
	const digits = "0123456789"
	result := make([]byte, length)

	for i := range result {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		result[i] = digits[num.Int64()]
	}

	return string(result)
}
