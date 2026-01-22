package utils

import (
	"regexp"
)

// IsEmail checks if a string is a valid email address
func IsEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// IsValidEmail is an alias for IsEmail for compatibility
func IsValidEmail(email string) bool {
	return IsEmail(email)
}

// IsPhone checks if a string is a valid phone number (basic check)
func IsPhone(phone string) bool {
	phoneRegex := regexp.MustCompile(`^\+?[1-9]\d{1,14}$`)
	return phoneRegex.MatchString(phone)
}

// IsValidPhone is an alias for IsPhone for compatibility
func IsValidPhone(phone string) bool {
	return IsPhone(phone)
}
