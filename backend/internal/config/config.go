package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type EnvConfig2 struct {
	App       AppConfig2
	Database  DatabaseConfig2
	Redis     RedisConfig2
	JWT       JWTConfig2
	OAuth     OAuthConfig2
	Email     EmailConfig2
	Smtp      SMTPConfig2
	SMS       SMSConfig2
	Upload    UploadConfig2
	CORS      CORSConfig2
	Turnstile TurnstileConfig2
	RateLimit RateLimitConfig2
}

type AppConfig2 struct {
	Name        string
	Port        string
	Debug       bool
	FrontendURL string
}

type DatabaseConfig2 struct {
	Master DatabaseConnection2
}

type DatabaseConnection2 struct {
	Host            string
	Port            string
	Name            string
	User            string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
}

type RedisConfig2 struct {
	Host     string
	Port     string
	Password string
	DB       int
}

type JWTConfig2 struct {
	Secret                 string
	ExpirationHours        int
	RefreshExpirationHours int
}

type OAuthConfig2 struct {
	Google   OAuthProvider2
	Facebook OAuthProvider2
}

type OAuthProvider2 struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type EmailConfig2 struct {
	SendGridAPIKey string
	FromEmail      string
	FromName       string
}

type SMTPConfig2 struct {
	SMTPHost      string
	SMTPPort      int
	SMTPUsername  string
	SMTPPassword  string
	SMTPFromName  string
	SMTPFromEmail string
	EmailConfig2
}

type SMSConfig2 struct {
	TwilioAccountSID  string
	TwilioAuthToken   string
	TwilioPhoneNumber string
}

type UploadConfig2 struct {
	MaxSizeMB    int
	AllowedTypes []string
	Path         string
}

type CORSConfig2 struct {
	AllowedOrigins   []string
	AllowCredentials bool
}

type TurnstileConfig2 struct {
	Enabled   bool
	SiteKey   string
	SecretKey string
}

type RateLimitConfig2 struct {
	Enabled           bool
	RequestsPerMinute int
}

// var GlobalConfig *EnvConfig2

// Load loads configuration from environment variables
func Load() (*EnvConfig2, error) {
	var err error

	// Load .env file if exists
	if err = godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	config := &EnvConfig2{
		App: AppConfig2{
			Name:        getEnv("APP_NAME", "HomeX"),
			Port:        getEnv("APP_PORT", "8080"),
			Debug:       getEnvBool("APP_DEBUG", true),
			FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
		},
		Database: DatabaseConfig2{
			Master: DatabaseConnection2{
				Host:            getEnv("DB_MASTER_HOST", "localhost"),
				Port:            getEnv("DB_MASTER_PORT", "3306"),
				Name:            getEnv("DB_MASTER_NAME", "homexai_master"),
				User:            getEnv("DB_MASTER_USER", "root"),
				Password:        getEnv("DB_MASTER_PASSWORD", "123456"),
				MaxOpenConns:    getEnvInt("DB_MASTER_MAX_OPEN_CONNS", 100),
				MaxIdleConns:    getEnvInt("DB_MASTER_MAX_IDLE_CONNS", 10),
				ConnMaxLifetime: getEnvInt("DB_MASTER_CONN_MAX_LIFETIME", 3600),
			},
		},
		Redis: RedisConfig2{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		JWT: JWTConfig2{
			Secret:                 getEnv("JWT_SECRET", "your-secret-key"),
			ExpirationHours:        getEnvInt("JWT_EXPIRATION_HOURS", 24),
			RefreshExpirationHours: getEnvInt("JWT_REFRESH_EXPIRATION_HOURS", 168),
		},
		OAuth: OAuthConfig2{
			Google: OAuthProvider2{
				ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
				ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
				RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", ""),
			},
			Facebook: OAuthProvider2{
				ClientID:     getEnv("FACEBOOK_APP_ID", ""),
				ClientSecret: getEnv("FACEBOOK_APP_SECRET", ""),
				RedirectURL:  getEnv("FACEBOOK_REDIRECT_URL", ""),
			},
		},
		Email: EmailConfig2{
			SendGridAPIKey: getEnv("SENDGRID_API_KEY", ""),
			FromEmail:      getEnv("SENDGRID_FROM_EMAIL", "noreply@homex.ph"),
			FromName:       getEnv("SENDGRID_FROM_NAME", "HomeX"),
		},
		Smtp: SMTPConfig2{
			SMTPHost:      getEnv("SMTP_HOST", "smtp.gmail.com"),
			SMTPPort:      getEnvInt("SMTP_PORT", 587),
			SMTPUsername:  getEnv("SMTP_USERNAME", ""),
			SMTPPassword:  getEnv("SMTP_PASSWORD", ""),
			SMTPFromName:  getEnv("SMTP_FROM_NAME", "HomeX AI"),
			SMTPFromEmail: getEnv("SMTP_FROM_EMAIL", ""),
		},
		SMS: SMSConfig2{
			TwilioAccountSID:  getEnv("TWILIO_ACCOUNT_SID", ""),
			TwilioAuthToken:   getEnv("TWILIO_AUTH_TOKEN", ""),
			TwilioPhoneNumber: getEnv("TWILIO_PHONE_NUMBER", ""),
		},
		Upload: UploadConfig2{
			MaxSizeMB:    getEnvInt("UPLOAD_MAX_SIZE_MB", 10),
			AllowedTypes: getEnvSlice("UPLOAD_ALLOWED_TYPES", []string{"jpg", "jpeg", "png", "pdf"}),
			Path:         getEnv("UPLOAD_PATH", "./uploads"),
		},
		CORS: CORSConfig2{
			AllowedOrigins:   getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),
			AllowCredentials: getEnvBool("CORS_ALLOW_CREDENTIALS", true),
		},
		Turnstile: TurnstileConfig2{
			Enabled:   getEnvBool("TURNSTILE_ENABLED", false),
			SiteKey:   getEnv("TURNSTILE_SITE_KEY", ""),
			SecretKey: getEnv("TURNSTILE_SECRET_KEY", ""),
		},
		RateLimit: RateLimitConfig2{
			Enabled:           getEnvBool("RATE_LIMIT_ENABLED", true),
			RequestsPerMinute: getEnvInt("RATE_LIMIT_REQUESTS_PER_MINUTE", 60),
		},
	}

	// Set global config
	// GlobalConfig = config

	return config, nil
}

// Helper functions
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intVal, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intVal
}

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	boolVal, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return boolVal
}

func getEnvSlice(key string, defaultValue []string) []string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return strings.Split(value, ",")
}

// GetMasterDSN returns the DSN for master database with enhanced parameters
// ✅ 修复：添加超时和保活参数
func (c *EnvConfig2) GetMasterDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s&readTimeout=30s&writeTimeout=30s&maxAllowedPacket=0&interpolateParams=true",
		c.Database.Master.User,
		c.Database.Master.Password,
		c.Database.Master.Host,
		c.Database.Master.Port,
		c.Database.Master.Name,
	)
}

// GetPropertyDSN returns the DSN for property database with enhanced parameters
// ✅ 修复：添加超时和保活参数
func (c *EnvConfig2) GetPropertyDSN(dbName string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s&readTimeout=30s&writeTimeout=30s&maxAllowedPacket=0&interpolateParams=true",
		c.Database.Master.User,
		c.Database.Master.Password,
		c.Database.Master.Host,
		c.Database.Master.Port,
		dbName,
	)
}
