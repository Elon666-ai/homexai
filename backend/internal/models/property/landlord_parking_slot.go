package property

import (
	"time"
)

// LandlordParkingSlot 业主-停车位关联表（一个业主可以拥有多个停车位）
type LandlordParkingSlot struct {
	ID                  uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UserID              uint       `gorm:"column:user_id;type:int unsigned;not null;uniqueIndex:uk_user_parking,priority:1;index:idx_user_id" json:"user_id" default:"" comment:"业主用户ID(主库users表)"`
	ParkingSlotID       uint       `gorm:"column:parking_slot_id;type:int unsigned;not null;uniqueIndex:uk_user_parking,priority:2;index:idx_parking_slot_id" json:"parking_slot_id" default:"" comment:"停车位ID"`
	OwnershipType       string     `gorm:"column:ownership_type;type:varchar(20);not null" json:"ownership_type" default:"full" comment:"所有权类型(full,partial)"`
	OwnershipPercentage *string    `gorm:"column:ownership_percentage;type:decimal(5,2)" json:"ownership_percentage" default:"100.00" comment:"所有权比例"`
	OwnershipStartDate  *time.Time `gorm:"column:ownership_start_date;type:date" json:"ownership_start_date" default:"null" comment:"所有权开始日期"`
	OwnershipEndDate    *time.Time `gorm:"column:ownership_end_date;type:date" json:"ownership_end_date" default:"null" comment:"所有权结束日期"`
	PurchasePrice       *string    `gorm:"column:purchase_price;type:decimal(12,2)" json:"purchase_price" default:"null" comment:"购买价格"`
	ContractNumber      *string    `gorm:"column:contract_number;type:varchar(100)" json:"contract_number" default:"null" comment:"合同编号"`
	Notes               *string    `gorm:"column:notes;type:text" json:"notes" default:"null" comment:"备注"`
	CreatedAt           time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	ParkingSlot ParkingSlot `gorm:"foreignKey:ParkingSlotID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"parking_slot,omitempty"`
}

func (LandlordParkingSlot) TableName() string {
	return "landlord_parking_slots"
}

// Ownership type constants (与 Landlord 表保持一致)
const (
	ParkingOwnershipTypeFull    = "full"
	ParkingOwnershipTypePartial = "partial"
)

// IsCurrentOwner 检查是否是当前有效的所有者
func (l *LandlordParkingSlot) IsCurrentOwner() bool {
	now := time.Now()
	if l.OwnershipStartDate != nil && now.Before(*l.OwnershipStartDate) {
		return false
	}
	if l.OwnershipEndDate != nil && now.After(*l.OwnershipEndDate) {
		return false
	}
	return true
}
