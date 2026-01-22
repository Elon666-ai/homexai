package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"homexai/internal/config"
	"homexai/internal/database"
	"homexai/internal/models/master"
	"homexai/internal/models/property"
	"homexai/internal/tracer"
	"homexai/internal/utils"

	"gorm.io/gorm"
)

// Helper functions for creating pointers
func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// dropUserRolesPropertyFK drops the FK constraint on user_roles.property_id if it exists
// This allows PropertyID=0 for system-level roles like super_admin
func dropUserRolesPropertyFK(db *gorm.DB) {
	log.Println("Checking FK constraints on user_roles...")

	// Check if constraint exists
	var count int64
	db.Raw(`
		SELECT COUNT(*) FROM information_schema.TABLE_CONSTRAINTS 
		WHERE CONSTRAINT_SCHEMA = DATABASE() 
		AND TABLE_NAME = 'user_roles' 
		AND CONSTRAINT_NAME = 'fk_properties_user_roles'
	`).Scan(&count)

	if count > 0 {
		log.Println("  - Dropping FK constraint fk_properties_user_roles...")
		if err := db.Exec("ALTER TABLE user_roles DROP FOREIGN KEY fk_properties_user_roles").Error; err != nil {
			log.Printf("  - Warning: Failed to drop FK constraint: %v", err)
		} else {
			log.Println("  ✓ FK constraint dropped")
		}
	} else {
		log.Println("  - FK constraint not found, skipping")
	}
}

func main() {
	var err error

	confFile := utils.APP_NAME + "-" + utils.GetRunEnv() + ".yaml"
	if err = config.InitConfig(confFile); err != nil {
		log.Println("[ERROR] init config failed:", err)
		os.Exit(1)
	}
	tracer.LogInfo(tracer.ID_APP, "load %s.yaml success!", confFile)

	// Initialize database
	if err := database.InitMasterDB(); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	db := database.GetMasterGormDB()

	// Drop FK constraint on user_roles.property_id if exists (allows PropertyID=0 for super_admin)
	dropUserRolesPropertyFK(db)

	// 1. Seed roles
	seedRoles(db)

	// 2. Seed permissions
	seedPermissions(db)
	seedRolePermissions(db)

	// 3. Seed translations
	seedTranslations(db)

	// 4. Seed default super admin
	superAdminID := seedSuperAdmin(db)

	// 5. Seed demo property
	demoPropertyID := seedDemoProperty(db)

	// 5.5 Seed forum ads (B2B system, stored in master DB)
	seedForumAds(db, superAdminID, demoPropertyID)

	// 6. Seed property admin user
	propertyAdminID := seedPropertyAdmin(db, demoPropertyID)

	// 7. Seed property staff user
	propertyStaffID := seedPropertyStaff(db, demoPropertyID)

	// 8. Seed property accountant user
	propertyAccountantID := seedPropertyAccountant(db, demoPropertyID)

	// 9. Seed property guard user
	propertyGuardID := seedPropertyGuard(db, demoPropertyID)

	// 10. Seed landlord users
	landlordIDs := seedLandlordUsers(db, demoPropertyID)

	// 11. Seed tenant users
	tenantIDs := seedTenantUsers(db, demoPropertyID)

	// 12. Seed SPA users (独立的SPA代理人用户)
	spaIDs := seedSPAUsers(db, demoPropertyID)

	// ==================== Property Database ====================

	// Initialize property database pool
	database.InitPropertyDBPool()

	// Get demo property database connection
	propDB, err := database.GetPropertyDB("homexai_property_demo")
	if err != nil {
		log.Printf("Warning: Could not connect to property database: %v", err)
		log.Println("Skipping property database seeding. Please create the database first.")
	} else {
		// Seed property data
		unitIDs := seedUnits(propDB)
		parkingSlotIDs := seedParkingSlots(propDB, landlordIDs, propertyAdminID) // 需要传入业主ID设置OwnerID
		seedLandlords(propDB, landlordIDs, unitIDs)
		seedLandlordParkingSlots(propDB, landlordIDs, parkingSlotIDs) // 业主拥有的停车位
		seedSPAs(propDB, spaIDs, unitIDs, parkingSlotIDs)             // SPA代理人及其代理的房源/停车位
		seedTenants(propDB, tenantIDs, unitIDs)
		seedParkingAssignments(propDB, parkingSlotIDs, unitIDs, landlordIDs, tenantIDs)
		seedAnnouncements(propDB, propertyAdminID)
		billIDs := seedBills(propDB, unitIDs, parkingSlotIDs, tenantIDs, landlordIDs)
		seedPayments(propDB, billIDs, tenantIDs)
		seedRequests(propDB, unitIDs, tenantIDs, landlordIDs, propertyStaffID) // Use staff for request handling
		seedVisitors(propDB, unitIDs, tenantIDs, landlordIDs, propertyGuardID) // 访客记录 (use guard)
		seedComplaints(propDB, unitIDs, tenantIDs)                             // 投诉记录
		seedComplaintMessages(propDB, tenantIDs, propertyStaffID)              // 投诉消息
		seedDocuments(propDB, billIDs, tenantIDs)                              // 文档（如支付凭证）
		seedForumPosts(propDB, tenantIDs, landlordIDs, propertyAdminID)        // 论坛帖子
		seedFacilitiesAndReservations(propDB, tenantIDs, landlordIDs, propertyStaffID)
		seedMarketplace(propDB, unitIDs, tenantIDs, landlordIDs, propertyAdminID, propertyStaffID) // 服务市场
		seedNotifications(propDB, demoPropertyID, tenantIDs, landlordIDs, propertyAdminID)         // 通知数据
	}

	log.Println("========================================")
	log.Println("Database seeding completed successfully!")
	log.Println("========================================")
	log.Println("")
	log.Println("Default Login Credentials:")
	log.Println("------------------------------------------")
	log.Printf("Super Admin: super_admin@homex.ph / Admin1234!  (ID: %d)\n", superAdminID)
	log.Printf("Property Admin: admin@demo.homex.ph / Admin1234!  (ID: %d)\n", propertyAdminID)
	log.Printf("Property Staff: staff@demo.homex.ph / Admin1234!  (ID: %d)\n", propertyStaffID)
	log.Printf("Property Accountant: accountant@demo.homex.ph / Admin1234!  (ID: %d)\n", propertyAccountantID)
	log.Printf("Property Guard: guard@demo.homex.ph / Admin1234!  (ID: %d)\n", propertyGuardID)
	log.Println("Landlords: landlord1~5@demo.homex.ph / Home1234!")
	log.Println("Tenants: tenant1~6@demo.homex.ph / Home1234!")
	log.Println("SPAs: spa1~3@demo.homex.ph / Home1234!")
	log.Println("------------------------------------------")
	log.Println("IMPORTANT: Please change these passwords in production!")
}

// ==================== Role Seeding ====================

func seedRoles(db *gorm.DB) {
	log.Println("Seeding roles...")

	roles := []master.Role{
		{Code: "super_admin", NameEN: "Super Admin", NameZhCN: strPtr("超级管理员"), NameZhTW: strPtr("超級管理員"), NameTL: strPtr("Super Admin"), IsSystem: true, IsCrossProperty: true},
		{Code: "property_admin", NameEN: "Property Admin", NameZhCN: strPtr("物业管理员"), NameZhTW: strPtr("物業管理員"), NameTL: strPtr("Property Admin"), IsSystem: true, IsCrossProperty: false},
		{Code: "property_account", NameEN: "Property Accountant", NameZhCN: strPtr("物业会计"), NameZhTW: strPtr("物業會計"), NameTL: strPtr("Property Accountant"), IsSystem: true, IsCrossProperty: false},
		{Code: "property_staff", NameEN: "Property Staff", NameZhCN: strPtr("物业员工"), NameZhTW: strPtr("物業員工"), NameTL: strPtr("Property Staff"), IsSystem: true, IsCrossProperty: false},
		{Code: "landlord", NameEN: "Landlord", NameZhCN: strPtr("业主"), NameZhTW: strPtr("業主"), NameTL: strPtr("May-ari"), IsSystem: true, IsCrossProperty: false},
		{Code: "spa", NameEN: "SPA Representative", NameZhCN: strPtr("SPA代理人"), NameZhTW: strPtr("SPA代理人"), NameTL: strPtr("Kinatawan ng SPA"), IsSystem: true, IsCrossProperty: false},
		{Code: "tenant", NameEN: "Tenant", NameZhCN: strPtr("租客"), NameZhTW: strPtr("租客"), NameTL: strPtr("Nangungupahan"), IsSystem: true, IsCrossProperty: false},
		{Code: "property_guard", NameEN: "Property Guard", NameZhCN: strPtr("物业保安"), NameZhTW: strPtr("物業保安"), NameTL: strPtr("Guard ng Property"), IsSystem: true, IsCrossProperty: false},
	}

	for _, role := range roles {
		if err := db.Create(&role).Error; err != nil {
			log.Printf("  ✗ Failed to create role %s: %v", role.Code, err)
		} else {
			log.Printf("  ✓ Created role: %s", role.Code)
		}
	}
}

// ==================== Permission Seeding ====================

func seedPermissions(db *gorm.DB) {
	log.Println("Seeding permissions...")

	permissions := []master.Permission{
		// Unit permissions
		{Code: "unit.create", NameEN: "Create Unit", NameZhCN: strPtr("创建房源"), Resource: "unit", Action: "create"},
		{Code: "unit.read", NameEN: "View Unit", NameZhCN: strPtr("查看房源"), Resource: "unit", Action: "read"},
		{Code: "unit.update", NameEN: "Update Unit", NameZhCN: strPtr("更新房源"), Resource: "unit", Action: "update"},
		{Code: "unit.delete", NameEN: "Delete Unit", NameZhCN: strPtr("删除房源"), Resource: "unit", Action: "delete"},
		// Parking permissions
		{Code: "parking.create", NameEN: "Create Parking", NameZhCN: strPtr("创建车位"), Resource: "parking", Action: "create"},
		{Code: "parking.read", NameEN: "View Parking", NameZhCN: strPtr("查看车位"), Resource: "parking", Action: "read"},
		{Code: "parking.update", NameEN: "Update Parking", NameZhCN: strPtr("更新车位"), Resource: "parking", Action: "update"},
		{Code: "parking.delete", NameEN: "Delete Parking", NameZhCN: strPtr("删除车位"), Resource: "parking", Action: "delete"},
		{Code: "parking.assign", NameEN: "Assign Parking", NameZhCN: strPtr("分配车位"), Resource: "parking", Action: "assign"},
		// Bill permissions
		{Code: "bill.create", NameEN: "Create Bill", NameZhCN: strPtr("创建账单"), Resource: "bill", Action: "create"},
		{Code: "bill.read", NameEN: "View Bill", NameZhCN: strPtr("查看账单"), Resource: "bill", Action: "read"},
		{Code: "bill.update", NameEN: "Update Bill", NameZhCN: strPtr("更新账单"), Resource: "bill", Action: "update"},
		{Code: "bill.delete", NameEN: "Delete Bill", NameZhCN: strPtr("删除账单"), Resource: "bill", Action: "delete"},
		// Payment permissions
		{Code: "payment.create", NameEN: "Create Payment", NameZhCN: strPtr("创建支付"), Resource: "payment", Action: "create"},
		{Code: "payment.read", NameEN: "View Payment", NameZhCN: strPtr("查看支付"), Resource: "payment", Action: "read"},
		{Code: "payment.approve", NameEN: "Approve Payment", NameZhCN: strPtr("审批支付"), Resource: "payment", Action: "approve"},
		// Request permissions
		{Code: "request.create", NameEN: "Create Request", NameZhCN: strPtr("创建请求"), Resource: "request", Action: "create"},
		{Code: "request.read", NameEN: "View Request", NameZhCN: strPtr("查看请求"), Resource: "request", Action: "read"},
		{Code: "request.update", NameEN: "Update Request", NameZhCN: strPtr("更新请求"), Resource: "request", Action: "update"},
		{Code: "request.approve", NameEN: "Approve Request", NameZhCN: strPtr("审批请求"), Resource: "request", Action: "approve"},
		// Visitor permissions
		{Code: "visitor.create", NameEN: "Create Visitor", NameZhCN: strPtr("创建访客"), Resource: "visitor", Action: "create"},
		{Code: "visitor.read", NameEN: "View Visitor", NameZhCN: strPtr("查看访客"), Resource: "visitor", Action: "read"},
		{Code: "visitor.checkin", NameEN: "Check-in Visitor", NameZhCN: strPtr("访客登记"), Resource: "visitor", Action: "checkin"},
		// Announcement permissions
		{Code: "announcement.create", NameEN: "Create Announcement", NameZhCN: strPtr("创建公告"), Resource: "announcement", Action: "create"},
		{Code: "announcement.read", NameEN: "View Announcement", NameZhCN: strPtr("查看公告"), Resource: "announcement", Action: "read"},
		{Code: "announcement.update", NameEN: "Update Announcement", NameZhCN: strPtr("更新公告"), Resource: "announcement", Action: "update"},
		// User permissions
		{Code: "user.create", NameEN: "Create User", NameZhCN: strPtr("创建用户"), Resource: "user", Action: "create"},
		{Code: "user.read", NameEN: "View User", NameZhCN: strPtr("查看用户"), Resource: "user", Action: "read"},
		{Code: "user.update", NameEN: "Update User", NameZhCN: strPtr("更新用户"), Resource: "user", Action: "update"},
		{Code: "user.assign", NameEN: "Assign User Role", NameZhCN: strPtr("分配用户角色"), Resource: "user", Action: "assign"},
		// Property permissions
		{Code: "property.create", NameEN: "Create Property", NameZhCN: strPtr("创建物业"), Resource: "property", Action: "create"},
		{Code: "property.read", NameEN: "View Property", NameZhCN: strPtr("查看物业"), Resource: "property", Action: "read"},
		{Code: "property.update", NameEN: "Update Property", NameZhCN: strPtr("更新物业"), Resource: "property", Action: "update"},
		{Code: "property.delete", NameEN: "Delete Property", NameZhCN: strPtr("删除物业"), Resource: "property", Action: "delete"},
		// Report permissions
		{Code: "report.view", NameEN: "View Reports", NameZhCN: strPtr("查看报表"), Resource: "report", Action: "view"},
		{Code: "report.export", NameEN: "Export Reports", NameZhCN: strPtr("导出报表"), Resource: "report", Action: "export"},
		// Forum permissions
		{Code: "forum.read", NameEN: "View Forum", NameZhCN: strPtr("查看论坛"), Resource: "forum", Action: "read"},
		{Code: "forum.create", NameEN: "Create Forum Post", NameZhCN: strPtr("创建论坛帖子"), Resource: "forum", Action: "create"},
	}

	for _, perm := range permissions {
		if err := db.Create(&perm).Error; err != nil {
			log.Printf("  ✗ Failed to create permission %s: %v", perm.Code, err)
		} else {
			log.Printf("  ✓ Created permission: %s", perm.Code)
		}
	}
}

