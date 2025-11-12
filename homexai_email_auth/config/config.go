package config

import (
	"bufio"
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// SMTP配置
	SMTPHost      string
	SMTPPort      int
	SMTPUsername  string
	SMTPPassword  string
	SMTPFromName  string
	SMTPFromEmail string
	
	// JWT配置
	JWTSecret string
	
	// 服务器配置
	ServerPort  string
	ServerHost  string
	FrontendURL string
	
	// 数据库配置
	DatabasePath string
	
	// 验证码配置
	VerificationCodeExpireMinutes int
	PasswordResetTokenExpireMinutes int
}

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
	
	smtpPort, _ := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	verificationExpire, _ := strconv.Atoi(getEnv("VERIFICATION_CODE_EXPIRE_MINUTES", "10"))
	passwordResetExpire, _ := strconv.Atoi(getEnv("PASSWORD_RESET_TOKEN_EXPIRE_MINUTES", "30"))
	
	return &Config{
		SMTPHost:                        getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:                        smtpPort,
		SMTPUsername:                    getEnv("SMTP_USERNAME", ""),
		SMTPPassword:                    getEnv("SMTP_PASSWORD", ""),
		SMTPFromName:                    getEnv("SMTP_FROM_NAME", "HomeX AI"),
		SMTPFromEmail:                   getEnv("SMTP_FROM_EMAIL", ""),
		JWTSecret:                       getEnv("JWT_SECRET", "your-super-secret-jwt-key"),
		ServerPort:                      getEnv("SERVER_PORT", "8080"),
		ServerHost:                      getEnv("SERVER_HOST", "localhost"),
		FrontendURL:                     getEnv("FRONTEND_URL", "http://localhost:8080"),
		DatabasePath:                    getEnv("DATABASE_PATH", "./homexai.db"),
		VerificationCodeExpireMinutes:   verificationExpire,
		PasswordResetTokenExpireMinutes: passwordResetExpire,
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
