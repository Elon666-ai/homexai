package property

import (
	"time"
)

// Complaint represents a user complaint
type Complaint struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"column:user_id;not null;index" json:"user_id"`
	UnitID      *uint     `gorm:"column:unit_id;index" json:"unit_id"`
	ParkingID   *uint     `gorm:"column:parking_id;index" json:"parking_id"`
	Title       string    `gorm:"column:title;type:varchar(255);not null" json:"title"`
	Description *string   `gorm:"column:description;type:text" json:"description"`
	Priority    string    `gorm:"column:priority;type:varchar(50);not null;default:'normal'" json:"priority"` // low, normal, high, urgent
	Status      string    `gorm:"column:status;type:varchar(50);not null;default:'open'" json:"status"`       // open, closed
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`

	// Relations
	Unit    *Unit        `gorm:"foreignKey:UnitID" json:"unit,omitempty"`
	Parking *ParkingSlot `gorm:"foreignKey:ParkingID" json:"parking,omitempty"`
}

func (Complaint) TableName() string {
	return "complaints"
}

// Complaint status constants
const (
	ComplaintStatusOpen   = "open"
	ComplaintStatusClosed = "closed"
)

// Complaint priority constants
const (
	ComplaintPriorityLow    = "low"
	ComplaintPriorityNormal = "normal"
	ComplaintPriorityHigh   = "high"
	ComplaintPriorityUrgent = "urgent"
)
