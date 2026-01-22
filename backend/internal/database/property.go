package database

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"homexai/internal/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PropertyDBPool manages connections to multiple property databases
type PropertyDBPool struct {
	connections    map[string]*propertyDBConnection
	mu             sync.RWMutex
	cfg            *config.ConfigYaml
	cleanupCtx     context.Context
	cleanupCancel  context.CancelFunc
	cleanupRunning bool
}

// propertyDBConnection wraps a GORM DB connection with metadata
type propertyDBConnection struct {
	DB           *gorm.DB
	LastAccessed time.Time
	CreatedAt    time.Time
}

var propertyPool *PropertyDBPool
var propertyPoolOnce sync.Once

// InitPropertyDBPool initializes the property database connection pool
func InitPropertyDBPool() {
	propertyPoolOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		propertyPool = &PropertyDBPool{
			connections:    make(map[string]*propertyDBConnection),
			cfg:            &config.Yaml,
			cleanupCtx:     ctx,
			cleanupCancel:  cancel,
			cleanupRunning: false,
		}

		// Start cleanup goroutine
		go propertyPool.startCleanupRoutine()

		log.Println("✓ Property database pool initialized with auto-cleanup")
	})
}

// startCleanupRoutine periodically cleans up stale connections
func (p *PropertyDBPool) startCleanupRoutine() {
	p.cleanupRunning = true
	ticker := time.NewTicker(5 * time.Minute) // Run every 5 minutes
	defer ticker.Stop()

	log.Println("Property DB pool cleanup routine started")

	for {
		select {
		case <-ticker.C:
			p.cleanStaleConnections()
		case <-p.cleanupCtx.Done():
			log.Println("Property DB pool cleanup routine stopped")
			p.cleanupRunning = false
			return
		}
	}
}

// cleanStaleConnections removes connections that haven't been used in 30 minutes
func (p *PropertyDBPool) cleanStaleConnections() {
	p.mu.Lock()
	defer p.mu.Unlock()

	staleThreshold := 30 * time.Minute
	now := time.Now()

	var closedCount int
	for dbName, conn := range p.connections {
		if now.Sub(conn.LastAccessed) > staleThreshold {
			// Close the connection
			if sqlDB, err := conn.DB.DB(); err == nil {
				if err := sqlDB.Close(); err != nil {
					log.Printf("⚠ Error closing stale connection to %s: %v", dbName, err)
				} else {
					delete(p.connections, dbName)
					closedCount++
					log.Printf("✓ Closed stale connection to %s (idle for %v)",
						dbName, now.Sub(conn.LastAccessed))
				}
			}
		}
	}

	if closedCount > 0 {
		log.Printf("Cleaned up %d stale database connections", closedCount)
	}
}

// GetPropertyDB returns a connection to a property database
// Creates a new connection if it doesn't exist
func GetPropertyDB(dbName string) (*gorm.DB, error) {
	if propertyPool == nil {
		return nil, fmt.Errorf("property database pool not initialized")
	}

	// Check if connection exists
	propertyPool.mu.RLock()
	conn, exists := propertyPool.connections[dbName]
	propertyPool.mu.RUnlock()

	if exists && conn != nil {
		// Test if connection is still alive
		sqlDB, err := conn.DB.DB()
		if err == nil && sqlDB.Ping() == nil {
			// Update last accessed time
			propertyPool.mu.Lock()
			conn.LastAccessed = time.Now()
			propertyPool.mu.Unlock()
			return conn.DB, nil
		}
		// Connection is dead, remove it
		propertyPool.mu.Lock()
		delete(propertyPool.connections, dbName)
		propertyPool.mu.Unlock()
		log.Printf("⚠ Dead connection to %s removed", dbName)
	}

	// Create new connection
	return propertyPool.createConnection(dbName)
}

// createConnection creates a new connection to a property database
func (p *PropertyDBPool) createConnection(dbName string) (*gorm.DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check if another goroutine already created the connection
	if conn, exists := p.connections[dbName]; exists {
		conn.LastAccessed = time.Now()
		return conn.DB, nil
	}

	dsn := p.cfg.GetPropertyDSN(dbName)

	// Configure GORM logger
	var gormLogger logger.Interface
	if p.cfg.Server.AppDebug {
		gormLogger = logger.Default.LogMode(logger.Info)
	} else {
		gormLogger = logger.Default.LogMode(logger.Error)
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: gormLogger,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
		// Optimize for production
		PrepareStmt:            true,  // Use prepared statements
		DisableAutomaticPing:   false, // Keep ping for connection health
		SkipDefaultTransaction: true,  // Skip default transaction for better performance
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to property database %s: %w", dbName, err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Set connection pool settings with optimized values
	maxOpenConns := p.cfg.Database.MaxOpenConns
	if maxOpenConns == 0 {
		maxOpenConns = 100 // Default value
	}

	maxIdleConns := p.cfg.Database.MaxIdleConns
	if maxIdleConns == 0 {
		maxIdleConns = 10 // Default value
	}

	connMaxLifetime := time.Duration(p.cfg.Database.ConnMaxLifetime) * time.Second
	if connMaxLifetime == 0 {
		connMaxLifetime = 30 * time.Minute // Default: 30 minutes
	}

	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute) // Idle connections are closed after 10 minutes

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping property database %s: %w", dbName, err)
	}

	// Store connection with metadata
	now := time.Now()
	p.connections[dbName] = &propertyDBConnection{
		DB:           db,
		LastAccessed: now,
		CreatedAt:    now,
	}

	log.Printf("✓ Property database '%s' connected (MaxOpen: %d, MaxIdle: %d, MaxLifetime: %v)",
		dbName, maxOpenConns, maxIdleConns, connMaxLifetime)

	return db, nil
}

