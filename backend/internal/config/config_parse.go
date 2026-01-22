package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

var Yaml ConfigYaml

type ConfigYaml struct {
	// to avoid name conflict with other structs
	Redis     RedisConf     `mapstructure:"redis"`
	JWT       JWTConf       `mapstructure:"jwt"`
	Upload    UploadConf    `mapstructure:"upload"`
	Email     EmailConf     `mapstructure:"email"`
	SMS       SMSConf       `mapstructure:"sms"`
	OAuth     OAuthConf     `mapstructure:"oauth"`
	CORS      CORSConf      `mapstructure:"cors"`
	RateLimit RateLimitConf `mapstructure:"rate_limit"`

	Server     ServerConfig     `mapstructure:"server"`
	Database   MysqlConfig      `mapstructure:"database"`
	Turnstile  TurnstileConfig  `mapstructure:"turnstile"`
	Log        LogConfig        `mapstructure:"log"`
	Cache      CacheConfig      `mapstructure:"cache"`
	Tenant     TenantConfig     `mapstructure:"tenant"`
	Queue      QueueConfig      `mapstructure:"queue"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
	Features   FeaturesConfig   `mapstructure:"features"`
}

/* -------------------- Server -------------------- */

type ServerConfig struct {
	AppName        string        `mapstructure:"app_name"`
	AppDebug       bool          `mapstructure:"app_debug"`
	FrontendURL    string        `mapstructure:"frontend_url"`
	Port           int           `mapstructure:"port"`
	Mode           string        `mapstructure:"mode"`
	Host           string        `mapstructure:"host"`
	ReadTimeout    time.Duration `mapstructure:"read_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	MaxHeaderBytes int           `mapstructure:"max_header_bytes"`
}

/* -------------------- Database -------------------- */

type MysqlConfig struct {
	Driver          string        `mapstructure:"driver"`
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	MasterDb        string        `mapstructure:"master_db"`
	Charset         string        `mapstructure:"charset"`
	ParseTime       bool          `mapstructure:"parse_time"`
	Loc             string        `mapstructure:"loc"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	ShardCount      int           `mapstructure:"shard_count"`
	LogLevel        string        `mapstructure:"log_level"`
	SlowThreshold   time.Duration `mapstructure:"slow_threshold"`
}

/* -------------------- Redis -------------------- */

type RedisConf struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	MaxRetries   int           `mapstructure:"max_retries"`
	PoolTimeout  time.Duration `mapstructure:"pool_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	KeyPrefix    string        `mapstructure:"key_prefix"`
}

type TurnstileConfig struct {
	Enable     bool   `mapstructure:"enable"`
	SiteKey    string `mapstructure:"site_key"`
	SiteSecret string `mapstructure:"site_secret"`
}

/* -------------------- JWT -------------------- */

type JWTConf struct {
	Secret             string `mapstructure:"secret"`
	ExpireHours        int    `mapstructure:"expire_hours"`
	RefreshExpireHours int    `mapstructure:"refresh_expire_hours"`
}

/* -------------------- Log -------------------- */

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxAge     int    `mapstructure:"max_age"`
	MaxBackups int    `mapstructure:"max_backups"`
	Compress   bool   `mapstructure:"compress"`
}

/* -------------------- Upload -------------------- */

type UploadConf struct {
	Path         string   `mapstructure:"path"`
	MaxSize      int64    `mapstructure:"max_size"`
	AllowedTypes []string `mapstructure:"allowed_types"`
}

/* -------------------- Email -------------------- */

type EmailConf struct {
	SMTPHost       string `mapstructure:"smtp_host"`
	SMTPPort       int    `mapstructure:"smtp_port"`
	FromEmail      string `mapstructure:"from_email"`
	FromName       string `mapstructure:"from_name"`
	Username       string `mapstructure:"username"`
	Password       string `mapstructure:"password"`
	UseTLS         bool   `mapstructure:"use_tls"`
	SendGridAPIKey string `mapstructure:"sender_grid_api_key"`
}

/* -------------------- SMS -------------------- */

type SMSConf struct {
	Provider   string `mapstructure:"provider"`
	AccountSID string `mapstructure:"account_sid"`
	AuthToken  string `mapstructure:"auth_token"`
	FromNumber string `mapstructure:"from_number"`
}

/* -------------------- OAuth -------------------- */

type OAuthConf struct {
	Google   OAuthGoogleConfig `mapstructure:"google"`
	Facebook OAuthGoogleConfig `mapstructure:"facebook"`
}

type OAuthGoogleConfig struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

/* -------------------- CORS -------------------- */

