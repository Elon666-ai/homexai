package middleware

import (
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ValidateMiddleware validates request body against a struct
func ValidateMiddleware(obj interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := c.ShouldBindJSON(obj); err != nil {
			if validationErrors, ok := err.(validator.ValidationErrors); ok {
				errors := formatValidationErrors(validationErrors)
				utils.ValidationErrorResponse(c, errors)
				c.Abort()
				return
			}

			utils.BadRequestResponse(c, "Invalid request body", err)
			c.Abort()
			return
		}

		// Store validated object in context
		c.Set("validated_data", obj)
		c.Next()
	}
}

// formatValidationErrors formats validator errors into a readable format
func formatValidationErrors(errs validator.ValidationErrors) []map[string]string {
	var errors []map[string]string

	for _, err := range errs {
		errors = append(errors, map[string]string{
			"field":   err.Field(),
			"tag":     err.Tag(),
			"value":   err.Param(),
			"message": getErrorMessage(err),
		})
	}

	return errors
}

// getErrorMessage returns a user-friendly error message for validation errors
func getErrorMessage(err validator.FieldError) string {
	field := err.Field()

	switch err.Tag() {
	case "required":
		return field + " is required"
	case "email":
		return field + " must be a valid email address"
	case "min":
		return field + " must be at least " + err.Param() + " characters"
	case "max":
		return field + " must be at most " + err.Param() + " characters"
	case "len":
		return field + " must be exactly " + err.Param() + " characters"
	case "numeric":
		return field + " must be a number"
	case "alpha":
		return field + " must contain only letters"
	case "alphanum":
		return field + " must contain only letters and numbers"
	case "url":
		return field + " must be a valid URL"
	case "oneof":
		return field + " must be one of: " + err.Param()
	case "gte":
		return field + " must be greater than or equal to " + err.Param()
	case "lte":
		return field + " must be less than or equal to " + err.Param()
	case "gt":
		return field + " must be greater than " + err.Param()
	case "lt":
		return field + " must be less than " + err.Param()
	case "eqfield":
		return field + " must equal " + err.Param()
	case "nefield":
		return field + " must not equal " + err.Param()
	default:
		return field + " is invalid"
	}
}

// GetValidatedData retrieves validated data from context
func GetValidatedData(c *gin.Context) interface{} {
	data, exists := c.Get("validated_data")
	if !exists {
		return nil
	}
	return data
}