func seedRolePermissions(db *gorm.DB) {
	log.Println("Seeding role-permission mappings...")

	rolePermissions := map[string][]string{
		"super_admin": {
			"unit.create", "unit.read", "unit.update", "unit.delete",
			"parking.create", "parking.read", "parking.update", "parking.delete", "parking.assign",
			"bill.create", "bill.read", "bill.update", "bill.delete",
			"payment.create", "payment.read", "payment.approve",
			"request.create", "request.read", "request.update", "request.approve",
			"visitor.create", "visitor.read", "visitor.checkin",
			"announcement.create", "announcement.read", "announcement.update",
			"user.create", "user.read", "user.update", "user.assign",
			"property.create", "property.read", "property.update", "property.delete",
			"report.view", "report.export",
		},
		"property_admin": {
			"unit.create", "unit.read", "unit.update", "unit.delete",
			"parking.create", "parking.read", "parking.update", "parking.delete", "parking.assign",
			"bill.create", "bill.read", "bill.update", "bill.delete",
			"payment.create", "payment.read", "payment.approve",
			"request.create", "request.read", "request.update", "request.approve",
			"visitor.create", "visitor.read", "visitor.checkin",
			"announcement.create", "announcement.read", "announcement.update",
			"user.create", "user.read", "user.update", "user.assign",
			"property.read", "property.update",
			"report.view", "report.export",
		},
		"property_account": {
			"unit.read",
			"parking.read",
			"bill.create", "bill.read", "bill.update",
			"payment.create", "payment.read", "payment.approve",
			"request.read",
			"announcement.read",
			"user.read",
			"report.view", "report.export",
		},
		"property_staff": {
			"unit.read",
			"parking.read",
			"request.read", "request.update",
			"visitor.create", "visitor.read", "visitor.checkin",
			"announcement.read",
		},
		"property_guard": {
			"request.read",
			"announcement.read",
			"forum.read", "forum.create",
			"visitor.create", "visitor.read", "visitor.checkin",
		},
		"landlord": {
			"unit.read",
			"parking.read",
			"bill.read",
			"payment.read",
			"request.create", "request.read",
			"visitor.create", "visitor.read",
			"announcement.read",
		},
		"spa": {
			"unit.read",
			"parking.read",
			"bill.read",
			"payment.read",
			"request.create", "request.read",
			"visitor.create", "visitor.read",
			"announcement.read",
		},
		"tenant": {
			"unit.read",
			"parking.read",
			"bill.read",
			"payment.create", "payment.read",
			"request.create", "request.read",
			"visitor.create", "visitor.read",
			"announcement.read",
		},
	}

	for roleName, permCodes := range rolePermissions {
		var role master.Role
		if err := db.Where("code = ?", roleName).First(&role).Error; err != nil {
			log.Printf("  - Role not found: %s", roleName)
			continue
		}

		for _, permCode := range permCodes {
			var perm master.Permission
			if err := db.Where("code = ?", permCode).First(&perm).Error; err != nil {
				log.Printf("  - Permission not found: %s", permCode)
				continue
			}
			rp := master.RolePermission{
				RoleID:       role.ID,
				PermissionID: perm.ID,
			}
			db.Where("role_id = ? AND permission_id = ?", role.ID, perm.ID).FirstOrCreate(&rp)
		}
	}

	log.Println("  ✓ Role-permission mappings seeded")
}

// ==================== Translation Seeding ====================

func seedTranslations(db *gorm.DB) {
	log.Println("Seeding translations...")

	translations := []master.Translation{
		// Common translations
		{Language: "en", TranslationKey: "welcome", TranslationValue: "Welcome"},
		{Language: "zh-CN", TranslationKey: "welcome", TranslationValue: "欢迎"},
		{Language: "zh-TW", TranslationKey: "welcome", TranslationValue: "歡迎"},
		{Language: "tl", TranslationKey: "welcome", TranslationValue: "Maligayang pagdating"},

		// Button translations
		{Language: "en", TranslationKey: "button.login", TranslationValue: "Login"},
		{Language: "zh-CN", TranslationKey: "button.login", TranslationValue: "登录"},
		{Language: "zh-TW", TranslationKey: "button.login", TranslationValue: "登錄"},
		{Language: "tl", TranslationKey: "button.login", TranslationValue: "Mag-login"},

		{Language: "en", TranslationKey: "button.register", TranslationValue: "Register"},
		{Language: "zh-CN", TranslationKey: "button.register", TranslationValue: "注册"},
		{Language: "zh-TW", TranslationKey: "button.register", TranslationValue: "註冊"},
		{Language: "tl", TranslationKey: "button.register", TranslationValue: "Magrehistro"},

		{Language: "en", TranslationKey: "button.submit", TranslationValue: "Submit"},
		{Language: "zh-CN", TranslationKey: "button.submit", TranslationValue: "提交"},
		{Language: "zh-TW", TranslationKey: "button.submit", TranslationValue: "提交"},
		{Language: "tl", TranslationKey: "button.submit", TranslationValue: "Isumite"},

		{Language: "en", TranslationKey: "button.cancel", TranslationValue: "Cancel"},
		{Language: "zh-CN", TranslationKey: "button.cancel", TranslationValue: "取消"},
		{Language: "zh-TW", TranslationKey: "button.cancel", TranslationValue: "取消"},
		{Language: "tl", TranslationKey: "button.cancel", TranslationValue: "Kanselahin"},

		{Language: "en", TranslationKey: "button.save", TranslationValue: "Save"},
		{Language: "zh-CN", TranslationKey: "button.save", TranslationValue: "保存"},
		{Language: "zh-TW", TranslationKey: "button.save", TranslationValue: "保存"},
		{Language: "tl", TranslationKey: "button.save", TranslationValue: "I-save"},

		// Error messages
		{Language: "en", TranslationKey: "error.invalid_credentials", TranslationValue: "Invalid credentials"},
		{Language: "zh-CN", TranslationKey: "error.invalid_credentials", TranslationValue: "凭据无效"},
		{Language: "zh-TW", TranslationKey: "error.invalid_credentials", TranslationValue: "憑據無效"},
		{Language: "tl", TranslationKey: "error.invalid_credentials", TranslationValue: "Hindi wastong kredensyal"},

		{Language: "en", TranslationKey: "error.email_required", TranslationValue: "Email is required"},
		{Language: "zh-CN", TranslationKey: "error.email_required", TranslationValue: "邮箱不能为空"},
		{Language: "zh-TW", TranslationKey: "error.email_required", TranslationValue: "郵箱不能為空"},
		{Language: "tl", TranslationKey: "error.email_required", TranslationValue: "Kinakailangan ang email"},

		{Language: "en", TranslationKey: "error.password_required", TranslationValue: "Password is required"},
		{Language: "zh-CN", TranslationKey: "error.password_required", TranslationValue: "密码不能为空"},
		{Language: "zh-TW", TranslationKey: "error.password_required", TranslationValue: "密碼不能為空"},
		{Language: "tl", TranslationKey: "error.password_required", TranslationValue: "Kinakailangan ang password"},

		// Success messages
		{Language: "en", TranslationKey: "success.login", TranslationValue: "Login successful"},
		{Language: "zh-CN", TranslationKey: "success.login", TranslationValue: "登录成功"},
		{Language: "zh-TW", TranslationKey: "success.login", TranslationValue: "登錄成功"},
		{Language: "tl", TranslationKey: "success.login", TranslationValue: "Matagumpay na naka-login"},

		{Language: "en", TranslationKey: "success.register", TranslationValue: "Registration successful"},
		{Language: "zh-CN", TranslationKey: "success.register", TranslationValue: "注册成功"},
		{Language: "zh-TW", TranslationKey: "success.register", TranslationValue: "註冊成功"},
		{Language: "tl", TranslationKey: "success.register", TranslationValue: "Matagumpay na nagrehistro"},

		// Menu translations
		{Language: "en", TranslationKey: "menu.dashboard", TranslationValue: "Dashboard"},
		{Language: "zh-CN", TranslationKey: "menu.dashboard", TranslationValue: "仪表盘"},
		{Language: "zh-TW", TranslationKey: "menu.dashboard", TranslationValue: "儀表盤"},
		{Language: "tl", TranslationKey: "menu.dashboard", TranslationValue: "Dashboard"},

		{Language: "en", TranslationKey: "menu.units", TranslationValue: "Units"},
		{Language: "zh-CN", TranslationKey: "menu.units", TranslationValue: "房源"},
		{Language: "zh-TW", TranslationKey: "menu.units", TranslationValue: "房源"},
		{Language: "tl", TranslationKey: "menu.units", TranslationValue: "Mga Unit"},

		{Language: "en", TranslationKey: "menu.parking", TranslationValue: "Parking"},
		{Language: "zh-CN", TranslationKey: "menu.parking", TranslationValue: "停车位"},
		{Language: "zh-TW", TranslationKey: "menu.parking", TranslationValue: "停車位"},
		{Language: "tl", TranslationKey: "menu.parking", TranslationValue: "Paradahan"},

		{Language: "en", TranslationKey: "menu.bills", TranslationValue: "Bills"},
		{Language: "zh-CN", TranslationKey: "menu.bills", TranslationValue: "账单"},
		{Language: "zh-TW", TranslationKey: "menu.bills", TranslationValue: "賬單"},
		{Language: "tl", TranslationKey: "menu.bills", TranslationValue: "Mga Singil"},

		{Language: "en", TranslationKey: "menu.requests", TranslationValue: "Requests"},
		{Language: "zh-CN", TranslationKey: "menu.requests", TranslationValue: "请求"},
		{Language: "zh-TW", TranslationKey: "menu.requests", TranslationValue: "請求"},
		{Language: "tl", TranslationKey: "menu.requests", TranslationValue: "Mga Kahilingan"},

		{Language: "en", TranslationKey: "menu.visitors", TranslationValue: "Visitors"},
		{Language: "zh-CN", TranslationKey: "menu.visitors", TranslationValue: "访客"},
		{Language: "zh-TW", TranslationKey: "menu.visitors", TranslationValue: "訪客"},
		{Language: "tl", TranslationKey: "menu.visitors", TranslationValue: "Mga Bisita"},

		{Language: "en", TranslationKey: "menu.announcements", TranslationValue: "Announcements"},
		{Language: "zh-CN", TranslationKey: "menu.announcements", TranslationValue: "公告"},
		{Language: "zh-TW", TranslationKey: "menu.announcements", TranslationValue: "公告"},
		{Language: "tl", TranslationKey: "menu.announcements", TranslationValue: "Mga Anunsyo"},
	}

	for _, trans := range translations {
		if err := db.Create(&trans).Error; err != nil {
			log.Printf("  ✗ Failed to create translation %s-%s: %v", trans.Language, trans.TranslationKey, err)
		}
	}

	log.Println("  ✓ Translations seeded")
}

// ==================== User Seeding ====================

func seedSuperAdmin(db *gorm.DB) uint {
	log.Println("Seeding super admin user...")

	email := "super_admin@homex.ph"
	passwordHash, salt := utils.CreatePasswordHash(utils.DEFAULT_ADMIN_PASSWORD)

	admin := master.User{
		Email:         &email,
		PasswordHash:  passwordHash,
		Salt:          salt,
		FullName:      "System Administrator",
		EmailVerified: true,
		Status:        "active",
	}

	if err := db.Create(&admin).Error; err != nil {
		log.Fatalf("Failed to create super admin: %v", err)
	}

	log.Printf("  ✓ Created super admin: %s (ID: %d)", email, admin.ID)

	// Assign super_admin role
	ensureUserRole(db, admin.ID, 0, "super_admin")

	return admin.ID
}

func seedPropertyAdmin(db *gorm.DB, propertyID uint) uint {
	log.Println("Seeding property admin user...")

	email := "admin@demo.homex.ph"
	passwordHash, salt := utils.CreatePasswordHash(utils.DEFAULT_ADMIN_PASSWORD)

	admin := master.User{
		Email:         &email,
		PasswordHash:  passwordHash,
		Salt:          salt,
		FullName:      "Demo Property Admin",
		Phone:         strPtr("+63 912 345 6789"),
		EmailVerified: true,
		Status:        "active",
	}

	if err := db.Create(&admin).Error; err != nil {
		log.Fatalf("Failed to create property admin: %v", err)
	}

	log.Printf("  ✓ Created property admin: %s (ID: %d)", email, admin.ID)

	// Assign property_admin role
	ensureUserRole(db, admin.ID, propertyID, "property_admin")

	return admin.ID
}

func seedPropertyStaff(db *gorm.DB, propertyID uint) uint {
	log.Println("Seeding property staff user...")

	email := "staff@demo.homex.ph"
	passwordHash, salt := utils.CreatePasswordHash(utils.DEFAULT_ADMIN_PASSWORD)

	staff := master.User{
		Email:         &email,
		PasswordHash:  passwordHash,
		Salt:          salt,
		FullName:      "Demo Property Staff",
		Phone:         strPtr("+63 912 345 6790"),
		EmailVerified: true,
		Status:        "active",
	}

	if err := db.Create(&staff).Error; err != nil {
		log.Fatalf("Failed to create property staff: %v", err)
	}

	log.Printf("  ✓ Created property staff: %s (ID: %d)", email, staff.ID)

	// Assign property_staff role
	ensureUserRole(db, staff.ID, propertyID, "property_staff")

	return staff.ID
}

func seedPropertyAccountant(db *gorm.DB, propertyID uint) uint {
	log.Println("Seeding property accountant user...")

	email := "accountant@demo.homex.ph"
	passwordHash, salt := utils.CreatePasswordHash(utils.DEFAULT_ADMIN_PASSWORD)

	accountant := master.User{
		Email:         &email,
		PasswordHash:  passwordHash,
		Salt:          salt,
		FullName:      "Demo Property Accountant",
		Phone:         strPtr("+63 912 345 6791"),
		EmailVerified: true,
		Status:        "active",
	}

	if err := db.Create(&accountant).Error; err != nil {
		log.Fatalf("Failed to create property accountant: %v", err)
	}

	log.Printf("  ✓ Created property accountant: %s (ID: %d)", email, accountant.ID)

	// Assign property_account role
	ensureUserRole(db, accountant.ID, propertyID, "property_account")

	return accountant.ID
}

func seedPropertyGuard(db *gorm.DB, propertyID uint) uint {
	log.Println("Seeding property guard user...")

	email := "guard@demo.homex.ph"
	passwordHash, salt := utils.CreatePasswordHash(utils.DEFAULT_ADMIN_PASSWORD)

	guard := master.User{
		Email:         &email,
		PasswordHash:  passwordHash,
		Salt:          salt,
		FullName:      "Demo Property Guard",
		Phone:         strPtr("+63 912 345 6792"),
		EmailVerified: true,
		Status:        "active",
	}

	if err := db.Create(&guard).Error; err != nil {
		log.Fatalf("Failed to create property guard: %v", err)
	}

	log.Printf("  ✓ Created property guard: %s (ID: %d)", email, guard.ID)

	// Assign property_guard role
	ensureUserRole(db, guard.ID, propertyID, "property_guard")

	return guard.ID
}

func seedLandlordUsers(db *gorm.DB, propertyID uint) []uint {
	log.Println("Seeding landlord users...")

	landlords := []struct {
		Email    string
		FullName string
		Phone    string
	}{
		{"landlord1@demo.homex.ph", "John Smith", "+63 917 111 1111"},
		{"landlord2@demo.homex.ph", "Maria Garcia", "+63 917 222 2222"},
		{"landlord3@demo.homex.ph", "David Chen", "+63 917 333 3333"},
		{"landlord4@demo.homex.ph", "Sarah Johnson", "+63 917 444 4444"},
		{"landlord5@demo.homex.ph", "Michael Wong", "+63 917 555 5555"},
	}

	var landlordIDs []uint

	for _, l := range landlords {
		passwordHash, salt := utils.CreatePasswordHash(utils.DEFAULT_USER_PASSWORD)
		user := master.User{
			Email:         strPtr(l.Email),
			PasswordHash:  passwordHash,
			Salt:          salt,
			FullName:      l.FullName,
			Phone:         strPtr(l.Phone),
			EmailVerified: true,
			Status:        "active",
		}

		if err := db.Create(&user).Error; err != nil {
			log.Printf("  ✗ Failed to create landlord %s: %v", l.Email, err)
			continue
		}

		log.Printf("  ✓ Created landlord: %s (ID: %d)", l.Email, user.ID)
		ensureUserRole(db, user.ID, propertyID, "landlord")
		landlordIDs = append(landlordIDs, user.ID)
	}

	return landlordIDs
}

