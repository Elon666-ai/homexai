package middleware

import (
	"strings"

	"homexai/internal/database"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TenantMiddleware identifies the property (tenant) from subdomain, header, or JWT token
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var subdomain string

		// Try to get subdomain from header first (for API testing)
		subdomain = c.GetHeader("X-Property-Subdomain")

		// If not in header, extract from Host
		if subdomain == "" {
			host := c.Request.Host
			subdomain = extractSubdomain(host)
		}

		// Check if this is the main portal (admin subdomain)
		if isMainPortal(subdomain) {
			role := GetUserRole(c)
			// Main portal is only for super_admin
			if role == "super_admin" {
				c.Set("is_main_portal", true)
				c.Next()
				return
			}
			// Non-super_admin users should not access main portal
			utils.ForbiddenResponse(c, "Access denied. Please use your property portal.")
			c.Abort()
			return
		}

		// If still no subdomain, try to get property_id from JWT token
		if subdomain == "" {
			role := GetUserRole(c)
			propertyID := GetPropertyID(c)

			// Super admin can operate without a specific property
			if role == "super_admin" {
				c.Next()
				return
			}

			// For property_admin/other roles with property_id in token
			if propertyID > 0 {
				// Get property subdomain from master DB by property_id
				type PropertyInfo struct {
					ID        uint   `gorm:"column:id"`
					Name      string `gorm:"column:name"`
					Subdomain string `gorm:"column:subdomain"`
					Status    string `gorm:"column:status"`
				}

				var property PropertyInfo
				err := database.GetMasterGormDB().Table("properties").
					Where("id = ?", propertyID).
					First(&property).Error

				if err == nil && property.Status == "active" {
					subdomain = property.Subdomain
				}
			}
		}

		// Final check: if still no subdomain, return error
		if subdomain == "" {
			// Provide more helpful error message
			role := GetUserRole(c)
			propertyID := GetPropertyID(c)
			if propertyID > 0 && role != "super_admin" {
				// User has property_id but we couldn't get subdomain
				utils.NotFoundResponse(c, "Property not found. Please ensure you are accessing through the correct property subdomain.")
			} else {
				utils.BadRequestResponse(c, "Property subdomain required. Please provide X-Property-Subdomain header or access through property subdomain.", nil)
			}
			c.Abort()
			return
		}

		// Get property database connection
		propertyDB, err := database.GetPropertyDBBySubdomain(subdomain)
		if err != nil {
			utils.NotFoundResponse(c, "Property not found. Please check if the property subdomain is correct and the property database exists.")
			c.Abort()
			return
		}

		// Get property information from master DB
		type PropertyInfo struct {
			ID        uint   `gorm:"column:id"`
			Name      string `gorm:"column:name"`
			Subdomain string `gorm:"column:subdomain"`
			Status    string `gorm:"column:status"`
		}

		var property PropertyInfo
		err = database.GetMasterGormDB().Table("properties").
			Where("subdomain = ?", subdomain).
			First(&property).Error

		if err != nil {
			utils.NotFoundResponse(c, "Property not found")
			c.Abort()
			return
		}

		// Check if property is active
		if property.Status != "active" {
			utils.ForbiddenResponse(c, "Property is not active")
			c.Abort()
			return
		}

		// Set property context
		c.Set("property_id", property.ID)
		c.Set("property_name", property.Name)
		c.Set("subdomain", subdomain)
		c.Set("property_db", propertyDB)
		// fmt.Printf("middleware set subdomain=%s,property_id=%d,property_name=%s,property_db=%v\n", subdomain, property.ID, property.Name, propertyDB)

		c.Next()
	}
}

// OptionalTenantMiddleware identifies property but doesn't require it
func OptionalTenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var subdomain string

		subdomain = c.GetHeader("X-Property-Subdomain")
		if subdomain == "" {
			host := c.Request.Host
			subdomain = extractSubdomain(host)
		}

		// Main portal doesn't need property context
		if isMainPortal(subdomain) {
			c.Set("is_main_portal", true)
			c.Next()
			return
		}

		if subdomain == "" {
			c.Next()
			return
		}

		propertyDB, err := database.GetPropertyDBBySubdomain(subdomain)
		if err != nil {
			c.Next()
			return
		}

		type PropertyInfo struct {
			ID        uint   `gorm:"column:id"`
			Name      string `gorm:"column:name"`
			Subdomain string `gorm:"column:subdomain"`
			Status    string `gorm:"column:status"`
		}

		var property PropertyInfo
		err = database.GetMasterGormDB().Table("properties").
			Where("subdomain = ?", subdomain).
			First(&property).Error

		if err != nil {
			c.Next()
			return
		}

		c.Set("property_id", property.ID)
		c.Set("property_name", property.Name)
		c.Set("subdomain", subdomain)
		c.Set("property_db", propertyDB)

		c.Next()
	}
}

// extractSubdomain extracts subdomain from host
// Example: demo.homex.ph -> demo
// Example: admin.localhost -> admin
// Example: admin.pp-cdn.org -> admin
func extractSubdomain(host string) string {
	// Remove port if present
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	// Split by dots
	parts := strings.Split(host, ".")

	// Handle localhost with subdomain (e.g., demo.localhost, admin.localhost)
	if len(parts) == 2 && parts[1] == "localhost" {
		return parts[0]
	}

	// If localhost or IP without subdomain, no subdomain
	if len(parts) < 3 {
		return ""
	}

	// Return first part as subdomain
	return parts[0]
}

// isMainPortal checks if the subdomain represents the main admin portal
// Main portal subdomains: "admin"
// This is where super_admin users access the system
func isMainPortal(subdomain string) bool {
	return subdomain == "admin"
}

// GetPropertyDB retrieves property database from context
func GetPropertyDB(c *gin.Context) *gorm.DB {
	db, ok := c.Get("property_db")
	if !ok {
		return nil
	}
	return db.(*gorm.DB)
}

// IsMainPortalRequest checks if the current request is from the main admin portal
func IsMainPortalRequest(c *gin.Context) bool {
	isMainPortal, exists := c.Get("is_main_portal")
	if !exists {
		return false
	}
	return isMainPortal.(bool)
}

// RequirePropertyContext ensures property context exists
func RequirePropertyContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := c.Get("property_id")
		if !exists {
			utils.BadRequestResponse(c, "Property context required", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// GetPropertyName retrieves property name from context
func GetPropertyName(c *gin.Context) string {
	name, ok := c.Get("property_name")
	if !ok {
		return ""
	}
	return name.(string)
}
