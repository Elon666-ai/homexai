package middleware

import (
	"strings"

	"homexai/internal/service"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates JWT token and sets user context
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.UnauthorizedResponse(c, "Authorization header required")
			c.Abort()
			return
		}

		// Check if it's a Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			utils.UnauthorizedResponse(c, "Invalid authorization header format")
			c.Abort()
			return
		}

		token := parts[1]

		// Validate token
		claims, err := service.ValidateTokenEnhanced(token)
		if err != nil {
			utils.UnauthorizedResponse(c, "Invalid or expired token")
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		if claims.PropertyID != 0 {
			c.Set("property_id", claims.PropertyID)
		}

		c.Next()
	}
}

// OptionalAuthMiddleware validates JWT token if present but doesn't require it
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.Next()
			return
		}

		token := parts[1]
		claims, err := service.ValidateTokenEnhanced(token)
		if err == nil {
			c.Set("user_id", claims.UserID)
			c.Set("email", claims.Email)
			c.Set("role", claims.Role)
			if claims.PropertyID != 0 {
				c.Set("property_id", claims.PropertyID)
			}
		}

		c.Next()
	}
}

// RequireRole checks if user has a specific role
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			utils.ForbiddenResponse(c, "Role information not found")
			c.Abort()
			return
		}

		role := userRole.(string)
		hasRole := false
		for _, r := range roles {
			if role == r {
				hasRole = true
				break
			}
		}

		if !hasRole {
			utils.ForbiddenResponse(c, "Insufficient permissions")
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireSuperAdmin checks if user is a super admin
func RequireSuperAdmin() gin.HandlerFunc {
	return RequireRole("super_admin")
}

// RequirePropertyAdmin checks if user is a property admin or super admin
func RequirePropertyAdmin() gin.HandlerFunc {
	return RequireRole("super_admin", "property_admin")
}

// GetUserID extracts user ID from context
func GetUserID(c *gin.Context) uint {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0
	}
	return userID.(uint)
}

// GetUserRole extracts user role from context
func GetUserRole(c *gin.Context) string {
	role, exists := c.Get("role")
	if !exists {
		return ""
	}
	return role.(string)
}

// GetPropertyID extracts property ID from context
func GetPropertyID(c *gin.Context) uint {
	propertyID, exists := c.Get("property_id")
	if !exists {
		return 0
	}
	return propertyID.(uint)
}

// IsPropertyAdmin checks if user is a property admin or super admin
func IsPropertyAdmin(c *gin.Context) bool {
	role := GetUserRole(c)
	return role == "property_admin" || role == "super_admin"
}