func seedTenantUsers(db *gorm.DB, propertyID uint) []uint {
	log.Println("Seeding tenant users...")

	tenants := []struct {
		Email    string
		FullName string
		Phone    string
	}{
		{"tenant1@demo.homex.ph", "Alice Brown", "+63 918 111 1111"},
		{"tenant2@demo.homex.ph", "Bob Wilson", "+63 918 222 2222"},
		{"tenant3@demo.homex.ph", "Carol Davis", "+63 918 333 3333"},
		{"tenant4@demo.homex.ph", "Daniel Lee", "+63 918 444 4444"},
		{"tenant5@demo.homex.ph", "Eva Martinez", "+63 918 555 5555"},
		{"tenant6@demo.homex.ph", "Frank Taylor", "+63 918 666 6666"},
	}

	var tenantIDs []uint

	for _, t := range tenants {
		passwordHash, salt := utils.CreatePasswordHash(utils.DEFAULT_USER_PASSWORD)
		user := master.User{
			Email:         strPtr(t.Email),
			PasswordHash:  passwordHash,
			Salt:          salt,
			FullName:      t.FullName,
			Phone:         strPtr(t.Phone),
			EmailVerified: true,
			Status:        "active",
		}

		if err := db.Create(&user).Error; err != nil {
			log.Printf("  ✗ Failed to create tenant %s: %v", t.Email, err)
			continue
		}

		log.Printf("  ✓ Created tenant: %s (ID: %d)", t.Email, user.ID)
		ensureUserRole(db, user.ID, propertyID, "tenant")
		tenantIDs = append(tenantIDs, user.ID)
	}

	return tenantIDs
}

func seedSPAUsers(db *gorm.DB, propertyID uint) []uint {
	log.Println("Seeding SPA (Special Power of Attorney) users...")

	spas := []struct {
		Email    string
		FullName string
		Phone    string
	}{
		{"spa1@demo.homex.ph", "Angela Reyes", "+63 919 111 1111"},
		{"spa2@demo.homex.ph", "Benjamin Cruz", "+63 919 222 2222"},
		{"spa3@demo.homex.ph", "Catherine Lim", "+63 919 333 3333"},
	}

	var spaIDs []uint

	for _, s := range spas {
		passwordHash, salt := utils.CreatePasswordHash(utils.DEFAULT_USER_PASSWORD)
		user := master.User{
			Email:         strPtr(s.Email),
			PasswordHash:  passwordHash,
			Salt:          salt,
			FullName:      s.FullName,
			Phone:         strPtr(s.Phone),
			EmailVerified: true,
			Status:        "active",
		}

		if err := db.Create(&user).Error; err != nil {
			log.Printf("  ✗ Failed to create SPA user %s: %v", s.Email, err)
			continue
		}

		log.Printf("  ✓ Created SPA user: %s (ID: %d)", s.Email, user.ID)
		ensureUserRole(db, user.ID, propertyID, "spa")
		spaIDs = append(spaIDs, user.ID)
	}

	return spaIDs
}

func ensureUserRole(db *gorm.DB, userID, propertyID uint, roleCode string) {
	now := time.Now()
	upr := master.UserRole{
		UserID:     userID,
		PropertyID: propertyID,
		Role:       roleCode,
		Status:     "active",
		AssignedAt: &now,
	}
	db.Where("user_id = ? AND property_id = ? AND role = ?", userID, propertyID, roleCode).FirstOrCreate(&upr)
}

// ==================== Property Seeding ====================

func seedDemoProperty(db *gorm.DB) uint {
	log.Println("Seeding demo property...")

	subdomain := "demo"
	var existing master.Property
	if err := db.Where("subdomain = ?", subdomain).First(&existing).Error; err == nil {
		log.Printf("  - Property already exists: %s (ID: %d)", subdomain, existing.ID)

		// Ensure DB mapping exists
		ensurePropertyDBMapping(db, existing.ID, subdomain)
		return existing.ID
	}

	// Set handover date to 2024-01-01 for demo property
	handoverDate, _ := time.Parse("2006-01-02", "2024-01-01")

	prop := master.Property{
		Name:                      "Demo Residence",
		Subdomain:                 subdomain,
		DBName:                    "homexai_property_demo",
		Status:                    "active",
		Category:                  "residential",
		Province:                  "Metro Manila",
		City:                      strPtr("Makati City"),
		Address:                   strPtr("123 Demo Street, Legazpi Village"),
		Developer:                 "Demo Development Corp",
		HandoverDate:              handoverDate,
		PropertyManagementCompany: strPtr("Demo Property Management"),
		Country:                   "Philippines",
		PostalCode:                strPtr("1229"),
		ContactEmail:              strPtr("admin@demo.homex.ph"),
		ContactPhone:              strPtr("+63 2 1234 5678"),
		Website:                   strPtr("https://demo.homex.ph"),
		Description:               strPtr("A premier residential property in the heart of Makati, featuring modern amenities and 24/7 security."),
		Timezone:                  "Asia/Manila",
		DefaultLanguage:           "en",
	}

	if err := db.Create(&prop).Error; err != nil {
		log.Fatalf("Failed to create demo property: %v", err)
	}

	log.Printf("  ✓ Created property: %s (ID: %d)", subdomain, prop.ID)

	// Create DB mapping
	ensurePropertyDBMapping(db, prop.ID, subdomain)

	return prop.ID
}

func ensurePropertyDBMapping(db *gorm.DB, propertyID uint, subdomain string) {
	mapping := master.PropertyDBMapping{
		PropertyID:          propertyID,
		Subdomain:           subdomain,
		DBHost:              config.Yaml.Database.Host,
		DBPort:              config.Yaml.Database.Port,
		DBName:              "homexai_property_" + subdomain,
		DBUser:              config.Yaml.Database.User,
		DBPasswordEncrypted: config.Yaml.Database.Password, // In production, encrypt this
		IsActive:            true,
		MaxConnections:      50,
	}

	if err := db.Create(&mapping).Error; err != nil {
		log.Printf("  ✗ Failed to create DB mapping: %v", err)
	} else {
		log.Printf("  ✓ Created DB mapping for: %s", subdomain)
	}
}

// ==================== Forum Ads (Master DB) ====================

func seedForumAds(db *gorm.DB, superAdminID, propertyID uint) {
	log.Println("Seeding forum ads (B2B system)...")

	now := time.Now()

	ads := []master.ForumAd{
		{
			AdType:           master.ForumAdTypePinned,
			Title:            "🔥 New Year Community Event - Join Us!",
			Content:          "We are excited to announce our annual New Year celebration event! Join us for food, games, and entertainment. All residents are welcome to participate. Date: January 15, 2026. Location: Clubhouse Main Hall. RSVP by January 10.",
			Status:           master.ForumAdStatusActive,
			TargetProperties: master.JSONB{float64(propertyID)},
			TargetCities:     nil,
			TargetTags:       nil,
			CreatedBy:        superAdminID,
			StartedAt:        &now,
		},
		{
			AdType:           master.ForumAdTypeOfficial,
			Title:            "📢 Official Notice: System Maintenance Schedule",
			Content:          "Dear Residents, Please be informed that our property management system will undergo scheduled maintenance on January 20, 2026 from 2:00 AM to 6:00 AM (PHT). During this time, online services may be temporarily unavailable. We apologize for any inconvenience. Thank you for your understanding.",
			Status:           master.ForumAdStatusActive,
			TargetProperties: nil, // All properties
			TargetCities:     nil,
			TargetTags:       nil,
			CreatedBy:        superAdminID,
			StartedAt:        &now,
		},
		{
			AdType:           master.ForumAdTypeMerchant,
			Title:            "🍕 Pizza Palace - 20% Off for All Residents!",
			Content:          "Exclusive offer for HomeX residents! Enjoy 20% off on all orders at Pizza Palace. Use code HOMEX20 when ordering. Valid until February 28, 2026. Free delivery within the property. Contact: 0917-123-4567 or visit our store at Ground Floor, Commercial Area.",
			Status:           master.ForumAdStatusActive,
			TargetProperties: master.JSONB{float64(propertyID)},
			TargetCities:     master.JSONB{"Makati City"},
			TargetTags:       master.JSONB{"resident", "tenant"},
			CreatedBy:        superAdminID,
			StartedAt:        &now,
		},
	}

	for _, ad := range ads {

		// var existing master.ForumAd
		// if err := db.Where("ad_type = ? AND title = ?", ad.AdType, ad.Title).First(&existing).Error; err == nil {
		// 	log.Printf("  - Forum ad already exists: %s", ad.Title[:30])
		// 	continue
		// }

		if err := db.Create(&ad).Error; err != nil {
			log.Printf("  ✗ Failed to create forum ad: %v", err)
		} else {
			log.Printf("  ✓ Created forum ad (%s): %s", ad.AdType, ad.Title[:30]+"...")
		}
	}
}

// ==================== Property Database Seeding ====================

func seedUnits(db *gorm.DB) map[string]uint {
	log.Println("Seeding units...")

	unitIDs := make(map[string]uint)

	units := []property.Unit{
		// Apartments - Building A
		{UnitNumber: "A-101", UnitType: "apartment", Floor: intPtr(1), Building: strPtr("A"), Area: strPtr("45.50"), Bedrooms: intPtr(1), Bathrooms: intPtr(1), Status: "occupied", MonthlyRent: strPtr("25000.00"), Description: strPtr("1BR unit with balcony facing the garden")},
		{UnitNumber: "A-102", UnitType: "apartment", Floor: intPtr(1), Building: strPtr("A"), Area: strPtr("65.00"), Bedrooms: intPtr(2), Bathrooms: intPtr(1), Status: "occupied", MonthlyRent: strPtr("35000.00"), Description: strPtr("2BR corner unit with extra storage")},
		{UnitNumber: "A-201", UnitType: "apartment", Floor: intPtr(2), Building: strPtr("A"), Area: strPtr("45.50"), Bedrooms: intPtr(1), Bathrooms: intPtr(1), Status: "available", MonthlyRent: strPtr("26000.00"), Description: strPtr("1BR unit with city view")},
		{UnitNumber: "A-202", UnitType: "apartment", Floor: intPtr(2), Building: strPtr("A"), Area: strPtr("85.00"), Bedrooms: intPtr(3), Bathrooms: intPtr(2), Status: "occupied", MonthlyRent: strPtr("50000.00"), Description: strPtr("3BR penthouse unit with rooftop access")},
		{UnitNumber: "A-301", UnitType: "apartment", Floor: intPtr(3), Building: strPtr("A"), Area: strPtr("45.50"), Bedrooms: intPtr(1), Bathrooms: intPtr(1), Status: "maintenance", MonthlyRent: strPtr("27000.00"), Description: strPtr("1BR unit under renovation")},
		{UnitNumber: "A-302", UnitType: "apartment", Floor: intPtr(3), Building: strPtr("A"), Area: strPtr("120.00"), Bedrooms: intPtr(4), Bathrooms: intPtr(3), Status: "occupied", MonthlyRent: strPtr("75000.00"), Description: strPtr("4BR premium corner unit with panoramic view")},

		// Apartments - Building B
		{UnitNumber: "B-101", UnitType: "apartment", Floor: intPtr(1), Building: strPtr("B"), Area: strPtr("55.00"), Bedrooms: intPtr(2), Bathrooms: intPtr(1), Status: "occupied", MonthlyRent: strPtr("32000.00"), Description: strPtr("2BR ground floor unit with direct garden access")},
		{UnitNumber: "B-102", UnitType: "apartment", Floor: intPtr(1), Building: strPtr("B"), Area: strPtr("55.00"), Bedrooms: intPtr(2), Bathrooms: intPtr(1), Status: "available", MonthlyRent: strPtr("32000.00"), Description: strPtr("2BR unit near amenities")},
		{UnitNumber: "B-201", UnitType: "apartment", Floor: intPtr(2), Building: strPtr("B"), Area: strPtr("70.00"), Bedrooms: intPtr(2), Bathrooms: intPtr(2), Status: "occupied", MonthlyRent: strPtr("40000.00"), Description: strPtr("2BR unit with maid's room")},
		{UnitNumber: "B-202", UnitType: "apartment", Floor: intPtr(2), Building: strPtr("B"), Area: strPtr("55.00"), Bedrooms: intPtr(2), Bathrooms: intPtr(1), Status: "reserved", MonthlyRent: strPtr("33000.00"), Description: strPtr("2BR unit reserved for incoming tenant")},

		// Storage
		{UnitNumber: "S-001", UnitType: "storage", Floor: intPtr(-1), Area: strPtr("5.00"), Status: "occupied", MonthlyRent: strPtr("2000.00"), Description: strPtr("Small storage unit")},
		{UnitNumber: "S-002", UnitType: "storage", Floor: intPtr(-1), Area: strPtr("10.00"), Status: "available", MonthlyRent: strPtr("3500.00"), Description: strPtr("Large storage unit with climate control")},

		// Commercial
		{UnitNumber: "C-001", UnitType: "commercial", Floor: intPtr(0), Area: strPtr("50.00"), Status: "occupied", MonthlyRent: strPtr("50000.00"), Description: strPtr("Ground floor retail space facing main road")},
		{UnitNumber: "C-002", UnitType: "commercial", Floor: intPtr(0), Area: strPtr("75.00"), Status: "available", MonthlyRent: strPtr("80000.00"), Description: strPtr("Large commercial space suitable for restaurant")},
	}

	for _, unit := range units {
		if err := db.Create(&unit).Error; err != nil {
			log.Printf("  ✗ Failed to create unit %s: %v", unit.UnitNumber, err)
			continue
		}

		log.Printf("  ✓ Created unit: %s (%s)", unit.UnitNumber, unit.UnitType)
		unitIDs[unit.UnitNumber] = unit.ID
	}

	return unitIDs
}

