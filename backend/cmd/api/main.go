package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"homexai/internal/config"
	"homexai/internal/database"
	propertyRepo "homexai/internal/repository/property"
	"homexai/internal/routes"
	"homexai/internal/service"
	"homexai/internal/tracer"
	"homexai/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/pbnjay/memory"
	"gorm.io/gorm"
)

var BuildTime string

func main() {
	var err error
	if len(os.Args) == 2 && (os.Args[1] == "version" || os.Args[1] == "-V" || os.Args[1] == "-v") {
		fmt.Println(utils.APP_NAME, utils.VERSION, "BuildTime:", BuildTime, utils.GetVersionModification())
		return
	}

	defer tracer.TryException()
	utils.AppendResetLog()

	numCPU := runtime.NumCPU()
	debug.SetGCPercent(200)
	capacity := memory.TotalMemory()
	debug.SetMemoryLimit(int64(capacity * 7 / 10))
	runtime.GOMAXPROCS(numCPU)
	strBuildInfo := fmt.Sprintf("%s[%s], version=%s, BuildTime=%s, cpunum=%d, memory=%dGB",
		utils.APP_NAME, utils.GetRunEnv(), utils.VERSION, BuildTime, numCPU, capacity/1024/1024/1024)
	tracer.LogInfo(tracer.ID_APP, "%v", strBuildInfo)

	confFile := utils.APP_NAME + "-" + utils.GetRunEnv() + ".yaml"
	if err = config.InitConfig(confFile); err != nil {
		log.Println("[ERROR] init config failed:", err)
		os.Exit(1)
	}
	tracer.LogInfo(tracer.ID_APP, "load %s.yaml success!", confFile)

	// Initialize database connections
	if err := database.InitMasterDB(); err != nil {
		log.Fatalf("Failed to initialize master database: %v", err)
	} else {
		tracer.LogInfo(tracer.ID_APP, "initialize master database success!")
		defer database.CloseMasterDB()
	}

	database.InitPropertyDBPool()

	if err := database.InitRedis(); err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	} else {
		tracer.LogInfo(tracer.ID_APP, "initialize redis client success!")
		defer database.CloseRedisClient()
	}

	// Set Gin mode
	if !config.Yaml.Server.AppDebug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	router := gin.New()

	// Setup routes
	masterDB := database.GetMasterDB()
	routes.SetupRoutes(router, masterDB)
	routes.SetupSwaggerRoutes(router)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start background tasks for forum maintenance
	go startForumMaintenanceTasks(ctx, masterDB.DB)

	// Create HTTP server
	addr := fmt.Sprintf(":%d", config.Yaml.Server.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	tracer.LogInfo(tracer.ID_APP, "Starting HomeX API Server on %s", addr)
	tracer.LogInfo(tracer.ID_APP, "Debug Mode: %v", config.Yaml.Server.AppDebug)
	tracer.LogInfo(tracer.ID_APP, "Swagger Docs: http://localhost%s/swagger/index.html", addr)

	// Start server in goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	// SIGINT: Ctrl+C, SIGTERM: kill command
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	tracer.LogInfo(tracer.ID_APP, "Shutting down server...")

	// Cancel background tasks
	cancel()

	// Give outstanding requests 10 seconds to complete
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		tracer.LogError(tracer.ID_APP, "Server forced to shutdown: %v", err)
	}

	// Close database connections
	tracer.LogInfo(tracer.ID_APP, "Closing database connections...")
	if err := database.CloseAllPropertyDBs(); err != nil {
		tracer.LogError(tracer.ID_APP, "Error closing property databases: %v", err)
	}

	tracer.LogInfo(tracer.ID_APP, "Server exited gracefully")
}

// startForumMaintenanceTasks starts background tasks for forum maintenance
// It can be gracefully stopped via context cancellation
func startForumMaintenanceTasks(ctx context.Context, masterDB *gorm.DB) {
	ticker := time.NewTicker(1 * time.Hour) // Run every hour
	defer ticker.Stop()

	tracer.LogInfo(tracer.ID_APP, "Forum maintenance tasks started")

	// Run immediately on startup
	runForumMaintenanceTasks(masterDB)

	for {
		select {
		case <-ticker.C:
			runForumMaintenanceTasks(masterDB)
		case <-ctx.Done():
			tracer.LogInfo(tracer.ID_APP, "Forum maintenance tasks stopped gracefully")
			return
		}
	}
}

// runForumMaintenanceTasks runs forum maintenance tasks for all property databases
func runForumMaintenanceTasks(masterDB *gorm.DB) {
	// Get all property database names
	propertyDBs, err := database.GetAllPropertyDBNames()
	if err != nil {
		tracer.LogError(tracer.ID_APP, "Failed to get property databases: %v", err)
		return
	}

	for _, dbName := range propertyDBs {
		propertyDB, err := database.GetPropertyDB(dbName)
		if err != nil {
			tracer.LogError(tracer.ID_APP, "Failed to get property database %s: %v", dbName, err)
			continue
		}

		// Check and lock expired votes
		forumRepo := propertyRepo.NewForumRepository(propertyDB)
		forumService := service.NewForumService(forumRepo)

		if err := forumService.CheckAndLockExpiredVotes(propertyDB); err != nil {
			tracer.LogError(tracer.ID_APP, "Failed to check expired votes for %s: %v", dbName, err)
		}

		// Hard delete old posts (soft deleted 15+ days ago)
		if err := forumService.HardDeleteOldPosts(propertyDB); err != nil {
			tracer.LogError(tracer.ID_APP, "Failed to hard delete old posts for %s: %v", dbName, err)
		}
	}

	tracer.LogInfo(tracer.ID_APP, "Forum maintenance tasks completed for %d property databases", len(propertyDBs))
}
