package property

import (
	"time"
)

// Notification 用户通知表（物业数据库）
type Notification struct {
	ID          uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UserID      uint       `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" default:"" comment:"用户ID(主库users表)"`
	PropertyID uint       `gorm:"column:property_id;type:int unsigned;not null;index:idx_property_id" json:"property_id" default:"" comment:"关联物业ID"`
	Type        string     `gorm:"column:type;type:varchar(50);not null;index:idx_type" json:"type" default:"" comment:"通知类型(request_status_change,announcement_published,etc)"`
	Title       string     `gorm:"column:title;type:varchar(200);not null" json:"title" default:"" comment:"通知标题"`
	Content     string     `gorm:"column:content;type:text;not null" json:"content" default:"" comment:"通知内容"`
	RelatedID   *uint      `gorm:"column:related_id;type:int unsigned;index:idx_related" json:"related_id" default:"null" comment:"关联实体ID(如request_id,announcement_id)"`
	RelatedType *string    `gorm:"column:related_type;type:varchar(50);index:idx_related" json:"related_type" default:"null" comment:"关联实体类型(request,announcement,etc)"`
	IsRead      bool       `gorm:"column:is_read;type:tinyint(1);not null;index:idx_is_read" json:"is_read" default:"0" comment:"是否已读"`
	ReadAt      *time.Time `gorm:"column:read_at;type:datetime" json:"read_at" default:"null" comment:"阅读时间"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime;index:idx_created_at" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`
}

func (Notification) TableName() string {
	return "notifications"
}

// Notification type constants
const (
	NotificationTypeRequestStatusChange = "request_status_change" // 申请状态变化
	NotificationTypeAnnouncementPublished = "announcement_published" // 公告发布
	NotificationTypeBillGenerated = "bill_generated" // 账单生成
	NotificationTypePaymentReceived = "payment_received" // 付款收到
	NotificationTypeInquiryAnswered = "inquiry_answered" // 询问已回复
	NotificationTypeComplaintResolved = "complaint_resolved" // 投诉已解决
)

// Related type constants
const (
	RelatedTypeRequest = "request"
	RelatedTypeAnnouncement = "announcement"
	RelatedTypeBill = "bill"
	RelatedTypePayment = "payment"
	RelatedTypeInquiry = "inquiry"
	RelatedTypeComplaint = "complaint"
)