func seedParkingSlots(db *gorm.DB, landlordUserIDs []uint, propertyAdminID uint) map[string]uint {
	log.Println("Seeding parking slots (independent table)...")

	parkingSlotIDs := make(map[string]uint)

	// 定义每个车位的所有者索引
	// -1 表示使用propertyAdminID（物业管理的公共车位）
	slotOwnerMap := map[string]int{
		"P-A01": 0,  // Landlord 1
		"P-A02": 4,  // Landlord 5
		"P-A03": 1,  // Landlord 2
		"P-A04": 4,  // Landlord 5
		"P-B01": 2,  // Landlord 3
		"P-B02": -1, // Property managed
		"P-B03": 2,  // Landlord 3
		"P-G01": -1, // Property managed
		"P-G02": 3,  // Landlord 4
		"P-M01": 0,  // Landlord 1
		"P-M02": -1, // Property managed
		"P-M03": 4,  // Landlord 5
		"P-V01": -1, // Visitor parking (property managed)
		"P-V02": -1, // Visitor parking (property managed)
	}

	getOwnerID := func(slotNum string) uint {
		if idx, ok := slotOwnerMap[slotNum]; ok {
			if idx >= 0 && idx < len(landlordUserIDs) {
				return landlordUserIDs[idx]
			}
		}
		return propertyAdminID // Default to property admin for public/visitor slots
	}

	type slotData struct {
		SlotNumber         string
		ParkingType        string
		ParkingLevel       string
		ParkingZone        string
		VehicleTypeAllowed string
		Size               string
		HasCharger         bool
		IsAccessible       bool
		MonthlyRent        *string
		Status             string
		Description        string
	}

	slots := []slotData{
		// Car parking - Indoor B1
		{"P-A01", "indoor", "B1", "A", "car", "standard", false, false, strPtr("3000.00"), "occupied", "Covered parking near elevator"},
		{"P-A02", "indoor", "B1", "A", "car", "standard", true, false, strPtr("3500.00"), "available", "Covered parking with EV charger"},
		{"P-A03", "indoor", "B1", "A", "car", "standard", false, true, strPtr("3000.00"), "occupied", "Accessible parking near elevator"},
		{"P-A04", "indoor", "B1", "A", "car", "large", false, false, strPtr("3500.00"), "available", "Large vehicle parking"},

		// Car parking - Indoor B2
		{"P-B01", "indoor", "B2", "B", "car", "standard", false, false, strPtr("2500.00"), "occupied", "Lower level parking"},
		{"P-B02", "indoor", "B2", "B", "car", "standard", false, false, strPtr("2500.00"), "available", "Lower level parking"},
		{"P-B03", "indoor", "B2", "B", "car", "compact", false, false, strPtr("2000.00"), "occupied", "Compact car parking"},

		// Car parking - Outdoor Ground
		{"P-G01", "outdoor", "G", "C", "car", "standard", false, false, strPtr("2000.00"), "available", "Open parking area"},
		{"P-G02", "covered", "G", "C", "car", "standard", false, false, strPtr("2500.00"), "occupied", "Covered outdoor parking"},

		// Motorcycle parking
		{"P-M01", "covered", "B1", "M", "motorcycle", "motorcycle", false, false, strPtr("800.00"), "occupied", "Motorcycle parking area"},
		{"P-M02", "covered", "B1", "M", "motorcycle", "motorcycle", false, false, strPtr("800.00"), "available", "Motorcycle parking area"},
		{"P-M03", "covered", "B1", "M", "motorcycle", "motorcycle", false, false, strPtr("800.00"), "occupied", "Motorcycle parking area"},

		// Visitor parking
		{"P-V01", "outdoor", "G", "V", "car", "standard", false, false, nil, "available", "Visitor parking - first 2 hours free"},
		{"P-V02", "outdoor", "G", "V", "car", "standard", false, false, nil, "available", "Visitor parking - first 2 hours free"},
	}

	for _, s := range slots {
		slot := property.ParkingSlot{
			SlotNumber:         s.SlotNumber,
			OwnerID:            getOwnerID(s.SlotNumber),
			ParkingType:        s.ParkingType,
			ParkingLevel:       strPtr(s.ParkingLevel),
			ParkingZone:        strPtr(s.ParkingZone),
			VehicleTypeAllowed: s.VehicleTypeAllowed,
			Size:               strPtr(s.Size),
			HasCharger:         s.HasCharger,
			IsAccessible:       s.IsAccessible,
			MonthlyRent:        s.MonthlyRent,
			Status:             s.Status,
			Description:        strPtr(s.Description),
		}

		if err := db.Create(&slot).Error; err != nil {
			log.Printf("  ✗ Failed to create parking slot %s: %v", slot.SlotNumber, err)
			continue
		}

		log.Printf("  ✓ Created parking slot: %s (%s - %s) owner: %d", slot.SlotNumber, slot.ParkingType, slot.VehicleTypeAllowed, slot.OwnerID)
		parkingSlotIDs[slot.SlotNumber] = slot.ID
	}

	return parkingSlotIDs
}

func seedParkingAssignments(db *gorm.DB, parkingSlotIDs map[string]uint, unitIDs map[string]uint, landlordUserIDs, tenantUserIDs []uint) {
	log.Println("Seeding parking assignments (linking slots to units)...")

	startDate := time.Now().AddDate(0, -6, 0)

	// 定义分配关系：一个unit可以有多个parking slot
	assignments := []struct {
		SlotNumber     string
		UnitNumber     string
		UserIndex      int
		IsLandlord     bool
		VehiclePlate   string
		VehicleType    string
		VehicleBrand   string
		VehicleColor   string
		AssignmentType string
		MonthlyFee     string
	}{
		// Unit A-101 有2个车位 (tenant使用)
		{"P-A01", "A-101", 0, false, "ABC 1234", "car", "Toyota", "White", "tenant", "3000.00"},
		{"P-M01", "A-101", 0, false, "MC 1111", "motorcycle", "Honda", "Black", "tenant", "800.00"},

		// Unit A-102 有1个车位 (tenant使用)
		{"P-A03", "A-102", 1, false, "DEF 5678", "car", "Honda", "Silver", "tenant", "3000.00"},

		// Unit A-202 有2个车位 (landlord拥有，tenant使用)
		{"P-B01", "A-202", 2, false, "GHI 9012", "car", "BMW", "Black", "tenant", "2500.00"},
		{"P-B03", "A-202", 2, false, "JKL 3456", "car", "Mini", "Red", "tenant", "2000.00"},

		// Unit B-101 有1个车位 (tenant使用)
		{"P-G02", "B-101", 3, false, "MNO 7890", "car", "Ford", "Blue", "tenant", "2500.00"},

		// Unit A-302 有2个车位 (landlord拥有)
		{"P-A04", "A-302", 4, true, "PQR 1357", "car", "Mercedes", "Gray", "owner", "3500.00"},
		{"P-M03", "A-302", 4, true, "MC 2468", "motorcycle", "Yamaha", "Blue", "owner", "800.00"},
	}

	for _, a := range assignments {
		slotID, ok := parkingSlotIDs[a.SlotNumber]
		if !ok {
			continue
		}

		unitID, ok := unitIDs[a.UnitNumber]
		if !ok {
			continue
		}

		var userID uint
		if a.IsLandlord {
			if a.UserIndex < len(landlordUserIDs) {
				userID = landlordUserIDs[a.UserIndex]
			}
		} else {
			if a.UserIndex < len(tenantUserIDs) {
				userID = tenantUserIDs[a.UserIndex]
			}
		}

		if userID == 0 {
			continue
		}

		assignment := property.ParkingAssignment{
			ParkingSlotID:  slotID,
			UnitID:         &unitID,
			UserID:         userID,
			VehiclePlate:   a.VehiclePlate,
			VehicleBrand:   strPtr(a.VehicleBrand),
			VehicleColor:   strPtr(a.VehicleColor),
			VehicleType:    a.VehicleType,
			AssignmentType: a.AssignmentType,
			StartDate:      startDate,
			MonthlyFee:     strPtr(a.MonthlyFee),
			Status:         "active",
			Notes:          strPtr(fmt.Sprintf("Assigned to unit %s", a.UnitNumber)),
		}

		if err := db.Create(&assignment).Error; err != nil {
			log.Printf("  ✗ Failed to create parking assignment: %v", err)
			continue
		}

		log.Printf("  ✓ Assigned parking %s to unit %s (plate: %s)", a.SlotNumber, a.UnitNumber, a.VehiclePlate)
	}
}

func seedLandlords(db *gorm.DB, landlordUserIDs []uint, unitIDs map[string]uint) {
	log.Println("Seeding landlord-unit associations...")

	if len(landlordUserIDs) < 5 {
		log.Println("  - Not enough landlord users to seed associations")
		return
	}

	associations := []struct {
		UserID       uint
		UnitNumbers  []string
		OwnershipPct string
	}{
		{landlordUserIDs[0], []string{"A-101", "A-102"}, "100.00"},
		{landlordUserIDs[1], []string{"A-202"}, "100.00"},
		{landlordUserIDs[2], []string{"B-101", "B-201"}, "100.00"},
		{landlordUserIDs[3], []string{"B-202", "S-001", "C-001"}, "100.00"},
		{landlordUserIDs[4], []string{"A-201", "A-302", "S-002"}, "100.00"},
	}

	startDate := time.Now().AddDate(-2, 0, 0)

	for _, assoc := range associations {
		for _, unitNum := range assoc.UnitNumbers {
			unitID, ok := unitIDs[unitNum]
			if !ok {
				continue
			}

			landlord := property.Landlord{
				UserID:              assoc.UserID,
				UnitID:              unitID,
				OwnershipType:       "full",
				OwnershipPercentage: strPtr(assoc.OwnershipPct),
				OwnershipStartDate:  &startDate,
				Notes:               strPtr("Original owner"),
			}

			if err := db.Create(&landlord).Error; err != nil {
				log.Printf("  ✗ Failed to create landlord association: %v", err)
				continue
			}

			log.Printf("  ✓ Associated landlord %d with unit %s", assoc.UserID, unitNum)
		}
	}
}

// seedLandlordParkingSlots 业主拥有的停车位（一个业主可以拥有多个停车位）
func seedLandlordParkingSlots(db *gorm.DB, landlordUserIDs []uint, parkingSlotIDs map[string]uint) {
	log.Println("Seeding landlord parking slot ownership...")

	if len(landlordUserIDs) < 5 {
		log.Println("  - Not enough landlord users to seed parking ownership")
		return
	}

	// 定义业主拥有的停车位
	// 一个业主可以拥有多个停车位
	ownerships := []struct {
		UserID       uint
		SlotNumbers  []string
		OwnershipPct string
	}{
		{landlordUserIDs[0], []string{"P-A01", "P-M01"}, "100.00"},          // Landlord 1 拥有2个车位
		{landlordUserIDs[1], []string{"P-A03"}, "100.00"},                   // Landlord 2 拥有1个车位
		{landlordUserIDs[2], []string{"P-B01", "P-B03"}, "100.00"},          // Landlord 3 拥有2个车位
		{landlordUserIDs[3], []string{"P-G02"}, "100.00"},                   // Landlord 4 拥有1个车位
		{landlordUserIDs[4], []string{"P-A04", "P-M03", "P-A02"}, "100.00"}, // Landlord 5 拥有3个车位
	}

	startDate := time.Now().AddDate(-2, 0, 0)

	for _, ownership := range ownerships {
		for _, slotNum := range ownership.SlotNumbers {
			slotID, ok := parkingSlotIDs[slotNum]
			if !ok {
				continue
			}

			landlordParking := property.LandlordParkingSlot{
				UserID:              ownership.UserID,
				ParkingSlotID:       slotID,
				OwnershipType:       "full",
				OwnershipPercentage: strPtr(ownership.OwnershipPct),
				OwnershipStartDate:  &startDate,
				Notes:               strPtr("Original parking slot owner"),
			}

			if err := db.Create(&landlordParking).Error; err != nil {
				log.Printf("  ✗ Failed to create landlord parking ownership: %v", err)
				continue
			}

			log.Printf("  ✓ Landlord %d owns parking slot %s", ownership.UserID, slotNum)
		}
	}
}

// seedSPAs SPA代理人及其代理的房源和停车位
func seedSPAs(db *gorm.DB, spaUserIDs []uint, unitIDs map[string]uint, parkingSlotIDs map[string]uint) {
	log.Println("Seeding SPAs (Special Power of Attorney) authorization records...")

	if len(spaUserIDs) < 3 {
		log.Println("  - Not enough SPA users to seed SPA authorizations")
		return
	}

	startDate := time.Now().AddDate(0, -6, 0)
	endDate := time.Now().AddDate(1, 0, 0)

	// SPA 1 (Angela Reyes): 代理房源 A-101, A-102, A-201
	spaUserID1 := spaUserIDs[0]
	spaUnits1 := []string{"A-101", "A-102", "A-201"}

	for _, unitNum := range spaUnits1 {
		if unitID, ok := unitIDs[unitNum]; ok {
			spaUnit := property.SPAUnit{
				SpaUserID:              spaUserID1,
				UnitID:                 unitID,
				AuthorizationStartDate: &startDate,
				AuthorizationEndDate:   &endDate,
				Scope:                  "full",
				Notes:                  strPtr("Full authorization for property management - Tower A units"),
				IsActive:               true,
			}
			if err := db.Create(&spaUnit).Error; err == nil {
				log.Printf("  ✓ SPA user %d (Angela) authorized for unit %s", spaUserID1, unitNum)
			} else {
				log.Printf("  ✗ Failed to create SPA unit: %v", err)
			}
		}
	}

	// SPA 1 的停车位授权
	spaSlots1 := []string{"P-A01", "P-A02"}
	for _, slotNum := range spaSlots1 {
		if slotID, ok := parkingSlotIDs[slotNum]; ok {
			spaSlot := property.SPAParkingSlot{
				SpaUserID:              spaUserID1,
				ParkingSlotID:          slotID,
				AuthorizationStartDate: &startDate,
				AuthorizationEndDate:   &endDate,
				Scope:                  "full",
				Notes:                  strPtr("Full authorization for parking slot management"),
				IsActive:               true,
			}
			if err := db.Create(&spaSlot).Error; err == nil {
				log.Printf("  ✓ SPA user %d (Angela) authorized for parking slot %s", spaUserID1, slotNum)
			} else {
				log.Printf("  ✗ Failed to create SPA parking slot: %v", err)
			}
		}
	}

	// SPA 2 (Benjamin Cruz): 代理房源 B-101, B-201, B-202
	spaUserID2 := spaUserIDs[1]
	spaUnits2 := []string{"B-101", "B-201", "B-202"}

	for _, unitNum := range spaUnits2 {
		if unitID, ok := unitIDs[unitNum]; ok {
			spaUnit := property.SPAUnit{
				SpaUserID:              spaUserID2,
				UnitID:                 unitID,
				AuthorizationStartDate: &startDate,
				AuthorizationEndDate:   &endDate,
				Scope:                  "full",
				Notes:                  strPtr("Full authorization for property management - Tower B units"),
				IsActive:               true,
			}
			if err := db.Create(&spaUnit).Error; err == nil {
				log.Printf("  ✓ SPA user %d (Benjamin) authorized for unit %s", spaUserID2, unitNum)
			} else {
				log.Printf("  ✗ Failed to create SPA unit: %v", err)
			}
		}
	}

	// SPA 2 的停车位授权
	spaSlots2 := []string{"P-B01", "P-M01", "P-M02"}
	for _, slotNum := range spaSlots2 {
		if slotID, ok := parkingSlotIDs[slotNum]; ok {
			spaSlot := property.SPAParkingSlot{
				SpaUserID:              spaUserID2,
				ParkingSlotID:          slotID,
				AuthorizationStartDate: &startDate,
				AuthorizationEndDate:   &endDate,
				Scope:                  "full",
				Notes:                  strPtr("Full authorization for parking slot management"),
				IsActive:               true,
			}
			if err := db.Create(&spaSlot).Error; err == nil {
				log.Printf("  ✓ SPA user %d (Benjamin) authorized for parking slot %s", spaUserID2, slotNum)
			} else {
				log.Printf("  ✗ Failed to create SPA parking slot: %v", err)
			}
		}
	}

	// SPA 3 (Catherine Lim): 代理房源 A-302, S-001, S-002, C-001
	spaUserID3 := spaUserIDs[2]
	spaUnits3 := []string{"A-302", "S-001", "S-002", "C-001"}

	for _, unitNum := range spaUnits3 {
		if unitID, ok := unitIDs[unitNum]; ok {
			spaUnit := property.SPAUnit{
				SpaUserID:              spaUserID3,
				UnitID:                 unitID,
				AuthorizationStartDate: &startDate,
				AuthorizationEndDate:   &endDate,
				Scope:                  "full",
				Notes:                  strPtr("Full authorization for property management - Special units"),
				IsActive:               true,
			}
			if err := db.Create(&spaUnit).Error; err == nil {
				log.Printf("  ✓ SPA user %d (Catherine) authorized for unit %s", spaUserID3, unitNum)
			} else {
				log.Printf("  ✗ Failed to create SPA unit: %v", err)
			}
		}
	}

	// SPA 3 的停车位授权
	spaSlots3 := []string{"P-A03", "P-A04", "P-M03"}
	for _, slotNum := range spaSlots3 {
		if slotID, ok := parkingSlotIDs[slotNum]; ok {
			spaSlot := property.SPAParkingSlot{
				SpaUserID:              spaUserID3,
				ParkingSlotID:          slotID,
				AuthorizationStartDate: &startDate,
				AuthorizationEndDate:   &endDate,
				Scope:                  "full",
				Notes:                  strPtr("Full authorization for parking slot management"),
				IsActive:               true,
			}
			if err := db.Create(&spaSlot).Error; err == nil {
				log.Printf("  ✓ SPA user %d (Catherine) authorized for parking slot %s", spaUserID3, slotNum)
			} else {
				log.Printf("  ✗ Failed to create SPA parking slot: %v", err)
			}
		}
	}
}

