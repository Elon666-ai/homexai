package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"homexai/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// MasterDB wraps the master database connection
type MasterDB struct {
	DB *gorm.DB
}

var masterDBInstance *MasterDB

// InitMasterDB initializes the master database connection with keep-alive mechanism
// ✅ 修复：添加完整的连接保活机制
func InitMasterDB() error {
	dsn := config.Yaml.GetMasterDSN()

	// Configure GORM logger
	var gormLogger logger.Interface
	if config.Yaml.Server.AppDebug {
		gormLogger = logger.Default.LogMode(logger.Info)
	} else {
		gormLogger = logger.Default.LogMode(logger.Error)
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLogger,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		// 连接池优化配置
		PrepareStmt:              true,  // 启用预编译语句缓存
		DisableNestedTransaction: false, // 允许嵌套事务
	})

	if err != nil {
		return fmt.Errorf("failed to connect to master database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	// ===== 核心保活配置 =====

	// 1. 连接池基础设置（带验证）
	maxOpenConns := config.Yaml.Database.MaxOpenConns
	if maxOpenConns <= 0 {
		maxOpenConns = 100 // 默认最大连接数
	}
	maxIdleConns := config.Yaml.Database.MaxIdleConns
	if maxIdleConns <= 0 {
		maxIdleConns = 10 // 默认空闲连接数
	}
	if maxIdleConns > maxOpenConns {
		maxIdleConns = maxOpenConns / 2
	}

	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)

	// 2. 连接生命周期管理（保活关键）
	connMaxLifetime := time.Duration(config.Yaml.Database.ConnMaxLifetime) * time.Second
	if connMaxLifetime <= 0 {
		connMaxLifetime = 3600 * time.Second // 默认 1 小时
	}
	// 设置为 MySQL wait_timeout 的一半，确保在服务器关闭前主动回收
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	// 3. 空闲连接超时（保活关键）⭐
	// 设置为 1 分钟，定期清理空闲连接并重建，防止连接过期
	// MySQL wait_timeout 默认是 120 秒，这里设置为 60 秒确保在服务器关闭前回收
	sqlDB.SetConnMaxIdleTime(60 * time.Second)

	// 4. 测试连接（带重试）
	var pingErr error
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		pingErr = sqlDB.PingContext(ctx)
		cancel()

		if pingErr == nil {
			break
		}
		log.Printf("⚠️  Ping attempt %d/3 failed: %v, retrying...", i+1, pingErr)
		time.Sleep(time.Second * 2)
	}
	if pingErr != nil {
		return fmt.Errorf("failed to ping master database after 3 attempts: %w", pingErr)
	}

	masterDBInstance = &MasterDB{DB: db}

	// 5. 启动连接保活后台任务（关键）⭐
	go startConnectionKeepAlive(sqlDB, 30*time.Second)

	log.Println("✓ Master database connected successfully")
	log.Printf("  - Max Open Connections: %d", maxOpenConns)
	log.Printf("  - Max Idle Connections: %d", maxIdleConns)
	log.Printf("  - Connection Max Lifetime: %v", connMaxLifetime)
	log.Printf("  - Connection Max Idle Time: 5m")
	log.Printf("  - Keep-Alive: Enabled (30s interval)")

	return nil
}

// startConnectionKeepAlive 启动连接保活后台任务
// 定期 ping 数据库，确保连接池中的连接保持活跃
// ✅ 新增：后台保活机制
func startConnectionKeepAlive(db *sql.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := db.PingContext(ctx)
		cancel()

		if err != nil {
			log.Printf("⚠️  Keep-alive ping failed: %v", err)
			// 尝试重新建立连接
			if err := db.Ping(); err != nil {
				log.Printf("❌ Connection recovery failed: %v", err)
			} else {
				log.Println("✓ Connection recovered successfully")
			}
		}
		// 成功时静默，避免日志污染
	}
}

// GetMasterDB returns the master database instance wrapper
func GetMasterDB() *MasterDB {
	return masterDBInstance
}

// GetMasterGormDB returns the raw GORM database instance
func GetMasterGormDB() *gorm.DB {
	if masterDBInstance != nil {
		return masterDBInstance.DB
	}
	return nil
}

// CloseMasterDB closes the master database connection
func CloseMasterDB() error {
	if masterDBInstance != nil && masterDBInstance.DB != nil {
		sqlDB, err := masterDBInstance.DB.DB()
		if err != nil {
			return err
		}
		log.Println("Closing master database connection...")
		return sqlDB.Close()
	}
	return nil
}

// HealthCheck performs a health check on the master database
// ✅ 新增：健康检查方法
func HealthCheck() error {
	if masterDBInstance == nil || masterDBInstance.DB == nil {
		return fmt.Errorf("master database not initialized")
	}

	sqlDB, err := masterDBInstance.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}

// GetConnectionStats returns database connection pool statistics
// ✅ 新增：获取连接池统计信息
func GetConnectionStats() sql.DBStats {
	if masterDBInstance == nil || masterDBInstance.DB == nil {
		return sql.DBStats{}
	}

	sqlDB, err := masterDBInstance.DB.DB()
	if err != nil {
		return sql.DBStats{}
	}

	return sqlDB.Stats()
}
