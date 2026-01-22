package master

import (
	"fmt"
	"time"
)

// PropertyDBMapping 物业数据库映射表
type PropertyDBMapping struct {
	ID                  uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	PropertyID          uint      `gorm:"column:property_id;type:int unsigned;not null;uniqueIndex:uk_property_id" json:"property_id" default:"" comment:"物业ID"`
	Subdomain           string    `gorm:"column:subdomain;type:varchar(50);not null;uniqueIndex:uk_subdomain" json:"subdomain" default:"" comment:"子域名"`
	DBHost              string    `gorm:"column:db_host;type:varchar(255);not null" json:"db_host" default:"localhost" comment:"数据库主机"`
	DBPort              int       `gorm:"column:db_port;type:int;not null" json:"db_port" default:"3306" comment:"数据库端口"`
	DBName              string    `gorm:"column:db_name;type:varchar(100);not null" json:"db_name" default:"" comment:"数据库名称"`
	DBUser              string    `gorm:"column:db_user;type:varchar(100);not null" json:"db_user" default:"" comment:"数据库用户"`
	DBPasswordEncrypted string    `gorm:"column:db_password_encrypted;type:varchar(500);not null" json:"-" default:"" comment:"加密的数据库密码"`
	IsActive            bool      `gorm:"column:is_active;type:tinyint(1);not null;index:idx_is_active" json:"is_active" default:"1" comment:"是否激活"`
	MaxConnections      int       `gorm:"column:max_connections;type:int;not null" json:"max_connections" default:"50" comment:"最大连接数"`
	CreatedAt           time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt           time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Property Property `gorm:"foreignKey:PropertyID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"property,omitempty"`
}

func (PropertyDBMapping) TableName() string {
	return "property_db_mappings"
}

// GetDSN 获取数据库连接字符串
func (m *PropertyDBMapping) GetDSN(password string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		m.DBUser, password, m.DBHost, m.DBPort, m.DBName)
}