func seedTenants(db *gorm.DB, tenantUserIDs []uint, unitIDs map[string]uint) {
	log.Println("Seeding tenant leases...")

	if len(tenantUserIDs) < 6 {
		log.Println("  - Not enough tenant users to seed leases")
		return
	}

	leaseStart := time.Now().AddDate(0, -6, 0)
	leaseEnd := time.Now().AddDate(0, 6, 0)

	leases := []struct {
		UserID         uint
		UnitNumber     string
		MonthlyRent    string
		DepositAmount  string
		ContractNumber string
	}{
		{tenantUserIDs[0], "A-101", "25000.00", "50000.00", "LEASE-2024-001"},
		{tenantUserIDs[1], "A-102", "35000.00", "70000.00", "LEASE-2024-002"},
		{tenantUserIDs[2], "A-202", "50000.00", "100000.00", "LEASE-2024-003"},
		{tenantUserIDs[3], "B-101", "32000.00", "64000.00", "LEASE-2024-004"},
		{tenantUserIDs[4], "B-201", "40000.00", "80000.00", "LEASE-2024-005"},
		{tenantUserIDs[5], "A-302", "75000.00", "150000.00", "LEASE-2024-006"},
	}

	for _, lease := range leases {
		unitID, ok := unitIDs[lease.UnitNumber]
		if !ok {
			continue
		}

		tenant := property.Tenant{
			UserID:         lease.UserID,
			UnitID:         unitID,
			LeaseStartDate: leaseStart,
			LeaseEndDate:   leaseEnd,
			MonthlyRent:    lease.MonthlyRent,
			DepositAmount:  strPtr(lease.DepositAmount),
			ContractNumber: strPtr(lease.ContractNumber),
			Status:         "active",
			Notes:          strPtr("Standard lease agreement"),
		}

		if err := db.Create(&tenant).Error; err != nil {
			log.Printf("  ✗ Failed to create tenant lease: %v", err)
			continue
		}

		log.Printf("  ✓ Created lease for tenant %d in unit %s", lease.UserID, lease.UnitNumber)
	}
}

func seedAnnouncements(db *gorm.DB, createdBy uint) {
	log.Println("Seeding announcements...")

	now := time.Now()
	publishedAt := now.AddDate(0, 0, -7)
	expiresAt := now.AddDate(0, 1, 0)

	announcements := []property.Announcement{
		{
			Title:       "Welcome to Demo Residence",
			Content:     "We are pleased to welcome all residents to Demo Residence. Please familiarize yourself with the building rules and regulations available at the management office. For any concerns, please don't hesitate to contact us.",
			Category:    "general",
			Priority:    "normal",
			Status:      "published",
			PublishedAt: &publishedAt,
			ExpiresAt:   &expiresAt,
			CreatedBy:   createdBy,
		},
		{
			Title:       "Scheduled Water Interruption",
			Content:     "Please be advised that there will be a scheduled water interruption on December 15, 2024, from 10:00 AM to 4:00 PM due to pipe maintenance. We apologize for any inconvenience.",
			Category:    "maintenance",
			Priority:    "high",
			Status:      "published",
			PublishedAt: &now,
			ExpiresAt:   timePtr(now.AddDate(0, 0, 10)),
			CreatedBy:   createdBy,
		},
		{
			Title:       "Holiday Party Invitation",
			Content:     "You are cordially invited to our annual Holiday Party on December 20, 2024, at 6:00 PM in the function room. Food and drinks will be provided. Please RSVP at the management office.",
			Category:    "event",
			Priority:    "normal",
			Status:      "published",
			PublishedAt: &now,
			ExpiresAt:   timePtr(now.AddDate(0, 0, 15)),
			CreatedBy:   createdBy,
		},
		{
			Title:       "New Parking Policy",
			Content:     "Effective January 1, 2025, all vehicles must display the official building sticker. Unregistered vehicles will not be allowed entry. Please register your vehicle at the management office.",
			Category:    "policy",
			Priority:    "high",
			Status:      "published",
			PublishedAt: &now,
			ExpiresAt:   timePtr(now.AddDate(0, 3, 0)),
			CreatedBy:   createdBy,
		},
		{
			Title:       "Fire Drill Schedule",
			Content:     "A fire drill will be conducted on December 18, 2024, at 2:00 PM. All residents are required to participate. Please proceed to the designated assembly area when you hear the alarm.",
			Category:    "emergency",
			Priority:    "urgent",
			Status:      "published",
			PublishedAt: &now,
			ExpiresAt:   timePtr(now.AddDate(0, 0, 13)),
			CreatedBy:   createdBy,
		},
		{
			Title:       "Gym Equipment Upgrade",
			Content:     "We are happy to announce that new gym equipment will be installed next week. The gym will be temporarily closed from December 12-14, 2024. Thank you for your patience.",
			Category:    "maintenance",
			Priority:    "normal",
			Status:      "published",
			PublishedAt: &now,
			ExpiresAt:   timePtr(now.AddDate(0, 0, 9)),
			CreatedBy:   createdBy,
		},
		{
			Title:       "Pool Maintenance",
			Content:     "The swimming pool will undergo maintenance on December 16, 2024. Pool access will be restricted from 8:00 AM to 5:00 PM.",
			Category:    "maintenance",
			Priority:    "normal",
			Status:      "published",
			PublishedAt: &now,
			ExpiresAt:   timePtr(now.AddDate(0, 0, 11)),
			CreatedBy:   createdBy,
		},
		{
			Title:     "Draft: New Year Celebration",
			Content:   "Join us for a spectacular New Year's Eve celebration...",
			Category:  "event",
			Priority:  "normal",
			Status:    "draft",
			CreatedBy: createdBy,
		},
	}

	for _, ann := range announcements {

		if err := db.Create(&ann).Error; err != nil {
			log.Printf("  ✗ Failed to create announcement: %v", err)
			continue
		}

		log.Printf("  ✓ Created announcement: %s", ann.Title)
	}
}

func seedBills(db *gorm.DB, unitIDs, parkingSlotIDs map[string]uint, tenantUserIDs, landlordUserIDs []uint) map[string]uint {
	log.Println("Seeding bills...")

	billIDs := make(map[string]uint)
	now := time.Now()

	bills := []struct {
		UnitNumber        string
		ParkingSlotNumber string // For parking bills
		UserIndex         int
		IsLandlord        bool
		BillType          string
		Amount            string
		DueDate           time.Time
		Status            string
		Description       string
	}{
		// Unit bills (物业管理费) for tenants and landlords
		{"A-101", "", 0, false, "unit", "25000.00", now.AddDate(0, 0, -15), "paid", "December 2024 Unit Management Fee"},
		{"A-102", "", 1, false, "unit", "35000.00", now.AddDate(0, 0, 15), "pending", "January 2025 Unit Management Fee"},
		{"A-202", "", 2, false, "unit", "50000.00", now.AddDate(0, 0, -5), "overdue", "December 2024 Unit Management Fee"},
		{"B-101", "", 3, false, "unit", "32000.00", now.AddDate(0, 0, 15), "pending", "January 2025 Unit Management Fee"},
		{"B-201", "", 4, false, "unit", "40000.00", now.AddDate(0, 0, -10), "partial", "December 2024 Unit Management Fee"},
		{"A-302", "", 5, false, "unit", "75000.00", now.AddDate(0, 0, -20), "paid", "December 2024 Unit Management Fee"},

		// Parking bills (停车位管理费)
		{"", "P-A01", 0, false, "parking", "3800.00", now.AddDate(0, 0, -5), "paid", "December 2024 Parking Slot Management Fee (P-A01 + P-M01)"},
		{"", "P-A04", 4, true, "parking", "4300.00", now.AddDate(0, 0, 15), "pending", "January 2025 Parking Slot Management Fee (P-A04 + P-M03)"},
	}

	for _, b := range bills {
		var unitID *uint
		var parkingSlotID *uint
		var billNumber string
		var number string

		// Get billing month from DueDate (YYYY-MM format)
		billingMonth := b.DueDate.Format("2006-01")

		// Set UnitID or ParkingSlotID and generate bill number based on bill type
		if b.BillType == "unit" {
			unitIDVal, ok := unitIDs[b.UnitNumber]
			if !ok {
				continue
			}
			unitID = &unitIDVal
			number = b.UnitNumber
			billNumber = fmt.Sprintf("%s_%s", number, strings.ReplaceAll(billingMonth, "-", "_"))
		} else if b.BillType == "parking" {
			parkingSlotIDVal, ok := parkingSlotIDs[b.ParkingSlotNumber]
			if !ok {
				continue
			}
			parkingSlotID = &parkingSlotIDVal
			number = b.ParkingSlotNumber
			billNumber = fmt.Sprintf("%s_%s", number, strings.ReplaceAll(billingMonth, "-", "_"))
		} else {
			continue
		}

		var userID uint
		var tenantID, landlordID *uint
		if b.IsLandlord {
			if b.UserIndex < len(landlordUserIDs) {
				userID = landlordUserIDs[b.UserIndex]
				landlordID = &landlordUserIDs[b.UserIndex]
			}
		} else {
			if b.UserIndex < len(tenantUserIDs) {
				userID = tenantUserIDs[b.UserIndex]
				tenantID = &tenantUserIDs[b.UserIndex]
			}
		}

		if userID == 0 {
			continue
		}

		paidAmount := "0.00"
		var paidAt *time.Time
		if b.Status == "paid" {
			paidAmount = b.Amount
			paidTime := b.DueDate.AddDate(0, 0, -2)
			paidAt = &paidTime
		} else if b.Status == "partial" {
			paidAmount = "20000.00"
		}

		bill := property.Bill{
			BillNumber:    billNumber,
			UnitID:        unitID,
			ParkingSlotID: parkingSlotID,
			UserID:        userID,
			TenantID:      tenantID,
			LandlordID:    landlordID,
			BillType:      b.BillType,
			BillingMonth:  billingMonth,
			Amount:        b.Amount,
			Currency:      "PHP",
			DueDate:       b.DueDate,
			IssueDate:     b.DueDate.AddDate(0, 0, -30),
			Status:        b.Status,
			Description:   strPtr(b.Description),
			PaidAmount:    paidAmount,
			PaidAt:        paidAt,
		}

		if err := db.Create(&bill).Error; err != nil {
			log.Printf("  ✗ Failed to create bill %s: %v", billNumber, err)
			continue
		}

		log.Printf("  ✓ Created bill: %s (%s - %s)", billNumber, b.BillType, b.Status)
		billIDs[billNumber] = bill.ID
	}

	return billIDs
}

func seedPayments(db *gorm.DB, billIDs map[string]uint, tenantUserIDs []uint) {
	log.Println("Seeding payments...")

	now := time.Now()

	payments := []struct {
		PaymentNumber     string
		UnitNumber        string    // For unit bills
		ParkingSlotNumber string    // For parking bills
		BillType          string    // "unit" or "parking"
		DueDate           time.Time // Bill due date to calculate billing month
		UserIndex         int
		Amount            string
		PaymentMethod     string
		PaymentDate       time.Time
		TransactionID     string
		ReferenceNumber   string
	}{
		// Match with seedBills data: A-101, DueDate: now.AddDate(0, 0, -15)
		{"PAY-2024-001", "A-101", "", "unit", now.AddDate(0, 0, -15), 0, "25000.00", "bank_transfer", now.AddDate(0, 0, -17), "BT20241201001", "REF-001"},
		// Match with seedBills data: A-302, DueDate: now.AddDate(0, 0, -20)
		{"PAY-2024-002", "A-302", "", "unit", now.AddDate(0, 0, -20), 5, "75000.00", "gcash", now.AddDate(0, 0, -22), "GC20241128001", "REF-002"},
		// Match with seedBills data: P-A01, DueDate: now.AddDate(0, 0, -5)
		{"PAY-2024-003", "", "P-A01", "parking", now.AddDate(0, 0, -5), 0, "3800.00", "credit_card", now.AddDate(0, 0, -7), "CC20241205001", "REF-003"},
		// Match with seedBills data: B-201, DueDate: now.AddDate(0, 0, -10)
		{"PAY-2024-004", "B-201", "", "unit", now.AddDate(0, 0, -10), 4, "20000.00", "cash", now.AddDate(0, 0, -8), "", "RCPT-2024-001"},
	}

	for _, p := range payments {
		// Calculate bill number using same logic as seedBills
		billingMonth := p.DueDate.Format("2006-01")
		var billNumber string
		if p.BillType == "unit" {
			billNumber = fmt.Sprintf("%s_%s", p.UnitNumber, strings.ReplaceAll(billingMonth, "-", "_"))
		} else if p.BillType == "parking" {
			billNumber = fmt.Sprintf("%s_%s", p.ParkingSlotNumber, strings.ReplaceAll(billingMonth, "-", "_"))
		} else {
			continue
		}

		billID, ok := billIDs[billNumber]
		if !ok {
			continue
		}

		if p.UserIndex >= len(tenantUserIDs) {
			continue
		}
		userID := tenantUserIDs[p.UserIndex]

		payment := property.Payment{
			PaymentNumber:   p.PaymentNumber,
			BillID:          billID,
			UserID:          userID,
			Amount:          p.Amount,
			Currency:        "PHP",
			PaymentMethod:   p.PaymentMethod,
			PaymentDate:     p.PaymentDate,
			Status:          "completed",
			TransactionID:   strPtr(p.TransactionID),
			ReferenceNumber: strPtr(p.ReferenceNumber),
			Notes:           strPtr("Payment received"),
		}

		if err := db.Create(&payment).Error; err != nil {
			log.Printf("  ✗ Failed to create payment %s: %v", p.PaymentNumber, err)
			continue
		}

		log.Printf("  ✓ Created payment: %s (%s)", p.PaymentNumber, p.PaymentMethod)
	}
}

