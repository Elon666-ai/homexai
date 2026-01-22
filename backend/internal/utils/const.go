package utils

const (
	APP_NAME = "homexai"
	VERSION  = "v1.2.1"
)

var versionModification = map[string]string{
	"v1.2.1": "业务逻辑:✅ 一个owner可以有多个unit和多个parking✅ 一个tenant可以同时租多个unit和多个parking✅ 一个spa可以同时代管多个unit和多个parking✅ 一个unit可以有多个landlord，spa和tenant",
	"v1.2.0": "1. 系统针对 tenant/landlord/spa 用户的移动端使用体验进行了优化。",
	"v1.1.1": "1. 投诉与咨询可以多次来回对话功能，只有投诉/咨询发起者才能主动关闭对话。",
	"v1.0.9": "1. Model重构：所有字段包含gorm,json,default,comment标签. 2. 移除单独的SQL建表文件，采用GORM AutoMigrate建表. 3. 新增SystemConfig,ImportTask,SystemAuditLog,UserSession,NotificationLog模型. ",
	"v1.0.8": "1. property_admin账号登录后的功能菜单完善. ",
	"v1.0.7": "1. import excel as admin. 2.完善账号登录逻辑",
	"v1.0.6": "1. 不同角色登录显示不同菜单. ",
	"v1.0.5": "1. 增加一个新角色：SPA。SPA享有和RoleLandlord完全相同的权限。2. super_admin角色创建一个property。3. property_admin角色，导入property.xlsx。 ",
	"v1.0.4": "1. 支持property_admin管理menu. ",
	"v1.0.3": "1. deploy to pp-cdn.org. ",
	"v1.0.2": "1. add support TURNSTILE. ",
	"v1.0.0": `init release`,
}

func GetVersionModification() string {
	return versionModification[VERSION]
}

const (
	CONF_FILE              = "conf/homexai.json"
	DB_NAME                = "homexai"
	RESET_LOGFILE          = "./resetlog.txt"
	PID_LOGFILE            = "./pid.txt"
	ENV_FILE               = ".env"
	DEFAULT_USER_PASSWORD  = "Home1234!"
	DEFAULT_ADMIN_PASSWORD = "Admin1234!"
)