type CORSConf struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	ExposeHeaders    []string `mapstructure:"expose_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

/* -------------------- Rate Limit -------------------- */

type RateLimitConf struct {
	Enabled           bool `mapstructure:"enabled"`
	RequestsPerSecond int  `mapstructure:"requests_per_second"`
	Burst             int  `mapstructure:"burst"`
}

/* -------------------- Cache -------------------- */

type CacheConfig struct {
	DefaultTTL  int `mapstructure:"default_ttl"`
	PropertyTTL int `mapstructure:"property_ttl"`
	UserTTL     int `mapstructure:"user_ttl"`
}

/* -------------------- Tenant -------------------- */

type TenantConfig struct {
	DomainMapping     map[string]int `mapstructure:"domain_mapping"`
	DefaultPropertyID int            `mapstructure:"default_property_id"`
}

/* -------------------- Queue -------------------- */

type QueueConfig struct {
	Enabled     bool              `mapstructure:"enabled"`
	WorkerCount int               `mapstructure:"worker_count"`
	MaxRetry    int               `mapstructure:"max_retry"`
	Queues      map[string]string `mapstructure:"queues"`
}

/* -------------------- Monitoring -------------------- */

type MonitoringConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	PrometheusPort int    `mapstructure:"prometheus_port"`
	HealthCheck    string `mapstructure:"health_check_path"`
	MetricsPath    string `mapstructure:"metrics_path"`
}

/* -------------------- Features -------------------- */

type FeaturesConfig struct {
	OAuthEnabled           bool `mapstructure:"oauth_enabled"`
	SMSEnabled             bool `mapstructure:"sms_enabled"`
	EmailEnabled           bool `mapstructure:"email_enabled"`
	ImageProcessingEnabled bool `mapstructure:"image_processing_enabled"`
	AuditLogEnabled        bool `mapstructure:"audit_log_enabled"`
	WebsocketEnabled       bool `mapstructure:"websocket_enabled"`
}

/* -------------------- Init -------------------- */
// helper: 递归扁平化可能被拆开的域名映射
// input: an interface{} read from viper.Get("tenant.domain_mapping")
// output: map[string]int (only keeps entries that are numeric)
func flattenDomainMapping(prefix string, raw interface{}, out map[string]int) {
	switch m := raw.(type) {
	case map[string]interface{}:
		for k, v := range m {
			var newPrefix string
			if prefix == "" {
				newPrefix = k
			} else {
				newPrefix = prefix + "." + k
			}
			// 如果值还是 map，递归；否则尝试把值转换为整数并写入 out
			switch vv := v.(type) {
			case map[string]interface{}:
				flattenDomainMapping(newPrefix, vv, out)
			case int:
				out[newPrefix] = vv
			case int64:
				out[newPrefix] = int(vv)
			case float64:
				// YAML 数字有时被解析为 float64
				out[newPrefix] = int(vv)
			case string:
				// 如果值是字符串，尝试解析为整数（防御性处理）
				// 不引入 strconv 解析失败就跳过
				// （若需要可选择解析）
			default:
				// 不支持的类型，忽略
			}
		}
	default:
		// 如果是其它类型（不常见），就不做处理
	}
}

func InitConfig(confFile string) error {
	viper.SetConfigType("yaml")
	viper.SetConfigName(confFile)
	viper.AddConfigPath("./conf")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("Load config failure. %w", err)
	}

	// 规范化 tenant.domain_mapping，兼容多种写法（直接 map[string]int / 嵌套 map 等）
	rawDM := viper.Get("tenant.domain_mapping")
	if rawDM != nil {
		out := make(map[string]int)
		// 如果 rawDM 本身就是 map[string]int (少见，因为 viper 解析通常为 map[string]interface{})
		switch t := rawDM.(type) {
		case map[string]int:
			// 直接复制
			for k, v := range t {
				out[k] = v
			}
		case map[string]interface{}:
			flattenDomainMapping("", t, out)
		default:
			// 其它类型忽略
		}
		// 覆写回 viper，确保 Unmarshal 时为 map[string]int 语义
		viper.Set("tenant.domain_mapping", out)
	}

	if err := viper.Unmarshal(&Yaml); err != nil {
		return fmt.Errorf("parse %s failure! %w", viper.GetViper().ConfigFileUsed(), err)
	}

	// 自动热更新
	//viper.WatchConfig()

	return nil
}

// GetMasterDSN returns the DSN for master database with enhanced parameters
// ✅ 修复：添加超时和保活参数
func (c *ConfigYaml) GetMasterDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s&readTimeout=30s&writeTimeout=30s&maxAllowedPacket=0&interpolateParams=true",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.MasterDb,
	)
}

// GetPropertyDSN returns the DSN for property database with enhanced parameters
// ✅ 修复：添加超时和保活参数
func (c *ConfigYaml) GetPropertyDSN(dbName string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s&readTimeout=30s&writeTimeout=30s&maxAllowedPacket=0&interpolateParams=true",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		dbName,
	)
}

func (c *ConfigYaml) GetDatabasePassword() string {
	return c.Database.Password
}
