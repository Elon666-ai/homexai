package utils

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// PasswordHash represents hashed password with salt
type PasswordHash struct {
	Hash string
	Salt string
}

// HashPasswordWithSalt hashes a password using MD5 with salt
// Formula: hash = md5(md5(password) + salt)
func HashPasswordWithSalt(password, salt string) (*PasswordHash, error) {
	if salt == "" {
		// Generate a 4-digit random number as salt (0000-9999)
		saltNum := make([]byte, 4)
		if _, err := rand.Read(saltNum); err != nil {
			return nil, err
		}
		salt = fmt.Sprintf("%04d", (int(saltNum[0])<<24|int(saltNum[1])<<16|int(saltNum[2])<<8|int(saltNum[3]))%10000)
	}

	// Calculate hash = md5(md5(password) + salt)
	md5Password := fmt.Sprintf("%x", md5.Sum([]byte(password)))
	hashInput := md5Password + salt
	hash := fmt.Sprintf("%x", md5.Sum([]byte(hashInput)))

	return &PasswordHash{
		Hash: hash,
		Salt: salt,
	}, nil
}

// func Md5Password(password string) string {

// 	md5Password := fmt.Sprintf("%x", md5.Sum([]byte(password)))
// 	return md5Password
// }

// HashPassword hashes a password using bcrypt (legacy function)
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// createPasswordHash creates password hash and salt for seeding
func CreatePasswordHash(password string) (string, string) {

	result, err := HashPasswordWithSalt(password, "")
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	return result.Hash, result.Salt
}

// CheckPasswordHash compares a password with its hash
// Supports both legacy bcrypt and new MD5 with salt
func CheckPasswordHash(password, hash, salt string) bool {
	fmt.Printf("CheckPasswordHash: password: %s, hash: %s, salt: %s\n", password, hash, salt)
	// If salt is provided, use new MD5 strategy
	if salt != "" {
		// Calculate hash = md5(md5(password) + salt)
		// md5Password := fmt.Sprintf("%x", md5.Sum([]byte(password)))
		hashInput := password + salt
		computedHash := fmt.Sprintf("%x", md5.Sum([]byte(hashInput)))
		return computedHash == hash
	}

	// Legacy support: check if password is already a bcrypt hash
	if strings.HasPrefix(password, "$2a$") || strings.HasPrefix(password, "$2b$") {
		return password == hash
	}

	// Legacy bcrypt comparison
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateTempPassword generates a random temporary password of specified length
func GenerateTempPassword(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a simple random string if crypto/rand fails
		return "TempPass123!"
	}
	return hex.EncodeToString(bytes)[:length]
}