func seedRequests(db *gorm.DB, unitIDs map[string]uint, tenantUserIDs, landlordUserIDs []uint, adminID uint) {
	log.Println("Seeding requests...")

	now := time.Now()

	requests := []struct {
		UnitNumber  string
		UserIndex   int
		IsLandlord  bool
		Category    string
		RequestType string
		Title       string
		Description string
		Priority    string
		Status      string
		AssignedTo  *uint
		Resolution  string
	}{
		// House category requests
		{"A-302", 5, false, "house", "gate_pass", "Request for gate pass", "I need a gate pass for my visiting relative who will stay for a week.", "normal", "pending", nil, ""},
		{"A-202", 1, true, "house", "move_out", "Move-out request", "Tenant has given notice. Please schedule move-out inspection for end of month.", "normal", "pending", &adminID, ""},
		{"A-101", 0, false, "house", "move_in", "Move-in inspection request", "Please schedule a move-in inspection for next week.", "normal", "rejected", &adminID, ""},
		// Parking category requests
		{"A-101", 0, false, "parking", "parking_rent_apply", "Apply for parking slot rental", "I would like to rent a parking slot for my second vehicle.", "normal", "pending", nil, ""},
		{"B-201", 4, false, "parking", "parking_sticker_apply", "Parking sticker application", "Need a new parking sticker for my vehicle. Old one expired.", "normal", "in_progress", &adminID, ""},
		{"A-202", 2, false, "parking", "parking_rent_termination", "Terminate parking rental", "I will be selling my car and would like to terminate my parking slot rental.", "low", "pending", nil, ""},
	}

	for i, r := range requests {
		unitID, ok := unitIDs[r.UnitNumber]
		if !ok {
			continue
		}

		var userID uint
		if r.IsLandlord {
			if r.UserIndex < len(landlordUserIDs) {
				userID = landlordUserIDs[r.UserIndex]
			}
		} else {
			if r.UserIndex < len(tenantUserIDs) {
				userID = tenantUserIDs[r.UserIndex]
			}
		}

		if userID == 0 {
			continue
		}

		request := property.Request{
			UserID:      userID,
			UnitID:      &unitID,
			Category:    r.Category,
			RequestType: r.RequestType,
			Title:       r.Title,
			Description: strPtr(r.Description),
			Priority:    r.Priority,
			Status:      r.Status,
			AssignedTo:  r.AssignedTo,
		}

		if r.Status == "completed" {
			resolvedAt := now.AddDate(0, 0, -3)
			request.ResolvedAt = &resolvedAt
			request.ResolvedBy = &adminID
			request.Resolution = strPtr(r.Resolution)
		}

		if err := db.Create(&request).Error; err != nil {
			log.Printf("  ✗ Failed to create request: %v", err)
			continue
		}

		log.Printf("  ✓ Created request #%d: [%s] %s (%s)", i+1, r.Category, r.Title, r.Status)
	}
}

func seedVisitors(db *gorm.DB, unitIDs map[string]uint, tenantUserIDs, landlordUserIDs []uint, guardID uint) {
	log.Println("Seeding visitors...")

	now := time.Now()

	visitors := []struct {
		UnitNumber   string
		HostIndex    int
		IsLandlord   bool
		VisitorName  string
		VisitorPhone string
		Purpose      string
		VehiclePlate string
		ExpectedAt   time.Time
		Status       string
		CheckedIn    bool
		CheckedOut   bool
	}{
		// Pending visitors (awaiting approval)
		{"A-101", 0, false, "James Anderson", "+63 920 111 1111", "visit", "ABC 1234", now.AddDate(0, 0, 1).Add(10 * time.Hour), "pending", false, false},
		{"A-102", 1, false, "Lazada Delivery", "+63 920 222 2222", "delivery", "", now.Add(2 * time.Hour), "pending", false, false},
		{"A-302", 5, false, "Family Visit - Mom", "+63 920 666 6666", "visit", "GHI 3456", now.AddDate(0, 0, 2).Add(14 * time.Hour), "pending", false, false},

		// Approved visitors (waiting to arrive)
		{"B-201", 4, false, "Plumber Mr. Santos", "+63 920 555 5555", "service", "DEF 9012", now.Add(4 * time.Hour), "approved", false, false},
		{"A-101", 0, true, "Real Estate Agent", "+63 920 999 1111", "visit", "QRS 4567", now.AddDate(0, 0, 1).Add(14 * time.Hour), "approved", false, false},

		// Currently checked in
		{"A-202", 2, false, "AC Technician", "+63 920 333 3333", "service", "XYZ 5678", now.Add(-1 * time.Hour), "checked_in", true, false},
		{"B-101", 0, true, "Furniture Assembly Team", "+63 920 888 9999", "service", "TUV 7890", now.Add(-30 * time.Minute), "checked_in", true, false},

		// Checked out (completed visits)
		{"B-101", 3, false, "Maria's Sister Ana", "+63 920 444 4444", "visit", "", now.Add(-5 * time.Hour), "checked_out", true, true},
		{"A-101", 0, true, "Property Inspector", "+63 920 777 7777", "service", "JKL 7890", now.Add(-24 * time.Hour), "checked_out", true, true},
		{"A-202", 1, true, "Furniture Supplier", "+63 920 888 8888", "delivery", "MNO 1234", now.Add(-48 * time.Hour), "checked_out", true, true},

		// Rejected visitor
		{"A-101", 0, false, "Unknown Person", "+63 920 000 0000", "visit", "", now.Add(-72 * time.Hour), "rejected", false, false},
	}

	for _, v := range visitors {
		unitID, ok := unitIDs[v.UnitNumber]
		if !ok {
			continue
		}

		var hostUserID uint
		if v.IsLandlord {
			if v.HostIndex < len(landlordUserIDs) {
				hostUserID = landlordUserIDs[v.HostIndex]
			}
		} else {
			if v.HostIndex < len(tenantUserIDs) {
				hostUserID = tenantUserIDs[v.HostIndex]
			}
		}

		if hostUserID == 0 {
			continue
		}

		visitor := property.Visitor{
			UnitID:       unitID,
			HostUserID:   hostUserID,
			VisitorName:  v.VisitorName,
			VisitorPhone: strPtr(v.VisitorPhone),
			Purpose:      v.Purpose,
			ExpectedAt:   v.ExpectedAt,
			Status:       v.Status,
		}

		if v.VehiclePlate != "" {
			visitor.VehiclePlate = strPtr(v.VehiclePlate)
		}

		if v.CheckedIn {
			checkedInAt := v.ExpectedAt.Add(5 * time.Minute)
			visitor.CheckedInAt = &checkedInAt
			visitor.CheckedInBy = &guardID
		}

		if v.CheckedOut {
			checkedOutAt := v.ExpectedAt.Add(2 * time.Hour)
			visitor.CheckedOutAt = &checkedOutAt
			visitor.CheckedOutBy = &guardID
		}

		if err := db.Create(&visitor).Error; err != nil {
			log.Printf("  ✗ Failed to create visitor: %v", err)
			continue
		}

		log.Printf("  ✓ Created visitor: %s visiting %s (%s)", v.VisitorName, v.UnitNumber, v.Status)
	}
}

// seedComplaints 创建投诉记录
func seedComplaints(db *gorm.DB, unitIDs map[string]uint, tenantUserIDs []uint) {
	log.Println("Seeding complaints...")

	if len(tenantUserIDs) < 1 {
		log.Println("  - Not enough tenant users to seed complaints")
		return
	}

	now := time.Now()

	// 获取第一个租户的单元
	var unitID uint
	for _, id := range unitIDs {
		unitID = id
		break
	}

	complaint := property.Complaint{
		UserID:      tenantUserIDs[0],
		UnitID:      &unitID,
		Title:       "Noisy neighbor at night",
		Description: strPtr("My neighbor in unit A-102 plays loud music every night after 11pm. This has been going on for 2 weeks. Please help resolve this issue."),
		Priority:    "high",
		Status:      "open",
		CreatedAt:   now.AddDate(0, 0, -1),
		UpdatedAt:   now.AddDate(0, 0, -1),
	}

	if err := db.Create(&complaint).Error; err != nil {
		log.Printf("  ✗ Failed to create complaint: %v", err)
		return
	}

	log.Printf("  ✓ Created complaint: %s (priority: %s, status: %s)", complaint.Title, complaint.Priority, complaint.Status)
}

// seedComplaintMessages 为投诉创建示例消息
func seedComplaintMessages(db *gorm.DB, tenantUserIDs []uint, propertyStaffID uint) {
	log.Println("Seeding complaint messages...")

	if len(tenantUserIDs) < 1 {
		log.Println("  - Not enough users to seed complaint messages")
		return
	}

	now := time.Now()

	// 获取第一个投诉
	var complaint property.Complaint
	if err := db.First(&complaint).Error; err != nil {
		log.Printf("  - No complaints found, skipping complaint messages")
		return
	}

	// 用户的消息
	userMessage1 := property.ComplaintMessage{
		ComplaintID: complaint.ID,
		UserID:      complaint.UserID,
		Message:     "The noise is really disturbing my sleep. Can you please address this issue urgently?",
		IsFromStaff: false,
		CreatedAt:   now.AddDate(0, 0, -1).Add(2 * time.Hour),
	}

	if err := db.Create(&userMessage1).Error; err != nil {
		log.Printf("  ✗ Failed to create user complaint message: %v", err)
		return
	}

	// 工作人员的回复
	staffMessage1 := property.ComplaintMessage{
		ComplaintID: complaint.ID,
		UserID:      propertyStaffID,
		Message:     "Thank you for reporting this issue. We will investigate the noise complaint and contact the neighbor in unit A-102. We will follow up with you within 24 hours.",
		IsFromStaff: true,
		CreatedAt:   now.AddDate(0, 0, -1).Add(4 * time.Hour),
	}

	if err := db.Create(&staffMessage1).Error; err != nil {
		log.Printf("  ✗ Failed to create staff complaint message: %v", err)
		return
	}

	log.Printf("  ✓ Created complaint conversation with %d messages", 2)
}

// seedDocuments 通用文档表（支付凭证、SPA授权文档等）
func seedDocuments(db *gorm.DB, billIDs map[string]uint, tenantUserIDs []uint) {
	log.Println("Seeding documents (payment receipts, etc.)...")

	if len(tenantUserIDs) < 3 {
		log.Println("  - Not enough tenant users to seed documents")
		return
	}

	now := time.Now()

	// 支付凭证示例
	docs := []struct {
		EntityType   string
		EntityID     uint
		DocumentType string
		DocumentName string
		DocumentPath string
		UploadedBy   uint
	}{
		// Tenant 1 上传的支付凭证 for BILL-2024-001
		{
			EntityType:   "payment",
			EntityID:     1, // Payment ID (假设)
			DocumentType: "payment_receipt",
			DocumentName: "bank_transfer_receipt_001.pdf",
			DocumentPath: "/uploads/payments/2024/12/bank_transfer_receipt_001.pdf",
			UploadedBy:   tenantUserIDs[0],
		},
		// Tenant 6 上传的 GCash 支付截图 for BILL-2024-006
		{
			EntityType:   "payment",
			EntityID:     2, // Payment ID
			DocumentType: "payment_receipt",
			DocumentName: "gcash_screenshot_002.png",
			DocumentPath: "/uploads/payments/2024/12/gcash_screenshot_002.png",
			UploadedBy:   tenantUserIDs[5],
		},
		// SPA 授权文档
		{
			EntityType:   "spa",
			EntityID:     1, // SPA ID
			DocumentType: "authorization",
			DocumentName: "spa_authorization_landlord4.pdf",
			DocumentPath: "/uploads/spa/2024/spa_authorization_landlord4.pdf",
			UploadedBy:   tenantUserIDs[3], // Landlord 4 上传
		},
		{
			EntityType:   "spa",
			EntityID:     1,
			DocumentType: "id_copy",
			DocumentName: "spa_id_copy_landlord4.pdf",
			DocumentPath: "/uploads/spa/2024/spa_id_copy_landlord4.pdf",
			UploadedBy:   tenantUserIDs[3],
		},
		// SPA 2 的多个授权文档
		{
			EntityType:   "spa",
			EntityID:     2,
			DocumentType: "authorization",
			DocumentName: "spa_authorization_landlord5.pdf",
			DocumentPath: "/uploads/spa/2024/spa_authorization_landlord5.pdf",
			UploadedBy:   tenantUserIDs[4],
		},
		{
			EntityType:   "spa",
			EntityID:     2,
			DocumentType: "notarized",
			DocumentName: "spa_notarized_landlord5.pdf",
			DocumentPath: "/uploads/spa/2024/spa_notarized_landlord5.pdf",
			UploadedBy:   tenantUserIDs[4],
		},
	}

	for _, d := range docs {

		doc := property.Document{
			EntityType:   d.EntityType,
			EntityID:     d.EntityID,
			DocumentType: d.DocumentType,
			DocumentName: d.DocumentName,
			DocumentPath: d.DocumentPath,
			FileSize:     func() *int64 { v := int64(1024 * 100); return &v }(), // 100KB
			MimeType:     strPtr("application/pdf"),
			UploadedBy:   d.UploadedBy,
			UploadedAt:   now.AddDate(0, 0, -7),
			IsActive:     true,
		}

		if err := db.Create(&doc).Error; err != nil {
			log.Printf("  ✗ Failed to create document: %v", err)
			continue
		}

		log.Printf("  ✓ Created document: %s (%s/%d)", d.DocumentName, d.EntityType, d.EntityID)
	}
}

