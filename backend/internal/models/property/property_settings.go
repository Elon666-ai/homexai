package property

import (
	"time"
)

// PropertySettings 物业设置表
type PropertySettings struct {
	ID        uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	SettingKey string  `gorm:"column:setting_key;type:varchar(100);not null;uniqueIndex:uk_setting_key" json:"setting_key" default:"" comment:"设置键"`
	SettingValue string `gorm:"column:setting_value;type:json" json:"setting_value" default:"" comment:"设置值(JSON格式)"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`
}

func (PropertySettings) TableName() string {
	return "property_settings"
}

// Setting key constants
const (
	SettingKeyPaymentMethods = "payment_methods"
)

