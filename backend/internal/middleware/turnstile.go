package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"homexai/internal/config"
	"homexai/internal/database"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
)

// TurnstileVerifiedTTL is the cache duration for verified IPs (2 hours)
const TurnstileVerifiedTTL = 2 * time.Hour

// TurnstileResponse represents Cloudflare Turnstile verification response
type TurnstileResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
	Action      string   `json:"action,omitempty"`
	CData       string   `json:"cdata,omitempty"`
}

// TurnstileMiddleware validates Cloudflare Turnstile token
func TurnstileMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Yaml

		// Skip if Turnstile is disabled
		if !cfg.Turnstile.Enable {
			c.Next()
			return
		}

		// Skip verification in localhost/development environment
		host := c.Request.Host
		origin := c.Request.Header.Get("Origin")
		if isLocalhost(host) || isLocalhost(origin) {
			c.Next()
			return
		}

		// Check if this IP has been verified within the last 2 hours
		clientIP := c.ClientIP()
		redisKey := "turnstile:verified:" + clientIP
		if isVerifiedInCache(redisKey) {
			c.Next()
			return
		}

		// Get token from request - read body without consuming it
		token := c.PostForm("cf-turnstile-response")
		if token == "" {
			// Read body and restore it for later use
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil && len(bodyBytes) > 0 {
				// Restore the body so it can be read again by subsequent handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				// Parse JSON to get turnstile token
				var body map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &body); err == nil {
					if t, ok := body["cf_turnstile_response"].(string); ok {
						token = t
					}
				}
			}
		}

		if token == "" {
			utils.BadRequestResponse(c, "Turnstile token required", nil)
			c.Abort()
			return
		}

		// Skip actual verification if token indicates cached/skipped verification
		// TURNSTILE_SKIPPED: graceful degradation when Turnstile fails to load
		// TURNSTILE_CACHED: frontend cached verification (2h validity)
		if token == "TURNSTILE_SKIPPED" || token == "TURNSTILE_CACHED" {
			// Still cache the skip to avoid repeated checks
			cacheVerification(redisKey)
			c.Next()
			return
		}

		// Verify token
		valid, err := verifyTurnstileToken(token, clientIP)
		if err != nil {
			utils.InternalServerErrorResponse(c, "Failed to verify Turnstile token", err)
			c.Abort()
			return
		}

		if !valid {
			utils.ForbiddenResponse(c, "Turnstile verification failed")
			c.Abort()
			return
		}

		// Cache the successful verification for 2 hours
		cacheVerification(redisKey)

		c.Next()
	}
}

// isLocalhost checks if the host is localhost or local IP
func isLocalhost(host string) bool {
	if host == "" {
		return false
	}
	host = strings.ToLower(host)
	return strings.HasPrefix(host, "localhost") ||
		strings.HasPrefix(host, "127.0.0.1") ||
		strings.HasPrefix(host, "http://localhost") ||
		strings.HasPrefix(host, "http://127.0.0.1")
}

// isVerifiedInCache checks if the IP has been verified recently
func isVerifiedInCache(redisKey string) bool {
	exists, err := database.ExistsCache(redisKey)
	if err != nil {
		// If Redis is unavailable, don't skip verification
		return false
	}
	return exists
}

// cacheVerification caches the successful verification
func cacheVerification(redisKey string) {
	// Store "1" as the value, the key existence is what matters
	_ = database.SetCache(redisKey, "1", TurnstileVerifiedTTL)
}

// verifyTurnstileToken verifies the Turnstile token with Cloudflare API
func verifyTurnstileToken(token, remoteIP string) (bool, error) {
	cfg := config.Yaml

	// Prepare request data
	data := url.Values{}
	data.Set("secret", cfg.Turnstile.SiteSecret)
	data.Set("response", token)
	data.Set("remoteip", remoteIP)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Send verification request
	resp, err := client.PostForm(
		"https://challenges.cloudflare.com/turnstile/v0/siteverify",
		data,
	)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	// Parse response
	var result TurnstileResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}

	return result.Success, nil
}

// OptionalTurnstileMiddleware validates Turnstile token if present
func OptionalTurnstileMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.PostForm("cf-turnstile-response")
		if token == "" {
			// Read body and restore it for later use
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil && len(bodyBytes) > 0 {
				// Restore the body so it can be read again by subsequent handlers
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

				// Parse JSON to get turnstile token
				var body map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &body); err == nil {
					if t, ok := body["cf_turnstile_response"].(string); ok {
						token = t
					}
				}
			}
		}

		if token == "" {
			c.Next()
			return
		}

		valid, err := verifyTurnstileToken(token, c.ClientIP())
		if err != nil || !valid {
			utils.ForbiddenResponse(c, "Turnstile verification failed")
			c.Abort()
			return
		}

		c.Next()
	}
}
