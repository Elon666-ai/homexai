package middleware

import (
	"homexai/internal/database"
	"homexai/internal/models/master"

	"github.com/gin-gonic/gin"
)

// I18nMiddleware handles internationalization
func I18nMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		lang := determineLanguage(c)
		c.Set("language", lang)
		c.Next()
	}
}

// determineLanguage determines the language for the request
// Priority: Query Parameter > User Preference > Accept-Language Header > Default
func determineLanguage(c *gin.Context) string {
	// 1. Check query parameter
	lang := c.Query("lang")
	if lang != "" && master.IsValidLanguage(lang) {
		return lang
	}

	// 2. Check user preference (if authenticated)
	userID := GetUserID(c)
	if userID != 0 {
		var user master.User
		err := database.GetMasterGormDB().Select("preferred_language").
			Where("id = ?", userID).
			First(&user).Error

		if err == nil && user.PreferredLanguage != "" {
			return user.PreferredLanguage
		}
	}

	// 3. Check Accept-Language header
	acceptLang := c.GetHeader("Accept-Language")
	if acceptLang != "" {
		// Parse Accept-Language header (simplified)
		// Example: en-US,en;q=0.9,zh-CN;q=0.8
		lang = parseAcceptLanguage(acceptLang)
		if master.IsValidLanguage(lang) {
			return lang
		}
	}

	// 4. Default language
	return master.LangEnglish
}

// parseAcceptLanguage parses Accept-Language header and returns best match
func parseAcceptLanguage(acceptLang string) string {
	// Simplified parser - just take the first language code
	if len(acceptLang) >= 2 {
		code := acceptLang[:2]

		// Map common codes to our language codes
		switch code {
		case "en":
			return master.LangEnglish
		case "zh":
			// Check if it's traditional or simplified
			if len(acceptLang) >= 5 {
				variant := acceptLang[:5]
				if variant == "zh-TW" || variant == "zh-HK" {
					return master.LangChineseTraditional
				}
			}
			return master.LangChineseSimplified
		case "tl":
			return master.LangTagalog
		}
	}

	return master.LangEnglish
}

// GetLanguage retrieves language from context
func GetLanguage(c *gin.Context) string {
	lang, exists := c.Get("language")
	if !exists {
		return master.LangEnglish
	}
	return lang.(string)
}

// Translate retrieves translation for a key
func Translate(c *gin.Context, key string) string {
	lang := GetLanguage(c)

	var translation master.Translation
	err := database.GetMasterGormDB().
		Where("translation_key = ? AND language = ?", key, lang).
		First(&translation).Error

	if err != nil {
		// Return key if translation not found
		return key
	}

	return translation.TranslationValue
}

// TranslateWithDefault retrieves translation with fallback
func TranslateWithDefault(c *gin.Context, key string, defaultValue string) string {
	lang := GetLanguage(c)

	var translation master.Translation
	err := database.GetMasterGormDB().
		Where("translation_key = ? AND language = ?", key, lang).
		First(&translation).Error

	if err != nil {
		return defaultValue
	}

	return translation.TranslationValue
}

// TranslateMultiple retrieves multiple translations at once
func TranslateMultiple(c *gin.Context, keys []string) map[string]string {
	lang := GetLanguage(c)
	result := make(map[string]string)

	var translations []master.Translation
	database.GetMasterGormDB().
		Where("translation_key IN ? AND language = ?", keys, lang).
		Find(&translations)

	// Map translations
	for _, t := range translations {
		result[t.TranslationKey] = t.TranslationValue
	}

	// Fill missing keys with the key itself
	for _, key := range keys {
		if _, exists := result[key]; !exists {
			result[key] = key
		}
	}

	return result
}