// seedForumPosts 创建论坛帖子
func seedForumPosts(db *gorm.DB, tenantUserIDs, landlordUserIDs []uint, adminID uint) {
	log.Println("Seeding forum posts...")

	now := time.Now()
	deadline := now.AddDate(0, 0, 7)              // 7天后截止
	eventTime := now.AddDate(0, 0, 14)            // 14天后活动
	registrationDeadline := now.AddDate(0, 0, 10) // 10天后报名截止

	// Vote post
	votePost := property.ForumPost{
		PostType: property.PostTypeVote,
		Title:    "Community Activity Vote: Which activity would you like to have?",
		Content:  "Hello everyone! To enrich our community life, we would like to gather your opinions. Please vote for the type of activity you would most like to have.",
		TemplateData: property.JSONB{
			"options":        []interface{}{"BBQ Party", "Movie Night", "Sports Competition", "Talent Show", "Other"},
			"allow_multiple": false,
			"deadline":       deadline.Format(time.RFC3339),
		},
		UserID:     adminID,
		ViewCount:  45,
		ReplyCount: 3,
		IsPinned:   true,
		PinnedAt:   timePtr(now.AddDate(0, 0, -2)),
		IsLocked:   false,
		IsEdited:   false,
	}

	// Activity post
	activityPost := property.ForumPost{
		PostType: property.PostTypeActivity,
		Title:    "New Year Community Gathering Event",
		Content:  "Inviting all neighbors to join our New Year gathering! The event includes food, games, and raffle prizes. Looking forward to your participation!",
		TemplateData: property.JSONB{
			"event_time":            eventTime.Format(time.RFC3339),
			"location":              "Community Activity Center",
			"registration_deadline": registrationDeadline.Format(time.RFC3339),
			"max_participants":      50,
		},
		UserID:     adminID,
		ViewCount:  32,
		ReplyCount: 8,
		IsPinned:   true,
		PinnedAt:   timePtr(now.AddDate(0, 0, -1)),
		IsLocked:   false,
		IsEdited:   false,
	}

	// Help post
	helpPost := property.ForumPost{
		PostType: property.PostTypeHelp,
		Title:    "Looking for Lost Keys",
		Content:  "Lost a set of keys near the parking lot yesterday afternoon. There's a small keychain attached. If anyone found them, please contact me. Thank you very much!",
		TemplateData: property.JSONB{
			"urgency": "medium",
			"contact": "tenant1@demo.homex.ph",
		},
		UserID:     tenantUserIDs[0],
		ViewCount:  18,
		ReplyCount: 2,
		IsPinned:   false,
		IsLocked:   false,
		IsEdited:   false,
	}

	// Marketplace post
	marketplacePost := property.ForumPost{
		PostType: property.PostTypeMarketplace,
		Title:    "Selling Used Refrigerator",
		Content:  "Selling a one-year-old refrigerator in good working condition and appearance. Selling due to moving, price negotiable.",
		TemplateData: property.JSONB{
			"price":     "8000 PHP",
			"condition": "like_new",
			"contact":   "landlord1@demo.homex.ph",
		},
		UserID:     landlordUserIDs[0],
		ViewCount:  25,
		ReplyCount: 1,
		IsPinned:   false,
		IsLocked:   false,
		IsEdited:   false,
	}

	// Social post
	socialPost := property.ForumPost{
		PostType: property.PostTypeSocial,
		Title:    "Looking for Workout Buddy",
		Content:  "Hello everyone! I'm a fitness enthusiast looking for like-minded neighbors to work out together. We can encourage each other and progress together!",
		TemplateData: property.JSONB{
			"introduction": "I'm a software engineer who enjoys sports, reading, and traveling. I hope to meet more interesting neighbors.",
			"interests":    []interface{}{"Fitness", "Running", "Reading", "Traveling"},
			"contact":      "tenant2@demo.homex.ph",
		},
		UserID:     tenantUserIDs[1],
		ViewCount:  12,
		ReplyCount: 0,
		IsPinned:   false,
		IsLocked:   false,
		IsEdited:   false,
	}

	// Another vote post (locked)
	lockedVotePost := property.ForumPost{
		PostType: property.PostTypeVote,
		Title:    "Community Facility Improvement Vote (Ended)",
		Content:  "This vote has ended. Thank you for your participation!",
		TemplateData: property.JSONB{
			"options":        []interface{}{"Add Gym Equipment", "Improve Parking Lot", "Add Children's Playground", "Improve Greenery"},
			"allow_multiple": true,
			"deadline":       now.AddDate(0, 0, -5).Format(time.RFC3339), // Ended 5 days ago
		},
		UserID:     adminID,
		ViewCount:  67,
		ReplyCount: 5,
		IsPinned:   false,
		IsLocked:   true, // 已锁定
		IsEdited:   false,
	}

	// Unit rent/sublet post
	unitRentPost := property.ForumPost{
		PostType: property.PostTypeRent,
		Title:    "Unit A-201 for Rent",
		Content:  "1-bedroom unit on 2nd floor of Building A, 45.5 sqm with balcony and open view. Subletting due to job transfer, lease term negotiable.",
		TemplateData: property.JSONB{
			"rent_type":   "unit",
			"unit_number": "A-201",
			"price":       "₱26,000/month",
			"description": "1 bedroom, 1 bathroom, with balcony, fully furnished and well-decorated. Convenient location near elevator and stairs. Suitable for singles or couples.",
			"contact":     "landlord5@demo.homex.ph",
		},
		UserID:     landlordUserIDs[4],
		ViewCount:  28,
		ReplyCount: 2,
		IsPinned:   false,
		IsLocked:   false,
		IsEdited:   false,
	}

	// Parking slot rent/sublet post
	parkingRentPost := property.ForumPost{
		PostType: property.PostTypeRent,
		Title:    "Parking Slot P-A02 for Rent",
		Content:  "Standard parking slot in B1 Level, Zone A, with EV charging station. Convenient location near elevator. Subletting as we no longer use an electric vehicle.",
		TemplateData: property.JSONB{
			"rent_type":           "parking_slot",
			"parking_slot_number": "P-A02",
			"price":               "₱3,500/month",
			"description":         "Indoor parking slot, B1 Level Zone A, standard size, equipped with EV charging station. Location near elevator for easy access.",
			"contact":             "landlord5@demo.homex.ph",
		},
		UserID:     landlordUserIDs[4],
		ViewCount:  15,
		ReplyCount: 1,
		IsPinned:   false,
		IsLocked:   false,
		IsEdited:   false,
	}

	posts := []property.ForumPost{
		votePost,
		activityPost,
		helpPost,
		marketplacePost,
		socialPost,
		lockedVotePost,
		unitRentPost,
		parkingRentPost,
	}

	var postIDs []uint
	for _, post := range posts {
		if err := db.Create(&post).Error; err != nil {
			log.Printf("  ✗ Failed to create forum post: %v", err)
			continue
		}
		postIDs = append(postIDs, post.ID)
		log.Printf("  ✓ Created forum post: %s (ID: %d)", post.Title, post.ID)
	}

	// 创建回复
	if len(postIDs) > 0 {
		seedForumReplies(db, postIDs, tenantUserIDs, landlordUserIDs, adminID)
	}
}

// seedForumReplies 创建论坛回复
func seedForumReplies(db *gorm.DB, postIDs []uint, tenantUserIDs, landlordUserIDs []uint, adminID uint) {
	log.Println("Seeding forum replies...")

	if len(postIDs) < 3 {
		return
	}

	now := time.Now()

	replies := []property.ForumReply{
		// Reply to vote post
		{
			PostID:  postIDs[0],
			Content: "I voted for Movie Night, hoping we can organize an outdoor movie screening!",
			UserID:  tenantUserIDs[0],
		},
		{
			PostID:  postIDs[0],
			Content: "BBQ party sounds great, it can help build neighborly relationships.",
			UserID:  landlordUserIDs[0],
		},
		{
			PostID:  postIDs[0],
			Content: "I support sports competition, we can organize basketball or badminton matches.",
			UserID:  tenantUserIDs[1],
		},
		// Reply to activity post
		{
			PostID:  postIDs[1],
			Content: "Great! I've already signed up, looking forward to meeting everyone!",
			UserID:  tenantUserIDs[0],
		},
		{
			PostID:  postIDs[1],
			Content: "I'll bring some homemade snacks to share with everyone.",
			UserID:  landlordUserIDs[0],
		},
		{
			PostID:  postIDs[1],
			Content: "Need help preparing for the event? I can assist.",
			UserID:  tenantUserIDs[2],
		},
		{
			PostID:  postIDs[1],
			Content: "Thanks to the management for organizing, such events are very meaningful!",
			UserID:  landlordUserIDs[1],
		},
		// Reply to help post
		{
			PostID:  postIDs[2],
			Content: "I saw a set of keys near the parking lot, I've left them at the management office. You can go ask there.",
			UserID:  tenantUserIDs[2],
		},
		{
			PostID:  postIDs[2],
			Content: "Great! Thank you so much! I'll go check at the management office right away.",
			UserID:  tenantUserIDs[0],
		},
		// Reply to marketplace post
		{
			PostID:  postIDs[3],
			Content: "What are the dimensions of the refrigerator?",
			UserID:  tenantUserIDs[3],
		},
		// Reply to locked vote post
		{
			PostID:  postIDs[5],
			Content: "I voted for adding gym equipment, hoping for more exercise equipment.",
			UserID:  tenantUserIDs[0],
		},
		{
			PostID:  postIDs[5],
			Content: "Improving the parking lot is also important, parking spaces are a bit tight now.",
			UserID:  landlordUserIDs[0],
		},
	}

	for _, reply := range replies {
		// 设置创建时间（分散在不同时间）
		reply.CreatedAt = now.AddDate(0, 0, -int(len(replies))).Add(time.Duration(len(replies)) * time.Hour)

		if err := db.Create(&reply).Error; err != nil {
			log.Printf("  ✗ Failed to create forum reply: %v", err)
			continue
		}

		// 更新帖子的回复数
		db.Model(&property.ForumPost{}).Where("id = ?", reply.PostID).
			UpdateColumn("reply_count", gorm.Expr("reply_count + ?", 1))

		log.Printf("  ✓ Created forum reply for post %d", reply.PostID)
	}

	// 创建一些投票记录
	seedForumVotes(db, postIDs, tenantUserIDs, landlordUserIDs)
}

// seedForumVotes 创建投票记录
func seedForumVotes(db *gorm.DB, postIDs []uint, tenantUserIDs, landlordUserIDs []uint) {
	log.Println("Seeding forum votes...")

	if len(postIDs) < 1 {
		return
	}

	// 为第一个投票贴创建投票记录
	votePostID := postIDs[0]

	// // 解析投票选项
	// options := []string{"烧烤聚会", "电影之夜", "运动比赛", "才艺表演", "其他"}

	votes := []struct {
		userID uint
		option string
	}{
		{tenantUserIDs[0], "电影之夜"},
		{landlordUserIDs[0], "烧烤聚会"},
		{tenantUserIDs[1], "运动比赛"},
		{tenantUserIDs[2], "电影之夜"},
		{landlordUserIDs[1], "烧烤聚会"},
		{tenantUserIDs[3], "才艺表演"},
	}

	for _, v := range votes {
		optionsJSON, _ := json.Marshal([]string{v.option})
		vote := property.ForumVote{
			PostID:  votePostID,
			UserID:  v.userID,
			Options: string(optionsJSON),
		}

		if err := db.Create(&vote).Error; err != nil {
			log.Printf("  ✗ Failed to create forum vote: %v", err)
			continue
		}
		log.Printf("  ✓ Created vote: user %d voted for %s", v.userID, v.option)
	}
}

// seedFacilitiesAndReservations 公共设施与示例预约/审批数据
func seedFacilitiesAndReservations(db *gorm.DB, tenantUserIDs, landlordUserIDs []uint, staffID uint) {
	log.Println("Seeding facilities and sample facility reservations...")

	if len(tenantUserIDs) == 0 || len(landlordUserIDs) == 0 {
		log.Println("  - Not enough tenant/landlord users to seed facilities")
		return
	}

	now := time.Now()

	// Create some shared facilities
	facilities := []property.Facility{
		{
			Name:               "Billiard Room",
			Type:               property.FacilityTypeBilliardRoom,
			Description:        strPtr("Air-conditioned billiard room with 2 professional tables."),
			AvailableStartTime: strPtr("10:00:00"),
			AvailableEndTime:   strPtr("22:00:00"),
			Notice:             strPtr("Please handle equipment with care. No food on the table."),
		},
		{
			Name:               "Function Room",
			Type:               property.FacilityTypeMeetingRoom,
			Description:        strPtr("Multi-purpose function room suitable for small parties and meetings (up to 40 pax)."),
			AvailableStartTime: strPtr("09:00:00"),
			AvailableEndTime:   strPtr("21:00:00"),
			Notice:             strPtr("Cleaning fee may apply. Please restore the layout after use."),
		},
		{
			Name:               "Game Room",
			Type:               property.FacilityTypeGameRoom,
			Description:        strPtr("Game room with various board games and video games."),
			AvailableStartTime: strPtr("10:00:00"),
			AvailableEndTime:   strPtr("22:00:00"),
			Notice:             strPtr("Please return all games to their original location after use."),
		},
		{
			Name:               "Activity Room",
			Type:               property.FacilityTypeActivityRoom,
			Description:        strPtr("Multi-purpose activity room for various activities and events."),
			AvailableStartTime: strPtr("09:00:00"),
			AvailableEndTime:   strPtr("21:00:00"),
			Notice:             strPtr("Please clean up after your activity."),
		},
		{
			Name:               "Sky Lounge",
			Type:               property.FacilityTypeSkyLounge,
			Description:        strPtr("Rooftop sky lounge with city view."),
			AvailableStartTime: strPtr("17:00:00"),
			AvailableEndTime:   strPtr("23:00:00"),
			Notice:             strPtr("Noise control after 22:00. No fireworks or open flame."),
		},
	}

	var facilityIDs []uint
	for _, f := range facilities {
		if err := db.Create(&f).Error; err != nil {
			log.Printf("  ✗ Failed to create facility %s: %v", f.Name, err)
			continue
		}
		log.Printf("  ✓ Created facility: %s (ID: %d)", f.Name, f.ID)
		facilityIDs = append(facilityIDs, f.ID)
	}

	if len(facilityIDs) < 2 {
		return
	}

	// Sample reservations with different statuses
	startPending := now.AddDate(0, 0, 3).Truncate(time.Hour).Add(18 * time.Hour)  // 3 days later 18:00
	endPending := startPending.Add(2 * time.Hour)                                 // 2-hour slot
	startApproved := now.AddDate(0, 0, 1).Truncate(time.Hour).Add(14 * time.Hour) // tomorrow 14:00
	endApproved := startApproved.Add(2 * time.Hour)
	startCompleted := now.AddDate(0, 0, -1).Truncate(time.Hour).Add(19 * time.Hour) // yesterday 19:00
	endCompleted := startCompleted.Add(2 * time.Hour)

	// Pending reservation (待审批)
	pending := property.FacilityReservation{
		FacilityID: facilityIDs[0], // Billiard Room
		UserID:     tenantUserIDs[0],
		StartTime:  startPending,
		EndTime:    endPending,
		Status:     "pending",
		Notes:      strPtr("Weekend billiard session with friends."),
	}

	// Approved reservation (已确认，未来时间)
	approved := property.FacilityReservation{
		FacilityID: facilityIDs[1], // Function Room
		UserID:     tenantUserIDs[1],
		StartTime:  startApproved,
		EndTime:    endApproved,
		Status:     "approved",
		ApprovedBy: &staffID,
	}
	approvedAt := now.Add(-2 * time.Hour)
	approved.ApprovedAt = &approvedAt
	approved.Notes = strPtr("Birthday party, 20 guests.")

	// Completed reservation (已完成，过去时间)
	completed := property.FacilityReservation{
		FacilityID: facilityIDs[1], // Function Room
		UserID:     landlordUserIDs[0],
		StartTime:  startCompleted,
		EndTime:    endCompleted,
		Status:     "completed",
	}
	completedAt := endCompleted
	completed.CompletedAt = &completedAt
	completed.Notes = strPtr("Landlord meeting with tenants.")

	// Rejected reservation
	rejected := property.FacilityReservation{
		FacilityID: facilityIDs[0], // Billiard Room
		UserID:     tenantUserIDs[2],
		StartTime:  startApproved,
		EndTime:    endApproved,
		Status:     "rejected",
	}
	rejectedBy := staffID
	rejectedAt := now.Add(-1 * time.Hour)
	reason := "Time slot already reserved for maintenance."
	rejected.RejectedBy = &rejectedBy
	rejected.RejectedAt = &rejectedAt
	rejected.RejectedReason = &reason

	// Cancelled reservation (user side)
	cancelled := property.FacilityReservation{
		FacilityID: facilityIDs[0],
		UserID:     tenantUserIDs[3],
		StartTime:  startPending.AddDate(0, 0, 1),
		EndTime:    endPending.AddDate(0, 0, 1),
		Status:     "cancelled",
	}
	cancelledBy := tenantUserIDs[3]
	cancelledAt := now.Add(-30 * time.Minute)
	cancelled.CancelledBy = &cancelledBy
	cancelled.CancelledAt = &cancelledAt
	cancelled.Notes = strPtr("Changed plan, no longer needed.")

	reservations := []property.FacilityReservation{
		pending,
		approved,
		completed,
		rejected,
		cancelled,
	}

	for _, r := range reservations {
		if err := db.Create(&r).Error; err != nil {
			log.Printf("  ✗ Failed to create facility reservation (facility %d user %d): %v", r.FacilityID, r.UserID, err)
			continue
		}
		log.Printf("  ✓ Created facility reservation: facility=%d user=%d status=%s", r.FacilityID, r.UserID, r.Status)
	}
}

