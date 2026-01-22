package main

import (
	"fmt"
	"log"

	"homexai/internal/config"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	// Load configuration
	if err := config.InitConfig("homexai-local"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to master database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Yaml.Database.User,
		config.Yaml.Database.GetPassword(),
		config.Yaml.Database.Host,
		config.Yaml.Database.Port,
		"homexai",
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	fmt.Println("Altering salt column length from varchar(32) to varchar(4)...")

	// Alter salt column length
	err = db.Exec("ALTER TABLE users MODIFY COLUMN salt VARCHAR(4) NOT NULL").Error
	if err != nil {
		log.Fatalf("Failed to alter salt column: %v", err)
	}

	fmt.Println("✅ Successfully altered salt column length")
}