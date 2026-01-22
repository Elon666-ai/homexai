package master

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// ForumAd 论坛广告表（B2B系统，存储在master库）
type ForumAd struct {
	ID             uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	AdType         string     `gorm:"column:ad_type;type:varchar(50);not null;index:idx_ad_type" json:"ad_type" default:"" comment:"广告类型(pinned,official,merchant)"`
	Title          string     `gorm:"column:title;type:varchar(200);not null" json:"title" default:"" comment:"标题"`
	Content        string     `gorm:"column:content;type:text;not null" json:"content" default:"" comment:"内容"`
	Status         string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"inactive" comment:"状态(active,inactive)"`
	TargetProperties JSONB    `gorm:"column:target_properties;type:text" json:"target_properties" default:"null" comment:"目标物业ID数组(JSON),null表示所有"`
	TargetCities    JSONB     `gorm:"column:target_cities;type:text" json:"target_cities" default:"null" comment:"目标城市数组(JSON),null表示所有"`
	TargetTags      JSONB     `gorm:"column:target_tags;type:text" json:"target_tags" default:"null" comment:"目标人群标签数组(JSON),null表示所有"`
	CreatedBy      uint       `gorm:"column:created_by;type:int unsigned;not null;index:idx_created_by" json:"created_by" default:"" comment:"创建者用户ID(super_admin)"`
	CreatedAt      time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime;index:idx_created_at" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`
	StartedAt      *time.Time `gorm:"column:started_at;type:datetime;index:idx_started_at" json:"started_at" default:"null" comment:"上架时间"`
	EndedAt        *time.Time `gorm:"column:ended_at;type:datetime;index:idx_ended_at" json:"ended_at" default:"null" comment:"下架时间"`
}

func (ForumAd) TableName() string {
	return "forum_ads"
}

// Ad type constants
const (
	ForumAdTypePinned   = "pinned"   // 置顶贴
	ForumAdTypeOfficial = "official" // 官方公告贴
	ForumAdTypeMerchant = "merchant" // 本地商户推广
)

// Ad status constants
const (
	ForumAdStatusActive   = "active"   // 上架
	ForumAdStatusInactive = "inactive" // 下架
)

// JSONB 用于存储JSON数组
type JSONB []interface{}

// Value 实现driver.Valuer接口
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现sql.Scanner接口
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

