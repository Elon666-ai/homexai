package master

import (
	"time"
)

// NotificationLog 通知发送记录表
type NotificationLog struct {
	ID               uint64     `gorm:"column:id;type:bigint unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UserID           *uint      `gorm:"column:user_id;type:int unsigned;index:idx_user_id" json:"user_id" default:"null" comment:"用户ID"`
	PropertyID       *uint      `gorm:"column:property_id;type:int unsigned;index:idx_property_id" json:"property_id" default:"null" comment:"关联物业ID"`
	NotificationType string     `gorm:"column:notification_type;type:varchar(20);not null;index:idx_notification_type" json:"notification_type" default:"" comment:"通知类型(email,sms,push)"`
	Recipient        string     `gorm:"column:recipient;type:varchar(255);not null" json:"recipient" default:"" comment:"接收者"`
	Subject          *string    `gorm:"column:subject;type:varchar(500)" json:"subject" default:"null" comment:"主题"`
	Content          *string    `gorm:"column:content;type:text" json:"content" default:"null" comment:"内容"`
	Language         string     `gorm:"column:language;type:varchar(10);not null" json:"language" default:"en" comment:"语言"`
	Status           string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"pending" comment:"发送状态(pending,sent,failed,bounced)"`
	ErrorMessage     *string    `gorm:"column:error_message;type:text" json:"error_message" default:"null" comment:"错误信息"`
	SentAt           *time.Time `gorm:"column:sent_at;type:datetime" json:"sent_at" default:"null" comment:"发送时间"`
	CreatedAt        time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime;index:idx_created_at" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
}

func (NotificationLog) TableName() string {
	return "notification_logs"
}

// Notification type constants
const (
	NotificationTypeEmail = "email"
	NotificationTypeSMS   = "sms"
	NotificationTypePush  = "push"
)

// Notification status constants
const (
	NotificationStatusPending = "pending"
	NotificationStatusSent    = "sent"
	NotificationStatusFailed  = "failed"
	NotificationStatusBounced = "bounced"
)
