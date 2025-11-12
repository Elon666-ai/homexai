package main

import (
	"fmt"
	"homexai_email_auth/config"
	"homexai_email_auth/database"
	"homexai_email_auth/routes"
	"homexai_email_auth/services"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg := config.LoadConfig()
	
	// 验证SMTP配置
	if cfg.SMTPUsername == "" || cfg.SMTPPassword == "" {
		log.Println("❌ 错误: SMTP 邮件服务未配置")
		log.Println("")
		log.Println("请配置 SMTP 邮件服务:")
		log.Println("1. 编辑 .env 文件")
		log.Println("2. 填入 SMTP_USERNAME 和 SMTP_PASSWORD")
		log.Println("")
		log.Println("Gmail 用户:")
		log.Println("- 开启两步验证")
		log.Println("- 生成应用专用密码: https://myaccount.google.com/apppasswords")
		log.Println("- 使用应用密码作为 SMTP_PASSWORD")
		log.Println("")
		log.Println("详细配置请查看: EMAIL_SETUP.md")
		os.Exit(1)
	}
	
	log.Println("✓ 配置加载成功")
	log.Printf("  SMTP Host: %s:%d", cfg.SMTPHost, cfg.SMTPPort)
	log.Printf("  SMTP User: %s", cfg.SMTPUsername)
	log.Println("")

	// 初始化数据库
	if err := database.InitDB(cfg.DatabasePath); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// 创建服务
	emailService := services.NewEmailService(cfg)
	authService := services.NewAuthService(cfg, emailService)

	// 创建Gin路由器
	router := gin.Default()

	// 配置CORS
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:8080", "http://127.0.0.1:8080"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// 设置静态文件服务
	router.Static("/static", "./static")
	router.StaticFile("/", "./static/index.html")
	router.StaticFile("/register", "./static/register.html")
	router.StaticFile("/login", "./static/login.html")
	router.StaticFile("/forgot-password", "./static/forgot-password.html")
	router.StaticFile("/reset-password", "./static/reset-password.html")
	router.StaticFile("/dashboard", "./static/dashboard.html")

	// 设置路由
	routes.SetupRoutes(router, authService)

	// 启动服务器
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("Server is running on http://%s%s", cfg.ServerHost, addr)
	log.Println("")
	log.Println("可用页面:")
	log.Println("  - 注册: http://localhost:8080/register")
	log.Println("  - 登录: http://localhost:8080/login")
	log.Println("  - 忘记密码: http://localhost:8080/forgot-password")
	log.Println("")
	
	if err := router.Run(addr); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
