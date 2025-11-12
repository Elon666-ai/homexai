package config

import (
	"bufio"
	"log"
	"os"
	"strings"
)

type Config struct {
	// Google OAuth
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	
	// Facebook OAuth
	FacebookAppID      string
	FacebookAppSecret  string
	FacebookRedirectURL string
	
	// JWT
	JWTSecret          string
	
	// Server
	ServerPort         string
	ServerHost         string
	FrontendURL        string
	
	// Database
	DatabasePath       string
}

// LoadEnvFile 从 .env 文件加载环境变量
func LoadEnvFile() error {
	file, err := os.Open(".env")
	if err != nil {
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
	
	if err := scanner.Err(); err != nil {
		return err
	}
	
	log.Println("✓ 已加载 .env 文件配置")
	return nil
}

func LoadConfig() *Config {
	if err := LoadEnvFile(); err != nil {
		log.Printf("警告: 加载 .env 文件失败: %v", err)
	}
	
	return &Config{
		GoogleClientID:      getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:  getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURL:   getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),
		
		FacebookAppID:       getEnv("FACEBOOK_APP_ID", ""),
		FacebookAppSecret:   getEnv("FACEBOOK_APP_SECRET", ""),
		FacebookRedirectURL: getEnv("FACEBOOK_REDIRECT_URL", "http://localhost:8080/auth/facebook/callback"),
		
		JWTSecret:           getEnv("JWT_SECRET", "your-super-secret-jwt-key"),
		ServerPort:          getEnv("SERVER_PORT", "8080"),
		ServerHost:          getEnv("SERVER_HOST", "localhost"),
		FrontendURL:         getEnv("FRONTEND_URL", "http://localhost:8080"),
		DatabasePath:        getEnv("DATABASE_PATH", "./homexai.db"),
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