// GetPropertyDBBySubdomain gets property database by subdomain
func GetPropertyDBBySubdomain(subdomain string) (*gorm.DB, error) {
	// Query property_db_mappings table to get database name
	type PropertyDBMapping struct {
		PropertyID uint   `gorm:"column:property_id"`
		DBName     string `gorm:"column:db_name"`
		Subdomain  string `gorm:"column:subdomain"`
	}

	var mapping PropertyDBMapping
	masterDB := GetMasterDB()
	if masterDB == nil || masterDB.DB == nil {
		return nil, fmt.Errorf("master database not initialized")
	}

	err := masterDB.DB.Table("property_db_mappings").
		Where("subdomain = ?", subdomain).
		First(&mapping).Error

	if err != nil {
		return nil, fmt.Errorf("property not found for subdomain %s: %w", subdomain, err)
	}

	return GetPropertyDB(mapping.DBName)
}

// StopCleanupRoutine stops the cleanup goroutine
func StopCleanupRoutine() {
	if propertyPool != nil && propertyPool.cleanupCancel != nil {
		propertyPool.cleanupCancel()
		// Wait a bit for cleanup routine to stop
		timeout := time.After(2 * time.Second)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-timeout:
				log.Println("⚠ Cleanup routine stop timeout")
				return
			case <-ticker.C:
				if !propertyPool.cleanupRunning {
					return
				}
			}
		}
	}
}

// CloseAllPropertyDBs closes all property database connections
func CloseAllPropertyDBs() error {
	if propertyPool == nil {
		return nil
	}

	// Stop cleanup routine first
	StopCleanupRoutine()

	propertyPool.mu.Lock()
	defer propertyPool.mu.Unlock()

	var lastErr error
	for dbName, conn := range propertyPool.connections {
		if conn != nil && conn.DB != nil {
			sqlDB, err := conn.DB.DB()
			if err != nil {
				lastErr = err
				continue
			}
			if err := sqlDB.Close(); err != nil {
				lastErr = err
				log.Printf("Error closing property database %s: %v", dbName, err)
			} else {
				log.Printf("✓ Property database '%s' closed\n", dbName)
			}
		}
	}

	return lastErr
}

// GetAllPropertyDBNames returns all property database names
func GetAllPropertyDBNames() ([]string, error) {
	type PropertyDBMapping struct {
		DBName string `gorm:"column:db_name"`
	}

	masterDB := GetMasterDB()
	if masterDB == nil || masterDB.DB == nil {
		return nil, fmt.Errorf("master database not initialized")
	}

	var mappings []PropertyDBMapping
	err := masterDB.DB.Table("property_db_mappings").
		Select("DISTINCT db_name").
		Find(&mappings).Error

	if err != nil {
		return nil, err
	}

	dbNames := make([]string, len(mappings))
	for i, mapping := range mappings {
		dbNames[i] = mapping.DBName
	}

	return dbNames, nil
}

// GetPoolStats returns statistics about the connection pool
func GetPoolStats() map[string]interface{} {
	if propertyPool == nil {
		return map[string]interface{}{
			"initialized": false,
		}
	}

	propertyPool.mu.RLock()
	defer propertyPool.mu.RUnlock()

	stats := map[string]interface{}{
		"initialized":       true,
		"total_connections": len(propertyPool.connections),
		"cleanup_running":   propertyPool.cleanupRunning,
		"connections":       make([]map[string]interface{}, 0),
	}

	now := time.Now()
	for dbName, conn := range propertyPool.connections {
		connStats := map[string]interface{}{
			"db_name":       dbName,
			"created_at":    conn.CreatedAt,
			"last_accessed": conn.LastAccessed,
			"idle_duration": now.Sub(conn.LastAccessed).String(),
			"age":           now.Sub(conn.CreatedAt).String(),
		}

		// Get SQL DB stats
		if sqlDB, err := conn.DB.DB(); err == nil {
			dbStats := sqlDB.Stats()
			connStats["open_connections"] = dbStats.OpenConnections
			connStats["in_use"] = dbStats.InUse
			connStats["idle"] = dbStats.Idle
			connStats["wait_count"] = dbStats.WaitCount
			connStats["wait_duration"] = dbStats.WaitDuration.String()
		}

		stats["connections"] = append(stats["connections"].([]map[string]interface{}), connStats)
	}

	return stats
}
