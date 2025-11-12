package main

import (
	"fmt"
	"homexai_oauth/config"
	"homexai_oauth/database"
	"homexai_oauth/routes"
	"homexai_oauth/services"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// 加载配置
	cfg := config.LoadConfig()
	
	// 验证Google配置（如果要使用Google登录）
	googleConfigured := cfg.GoogleClientID != "" && 
		cfg.GoogleClientID != "your-google-client-id.apps.googleusercontent.com"
	
	// 验证Facebook配置（如果要使用Facebook登录）
	facebookConfigured := cfg.FacebookAppID != "" && 
		cfg.FacebookAppID != "your-facebook-app-id"
	
	if !googleConfigured && !facebookConfigured {
		log.Println("❌ 错误: 至少需要配置一个OAuth提供商")
		log.Println("")
		log.Println("请配置以下任一选项:")
		log.Println("1. Google OAuth - 编辑 .env 文件配置 GOOGLE_CLIENT_ID 和 GOOGLE_CLIENT_SECRET")
		log.Println("2. Facebook OAuth - 编辑 .env 文件配置 FACEBOOK_APP_ID 和 FACEBOOK_APP_SECRET")
		log.Println("")
		log.Println("详细配置步骤请查看:")
		log.Println("- Google: GOOGLE_OAUTH_SETUP.md")
		log.Println("- Facebook: FACEBOOK_OAUTH_SETUP.md")
		os.Exit(1)
	}
	
	// 显示已配置的提供商
	log.Println("✓ 配置加载成功")
	if googleConfigured {
		log.Printf("  ✓ Google OAuth 已配置")
		log.Printf("    Client ID: %s...%s", cfg.GoogleClientID[:20], cfg.GoogleClientID[len(cfg.GoogleClientID)-20:])
	} else {
		log.Println("  ⚠ Google OAuth 未配置")
	}
	
	if facebookConfigured {
		log.Printf("  ✓ Facebook OAuth 已配置")
		log.Printf("    App ID: %s", cfg.FacebookAppID)
	} else {
		log.Println("  ⚠ Facebook OAuth 未配置")
	}
	log.Println("")

	// 初始化数据库
	if err := database.InitDB(cfg.DatabasePath); err != nil {
		log.Fatal("Failed to initialize database:", err)
	}

	// 创建认证服务
	authService := services.NewAuthService(cfg)

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
	router.StaticFile("/auth-success.html", "./static/auth-success.html")

	// 设置路由
	routes.SetupRoutes(router, authService)

	// 启动服务器
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("Server is running on http://%s%s", cfg.ServerHost, addr)
	if googleConfigured {
		log.Printf("Google OAuth redirect URL: %s", cfg.GoogleRedirectURL)
	}
	if facebookConfigured {
		log.Printf("Facebook OAuth redirect URL: %s", cfg.FacebookRedirectURL)
	}
	log.Println("")
	
	if err := router.Run(addr); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
