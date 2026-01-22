package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"homexai/internal/config"
	"homexai/internal/models/master"
	"homexai/internal/models/property"
	"homexai/internal/tracer"
	"homexai/internal/utils"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	masterDB *gorm.DB
)

func main() {
	var err error

	// Command line flags
	dropFlag := flag.Bool("drop", false, "Drop all tables before migration (DANGEROUS!)")
	propertySubdomain := flag.String("property", "", "Migrate specific property database (subdomain)")
	masterOnly := flag.Bool("master_only", false, "Only migrate master database")
	flag.Parse()

	// Load configuration
	confFile := utils.APP_NAME + "-" + utils.GetRunEnv()
	if err = config.InitConfig(confFile); err != nil {
		log.Println("[ERROR] init config failed:", err)
		os.Exit(1)
	}
	tracer.LogInfo(tracer.ID_APP, "load %s.yaml success!", confFile)

	// Initialize master database
	if err := initMasterDB(); err != nil {
		log.Fatalf("Failed to connect master database: %v", err)
		return
	}

	fmt.Printf("HomeX Database Migration Tool %s\n", utils.VERSION)
	fmt.Println("=====================================")

	// Drop tables if requested
	if *dropFlag {
		fmt.Println("\n⚠️  WARNING: Dropping all tables!")
		if err := dropAllTables(); err != nil {
			log.Fatalf("Failed to drop tables: %v", err)
		}
		fmt.Println("✓ All tables dropped")
		return
	}

	// Migrate master database
	fmt.Println("\n📦 Migrating master database...")
	if err := migrateMasterDB(); err != nil {
		log.Fatalf("Failed to migrate master database: %v", err)
	}
	fmt.Println("✓ Master database migrated successfully")

	// Migrate property databases
	if !*masterOnly {
		fmt.Println("\n📦 Migrating property databases...", *propertySubdomain)
		if err := migratePropertyDBs(*propertySubdomain); err != nil {
			log.Fatalf("Failed to migrate property databases: %v", err)
		}
		fmt.Println("✓ Property databases migrated successfully")
	}

	fmt.Println("\n✅ Migration completed successfully!")
}

func initMasterDB() error {
	dsn := config.Yaml.GetMasterDSN()

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	var err error
	masterDB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	return err
}

func dropAllTables() error {

	tables := []string{
		"forum_ads",
		"notification_logs",
		"user_sessions",
		"system_audit_logs",
		"import_tasks",
		"system_configs",
		"translations",
		"role_permissions",
		"user_roles",
		"permissions",
		"roles",
		"property_db_mappings",
		"oauth_providers",
		"properties",
		"users",
	}

	for _, table := range tables {
		if err := masterDB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)).Error; err != nil {
			return fmt.Errorf("failed to drop table %s: %v", table, err)
		}
	}

	dbName := "homexai_property_demo"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Yaml.Database.User,
		config.Yaml.GetDatabasePassword(),
		config.Yaml.Database.Host,
		config.Yaml.Database.Port,
		dbName,
	)

	propDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Printf("    ⚠️  Failed to connect: %v\n", err)
		return err
	}

	tables = []string{
		// 先删除有外键引用的表
		"service_orders",         // 引用 service_listings 和 units
		"forum_votes",            // 引用 forum_posts
		"forum_view_logs",        // 引用 forum_posts
		"forum_replies",          // 引用 forum_posts
		"request_traces",         // 引用 requests
		"work_permits",           // 引用 requests
		"gate_passes",            // 引用 requests
		"vehicle_stickers",       // 引用 requests
		"pet_registrations",           // 引用 requests
		"move_ins",                    // 引用 requests
		"move_outs",                   // 引用 requests
		"household_staff_registrations", // 引用 requests
		"visitor_registrations",  // 访客登记表
		"documents",              // 通用文档表
		"payments",               // 引用 bills
		"bill_items",             // 引用 bills
		"bills",                  // 引用 units, users
		"requests",               // 引用 units, parking_slots  ← 必须在 parking_slots 之前
		"complaint_messages",     // 引用 complaints
		"complaints",             // 引用 units
		"visitors",               // 引用 units
		"tenants",                // 引用 units
		"facility_reservations",  // 引用 facilities 和 units  ← 必须在 facilities 和 units 之前
		"parking_assignments",    // 引用 parking_slots
		"spa_parking_slots",      // SPA-停车位关联
		"landlord_parking_slots", // 业主-停车位关联
		"spa_units",              // SPA-房源关联
		"landlords",              // 业主-房源关联
		// 最后删除基础表
		"service_listings",       // 服务市场-服务列表表
		"forum_posts", // 论坛帖子表
		"facilities",  // 公共设施表
		"parking_slots",
		"units",
		"announcements",
		"audit_logs",
		"property_settings", // 物业设置表
		"notifications",     // 用户通知表
	}

	for _, table := range tables {
		if err := propDB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)).Error; err != nil {
			return fmt.Errorf("failed to drop table %s: %v", table, err)
		}
	}

	return nil
}

