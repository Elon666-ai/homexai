package property

import (
	"time"
)

// Announcement 公告表
type Announcement struct {
	ID          uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	Title       string     `gorm:"column:title;type:varchar(200);not null" json:"title" default:"" comment:"标题"`
	Content     string     `gorm:"column:content;type:text;not null" json:"content" default:"" comment:"内容"`
	Category    string     `gorm:"column:category;type:varchar(50);not null;index:idx_category" json:"category" default:"general" comment:"分类(general,maintenance,emergency,event,policy,other)"`
	Priority    string     `gorm:"column:priority;type:varchar(20);not null;index:idx_priority" json:"priority" default:"normal" comment:"优先级(low,normal,high,urgent)"`
	Status      string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"draft" comment:"状态(draft,published,archived)"`
	PublishedAt *time.Time `gorm:"column:published_at;type:datetime" json:"published_at" default:"null" comment:"发布时间"`
	ExpiresAt   *time.Time `gorm:"column:expires_at;type:datetime;index:idx_expires_at" json:"expires_at" default:"null" comment:"过期时间"`
	CreatedBy   uint       `gorm:"column:created_by;type:int unsigned;not null;index:idx_created_by" json:"created_by" default:"" comment:"创建者用户ID"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`
}

func (Announcement) TableName() string {
	return "announcements"
}

// Announcement category constants
const (
	AnnouncementCategoryGeneral     = "general"
	AnnouncementCategoryMaintenance = "maintenance"
	AnnouncementCategoryEmergency   = "emergency"
	AnnouncementCategoryEvent       = "event"
	AnnouncementCategoryPolicy      = "policy"
	AnnouncementCategoryOther       = "other"
)

// Announcement priority constants
const (
	AnnouncementPriorityLow    = "low"
	AnnouncementPriorityNormal = "normal"
	AnnouncementPriorityHigh   = "high"
	AnnouncementPriorityUrgent = "urgent"
)

// Announcement status constants
const (
	AnnouncementStatusDraft     = "draft"
	AnnouncementStatusPublished = "published"
	AnnouncementStatusArchived  = "archived"
)

// IsPublished 检查公告是否已发布
func (a *Announcement) IsPublished() bool {
	return a.Status == AnnouncementStatusPublished
}

// IsDraft 检查公告是否是草稿
func (a *Announcement) IsDraft() bool {
	return a.Status == AnnouncementStatusDraft
}

// IsExpired 检查公告是否已过期
func (a *Announcement) IsExpired() bool {
	if a.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*a.ExpiresAt)
}

// IsActive 检查公告是否有效(已发布且未过期)
func (a *Announcement) IsActive() bool {
	return a.IsPublished() && !a.IsExpired()
}
