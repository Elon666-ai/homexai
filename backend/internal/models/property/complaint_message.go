package property

import (
	"time"
)

// ComplaintMessage represents a message in a complaint conversation
type ComplaintMessage struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ComplaintID uint      `gorm:"column:complaint_id;not null;index" json:"complaint_id"`
	UserID      uint      `gorm:"column:user_id;not null;index" json:"user_id"`
	Message     string    `gorm:"column:message;type:text;not null" json:"message"`
	IsFromStaff bool      `gorm:"column:is_from_staff;not null;default:false" json:"is_from_staff"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`

	// Relations - Note: User relation removed to avoid cross-database references
	// User information should be fetched from master database when needed
	Complaint *Complaint `gorm:"foreignKey:ComplaintID" json:"complaint,omitempty"`
}

func (ComplaintMessage) TableName() string {
	return "complaint_messages"
}
