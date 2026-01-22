package property

import (
	"time"
)

// Inquiry represents a user inquiry/question
type Inquiry struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"column:user_id;not null;index" json:"user_id"`
	UnitID      *uint      `gorm:"column:unit_id;index" json:"unit_id"`
	ParkingID   *uint      `gorm:"column:parking_id;index" json:"parking_id"`
	Title       string     `gorm:"column:title;type:varchar(255);not null" json:"title"`
	Description *string    `gorm:"column:description;type:text" json:"description"`
	Status      string     `gorm:"column:status;type:varchar(50);not null;default:'pending'" json:"status"` // pending, answered, closed
	Response    *string    `gorm:"column:response;type:text" json:"response"`
	RespondedBy *uint      `gorm:"column:responded_by" json:"responded_by"`
	RespondedAt *time.Time `gorm:"column:responded_at" json:"responded_at"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`

	// Relations
	Unit    *Unit    `gorm:"foreignKey:UnitID" json:"unit,omitempty"`
	Parking *ParkingSlot `gorm:"foreignKey:ParkingID" json:"parking,omitempty"`
}

func (Inquiry) TableName() string {
	return "inquiries"
}

// Inquiry status constants
const (
	InquiryStatusPending  = "pending"
	InquiryStatusAnswered = "answered"
	InquiryStatusClosed   = "closed"
)
