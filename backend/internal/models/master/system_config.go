package master

import (
	"time"
)

// SystemConfig 系统配置表
type SystemConfig struct {
	ID          uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	ConfigKey   string    `gorm:"column:config_key;type:varchar(100);not null;uniqueIndex:uk_config_key" json:"config_key" default:"" comment:"配置键"`
	ConfigValue *string   `gorm:"column:config_value;type:text" json:"config_value" default:"null" comment:"配置值"`
	ConfigType  string    `gorm:"column:config_type;type:varchar(20);not null" json:"config_type" default:"string" comment:"配置类型(string,number,boolean,json)"`
	Description *string   `gorm:"column:description;type:text" json:"description" default:"null" comment:"配置描述"`
	IsPublic    bool      `gorm:"column:is_public;type:tinyint(1);not null;index:idx_is_public" json:"is_public" default:"0" comment:"是否公开(前端可访问)"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`
}

func (SystemConfig) TableName() string {
	return "system_configs"
}