func migrateMasterDB() error {
	// Auto migrate all master models
	return masterDB.AutoMigrate(
		&master.User{},
		&master.OAuthProvider{},
		&master.Property{},
		&master.PropertyDBMapping{},
		&master.Role{},
		&master.Permission{},
		&master.RolePermission{},
		&master.UserRole{},
		&master.Translation{},
		&master.SystemConfig{},
		&master.ImportTask{},
		&master.SystemAuditLog{},
		&master.UserSession{},
		&master.NotificationLog{},
		&master.ForumAd{},
	)
}

func migratePropertyDBs(subdomain string) error {

	dbName := "homexai_property_" + subdomain

	fmt.Printf("  Migrating property: %s (db: %s)...\n", subdomain, dbName)

	// Connect to property database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Yaml.Database.User,
		config.Yaml.GetDatabasePassword(),
		config.Yaml.Database.Host,
		config.Yaml.Database.Port,
		dbName,
	)

	propDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Printf("    ⚠️  Failed to connect: %v\n", err)
		return err
	}

	// Migrate property tables
	if err := propDB.AutoMigrate(
		&property.Unit{},
		&property.ParkingSlot{},
		&property.Facility{},           // 公共设施
		&property.FacilityReservation{}, // 公共设施预定
		&property.Landlord{},            // 业主-房源关联
		&property.LandlordParkingSlot{}, // 业主-停车位关联
		&property.SPAUnit{},             // SPA-房源关联（直接关联SPA用户）
		&property.SPAParkingSlot{},      // SPA-停车位关联（直接关联SPA用户）
		&property.Document{},            // 通用文档表
		&property.Tenant{},
		&property.ParkingAssignment{},
		&property.Bill{},
		&property.BillItem{}, // 账单项表
		&property.Payment{},
		&property.Request{},
		&property.RequestTrace{}, // 请求流程轨迹表
		&property.WorkPermit{},   // 工作许可证表
		&property.GatePass{},     // 门禁通行证表
		&property.VehicleSticker{}, // 车辆贴纸申请表
		&property.PetRegistration{},         // 宠物登记表
		&property.MoveIn{},                  // 入住申请表
		&property.MoveOut{},                 // 搬出申请表
		&property.HouseholdStaffRegistration{}, // 家政人员登记表
		&property.Complaint{},
		&property.ComplaintMessage{},  // 投诉消息表
		&property.Visitor{},
		&property.Announcement{},
		&property.AuditLog{},
		&property.ForumPost{},    // 论坛帖子表
		&property.ForumReply{},   // 论坛回复表
		&property.ForumViewLog{}, // 论坛浏览记录表
		&property.ForumVote{},    // 论坛投票记录表
		&property.ServiceListing{}, // 服务市场-服务列表表
		&property.ServiceOrder{},   // 服务市场-服务订单表
		&property.PropertySettings{}, // 物业设置表
		&property.Notification{},    // 用户通知表
	); err != nil {
		fmt.Printf("    ⚠️  Migration failed: %v\n", err)
		return err
	}

	fmt.Printf("    ✓ %s migrated\n", dbName)

	return nil
}