// seedMarketplace 创建服务市场和订单数据
func seedMarketplace(db *gorm.DB, unitIDs map[string]uint, tenantUserIDs, landlordUserIDs []uint, adminID, staffID uint) {
	log.Println("Seeding marketplace service listings and orders...")

	now := time.Now()
	adminIDPtr := &adminID

	// Create service listings
	serviceListings := []property.ServiceListing{
		// Property service - Renovation
		{
			ServiceType:       property.ServiceTypeRenovation,
			Title:             "Interior Renovation Service",
			Description:       strPtr("Professional interior renovation service, including walls, floors, ceilings, etc. Provides design consultation and construction services."),
			Price:             "50000.00",
			Currency:          "PHP",
			IsPropertyService: true,
			ThirdPartyContact: nil,
			Status:            property.ServiceListingStatusActive,
			CreatedBy:         adminIDPtr,
		},
		// Property service - Cleaning
		{
			ServiceType:       property.ServiceTypeCleaning,
			Title:             "Deep Cleaning Service",
			Description:       strPtr("Professional deep cleaning service, including whole house cleaning, window cleaning, carpet cleaning, etc."),
			Price:             "3000.00",
			Currency:          "PHP",
			IsPropertyService: true,
			ThirdPartyContact: nil,
			Status:            property.ServiceListingStatusActive,
			CreatedBy:         adminIDPtr,
		},
		// Property service - Repair
		{
			ServiceType:       property.ServiceTypeRepair,
			Title:             "Appliance Repair Service",
			Description:       strPtr("Professional appliance repair service, including maintenance and repair of air conditioners, refrigerators, washing machines, and other household appliances."),
			Price:             "2000.00",
			Currency:          "PHP",
			IsPropertyService: true,
			ThirdPartyContact: nil,
			Status:            property.ServiceListingStatusActive,
			CreatedBy:         adminIDPtr,
		},
		// Property service - Moving
		{
			ServiceType:       property.ServiceTypeMoving,
			Title:             "Moving Service",
			Description:       strPtr("Professional moving service, including packing, moving, furniture assembly and disassembly. Insurance service provided."),
			Price:             "8000.00",
			Currency:          "PHP",
			IsPropertyService: true,
			ThirdPartyContact: nil,
			Status:            property.ServiceListingStatusActive,
			CreatedBy:         adminIDPtr,
		},
		// Third-party service - Renovation
		{
			ServiceType:       property.ServiceTypeRenovation,
			Title:             "Premium Custom Renovation",
			Description:       strPtr("Premium custom renovation service with personalized design solutions. Please contact the third-party service provider for detailed quotes."),
			Price:             "0.00",
			Currency:          "PHP",
			IsPropertyService: false,
			ThirdPartyContact: strPtr("Phone: +63 2 8888 1234, Email: renovation@example.com"),
			Status:            property.ServiceListingStatusActive,
			CreatedBy:         adminIDPtr,
		},
		// Third-party service - Cleaning
		{
			ServiceType:       property.ServiceTypeCleaning,
			Title:             "Professional Carpet Cleaning",
			Description:       strPtr("Professional deep carpet cleaning service using imported equipment and cleaning agents. Please contact the service provider to book an appointment."),
			Price:             "0.00",
			Currency:          "PHP",
			IsPropertyService: false,
			ThirdPartyContact: strPtr("Phone: +63 2 8888 5678, WeChat: carpet_cleaning"),
			Status:            property.ServiceListingStatusActive,
			CreatedBy:         adminIDPtr,
		},
		// Property service - Other
		{
			ServiceType:       property.ServiceTypeOther,
			Title:             "Furniture Assembly Service",
			Description:       strPtr("Professional furniture assembly service, including assembly and installation of IKEA and other furniture brands."),
			Price:             "1500.00",
			Currency:          "PHP",
			IsPropertyService: true,
			ThirdPartyContact: nil,
			Status:            property.ServiceListingStatusActive,
			CreatedBy:         adminIDPtr,
		},
		// Discontinued service
		{
			ServiceType:       property.ServiceTypeRepair,
			Title:             "Plumbing Repair Service (Discontinued)",
			Description:       strPtr("This service has been suspended. Please use other repair services."),
			Price:             "2500.00",
			Currency:          "PHP",
			IsPropertyService: true,
			ThirdPartyContact: nil,
			Status:            property.ServiceListingStatusInactive,
			CreatedBy:         adminIDPtr,
		},
	}

	serviceListingIDs := make(map[int]uint)
	for i, listing := range serviceListings {
		if err := db.Create(&listing).Error; err != nil {
			log.Printf("  ✗ Failed to create service listing %s: %v", listing.Title, err)
			continue
		}
		log.Printf("  ✓ Created service listing: %s (ID: %d)", listing.Title, listing.ID)
		serviceListingIDs[i] = listing.ID
	}

	// 创建服务订单
	if len(unitIDs) == 0 || len(tenantUserIDs) == 0 {
		log.Println("  - Not enough units or tenants to seed service orders")
		return
	}

	// 获取一些unit和user的ID
	var unitNumbers []string
	for k := range unitIDs {
		unitNumbers = append(unitNumbers, k)
	}

	orders := []struct {
		ServiceListingIndex int
		UnitNumber          string
		UserIndex           int
		IsLandlord          bool
		Nickname            string
		Phone               string
		ServiceTime         time.Time
		Status              string
		AssignedStaffID     *uint
		CompletedAt         *time.Time
		ConfirmedAt         *time.Time
	}{
		// Order in service - Property service
		{0, "A-101", 0, false, "Mr. Zhang", "+63 912 345 6789", now.AddDate(0, 0, 2), property.ServiceOrderStatusInService, &staffID, nil, nil},
		// Order in service - Third-party service
		{4, "A-102", 1, false, "Ms. Li", "+63 912 345 6790", now.AddDate(0, 0, 3), property.ServiceOrderStatusInService, nil, nil, nil},
		// Completed order - Pending confirmation
		{1, "A-202", 2, false, "Mr. Wang", "+63 912 345 6791", now.AddDate(0, 0, -1), property.ServiceOrderStatusCompleted, &staffID, timePtr(now.AddDate(0, 0, -1).Add(4 * time.Hour)), nil},
		// Completed order - Confirmed
		{2, "B-101", 3, false, "Ms. Zhao", "+63 912 345 6792", now.AddDate(0, 0, -3), property.ServiceOrderStatusCompleted, &staffID, timePtr(now.AddDate(0, 0, -3).Add(5 * time.Hour)), timePtr(now.AddDate(0, 0, -2))},
		// Cancelled order
		{3, "B-201", 4, false, "Mr. Liu", "+63 912 345 6793", now.AddDate(0, 0, 5), property.ServiceOrderStatusCancelled, nil, nil, nil},
		// Landlord's order
		{1, "A-101", 0, true, "Mr. Chen", "+63 912 345 6794", now.AddDate(0, 0, 4), property.ServiceOrderStatusInService, &staffID, nil, nil},
	}

	orderCounter := 1
	for _, o := range orders {
		unitID, ok := unitIDs[o.UnitNumber]
		if !ok {
			continue
		}

		serviceListingID, ok := serviceListingIDs[o.ServiceListingIndex]
		if !ok {
			continue
		}

		var userID uint
		if o.IsLandlord {
			if o.UserIndex < len(landlordUserIDs) {
				userID = landlordUserIDs[o.UserIndex]
			}
		} else {
			if o.UserIndex < len(tenantUserIDs) {
				userID = tenantUserIDs[o.UserIndex]
			}
		}

		if userID == 0 {
			continue
		}

		// 生成订单号
		orderNumber := fmt.Sprintf("SO-%s-%04d", now.Format("20060102"), orderCounter)
		orderCounter++

		var cancelledAt *time.Time
		if o.Status == property.ServiceOrderStatusCancelled {
			cancelledAt = timePtr(now.AddDate(0, 0, -1))
		}

		order := property.ServiceOrder{
			OrderNumber:      orderNumber,
			ServiceListingID: serviceListingID,
			UnitID:           unitID,
			UserID:           userID,
			Nickname:         o.Nickname,
			Phone:            o.Phone,
			ServiceTime:      o.ServiceTime,
			Status:           o.Status,
			AssignedStaffID:  o.AssignedStaffID,
			CompletedAt:      o.CompletedAt,
			ConfirmedAt:      o.ConfirmedAt,
			CancelledAt:      cancelledAt,
		}

		if err := db.Create(&order).Error; err != nil {
			log.Printf("  ✗ Failed to create service order %s: %v", orderNumber, err)
			continue
		}

		log.Printf("  ✓ Created service order: %s (%s - %s)", orderNumber, o.Nickname, o.Status)
	}
}

func seedNotifications(db *gorm.DB, propertyID uint, tenantUserIDs, landlordUserIDs []uint, adminID uint) {
	log.Println("Seeding notifications...")

	now := time.Now()
	requestType := property.RelatedTypeRequest
	announcementType := property.RelatedTypeAnnouncement
	billType := property.RelatedTypeBill

	// Get some announcement IDs for notifications
	var announcements []property.Announcement
	db.Where("status = ?", "published").Limit(3).Find(&announcements)

	// Get some request IDs for notifications
	var requests []property.Request
	db.Limit(5).Find(&requests)

	// Get some bill IDs for notifications
	var bills []property.Bill
	db.Limit(3).Find(&bills)

	// Helper function to get announcement ID
	getAnnouncementID := func(index int) *uint {
		if index < len(announcements) {
			return &announcements[index].ID
		}
		return nil
	}

	// Helper function to get request ID
	getRequestID := func(index int) *uint {
		if index < len(requests) {
			return &requests[index].ID
		}
		return nil
	}

	// Helper function to get bill ID
	getBillID := func(index int) *uint {
		if index < len(bills) {
			return &bills[index].ID
		}
		return nil
	}

	notifications := []property.Notification{
		// Announcement notifications for tenants
		{
			UserID:      tenantUserIDs[0],
			PropertyID:  propertyID,
			Type:        property.NotificationTypeAnnouncementPublished,
			Title:       "New Announcement",
			Content:     "A new announcement \"Welcome to Demo Residence\" has been published.",
			RelatedID:   getAnnouncementID(0),
			RelatedType: &announcementType,
			IsRead:      false,
			CreatedAt:   now.AddDate(0, 0, -7),
		},
		{
			UserID:      tenantUserIDs[1],
			PropertyID:  propertyID,
			Type:        property.NotificationTypeAnnouncementPublished,
			Title:       "New Announcement",
			Content:     "A new announcement \"Scheduled Water Interruption\" has been published.",
			RelatedID:   getAnnouncementID(1),
			RelatedType: &announcementType,
			IsRead:      false,
			CreatedAt:   now.AddDate(0, 0, -5),
		},
		{
			UserID:      tenantUserIDs[2],
			PropertyID:  propertyID,
			Type:        property.NotificationTypeAnnouncementPublished,
			Title:       "New Announcement",
			Content:     "A new announcement \"Holiday Party Invitation\" has been published.",
			RelatedID:   getAnnouncementID(2),
			RelatedType: &announcementType,
			IsRead:      true,
			ReadAt:      timePtr(now.AddDate(0, 0, -3)),
			CreatedAt:   now.AddDate(0, 0, -4),
		},
		// Request status change notifications
		{
			UserID:      tenantUserIDs[0],
			PropertyID:  propertyID,
			Type:        property.NotificationTypeRequestStatusChange,
			Title:       "Request Status Updated",
			Content:     "Your request \"Request for gate pass\" is now in progress.",
			RelatedID:   getRequestID(0),
			RelatedType: &requestType,
			IsRead:      false,
			CreatedAt:   now.AddDate(0, 0, -2),
		},
		{
			UserID:      tenantUserIDs[1],
			PropertyID:  propertyID,
			Type:        property.NotificationTypeRequestStatusChange,
			Title:       "Request Status Updated",
			Content:     "Your request \"Parking sticker application\" status changed from pending to in_progress.",
			RelatedID:   getRequestID(1),
			RelatedType: &requestType,
			IsRead:      false,
			CreatedAt:   now.AddDate(0, 0, -1),
		},
		{
			UserID:      tenantUserIDs[2],
			PropertyID:  propertyID,
			Type:        property.NotificationTypeRequestStatusChange,
			Title:       "Request Status Updated",
			Content:     "Your request \"Move-in inspection request\" has been rejected.",
			RelatedID:   getRequestID(2),
			RelatedType: &requestType,
			IsRead:      true,
			ReadAt:      timePtr(now.AddDate(0, 0, -1)),
			CreatedAt:   now.AddDate(0, 0, -3),
		},
		// Bill generated notifications
		{
			UserID:      tenantUserIDs[0],
			PropertyID:  propertyID,
			Type:        property.NotificationTypeBillGenerated,
			Title:       "New Bill Generated",
			Content:     "A new bill has been generated for your unit. Please check your bills section for details.",
			RelatedID:   getBillID(0),
			RelatedType: &billType,
			IsRead:      false,
			CreatedAt:   now.AddDate(0, 0, -10),
		},
		{
			UserID:      tenantUserIDs[1],
			PropertyID:  propertyID,
			Type:        property.NotificationTypeBillGenerated,
			Title:       "New Bill Generated",
			Content:     "A new bill \"January 2025 Rent\" has been generated. Due date: " + now.AddDate(0, 0, 15).Format("January 2, 2006"),
			RelatedID:   getBillID(1),
			RelatedType: &billType,
			IsRead:      false,
			CreatedAt:   now.AddDate(0, 0, -8),
		},
		// Payment received notifications
		{
			UserID:      tenantUserIDs[0],
			PropertyID:  propertyID,
			Type:        property.NotificationTypePaymentReceived,
			Title:       "Payment Received",
			Content:     "Your payment of PHP 25,000.00 has been received and processed successfully.",
			RelatedID:   nil,
			RelatedType: nil,
			IsRead:      true,
			ReadAt:      timePtr(now.AddDate(0, 0, -5)),
			CreatedAt:   now.AddDate(0, 0, -6),
		},
		// Landlord notifications
		{
			UserID:      landlordUserIDs[0],
			PropertyID:  propertyID,
			Type:        property.NotificationTypeAnnouncementPublished,
			Title:       "New Announcement",
			Content:     "A new announcement \"New Parking Policy\" has been published.",
			RelatedID:   getAnnouncementID(0),
			RelatedType: &announcementType,
			IsRead:      false,
			CreatedAt:   now.AddDate(0, 0, -4),
		},
		{
			UserID:      landlordUserIDs[1],
			PropertyID:  propertyID,
			Type:        property.NotificationTypeRequestStatusChange,
			Title:       "Request Status Updated",
			Content:     "Your request \"Move-out request\" is now in progress.",
			RelatedID:   getRequestID(1),
			RelatedType: &requestType,
			IsRead:      false,
			CreatedAt:   now.AddDate(0, 0, -2),
		},
	}

	for _, notif := range notifications {
		if err := db.Create(&notif).Error; err != nil {
			log.Printf("  ✗ Failed to create notification: %v", err)
			continue
		}

		log.Printf("  ✓ Created notification: %s (User: %d, Read: %v)", notif.Title, notif.UserID, notif.IsRead)
	}
}
