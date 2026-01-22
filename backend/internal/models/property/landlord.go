package property

import (
	"time"
)

// Landlord 业主-房源关联表（一个业主可以拥有多个房源）
// 表名保持 landlords 以兼容现有代码，实际上是 landlord_units 的概念
// 一个 UserID 可以有多条记录，关联多个 UnitID
type Landlord struct {
	ID                  uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UserID              uint       `gorm:"column:user_id;type:int unsigned;not null;uniqueIndex:uk_user_unit,priority:1;index:idx_user_id" json:"user_id" default:"" comment:"业主用户ID(主库users表)"`
	UnitID              uint       `gorm:"column:unit_id;type:int unsigned;not null;uniqueIndex:uk_user_unit,priority:2;index:idx_unit_id" json:"unit_id" default:"" comment:"房源ID"`
	OwnershipType       string     `gorm:"column:ownership_type;type:varchar(20);not null" json:"ownership_type" default:"full" comment:"所有权类型(full,partial)"`
	OwnershipPercentage *string    `gorm:"column:ownership_percentage;type:decimal(5,2)" json:"ownership_percentage" default:"100.00" comment:"所有权比例"`
	OwnershipStartDate  *time.Time `gorm:"column:ownership_start_date;type:date" json:"ownership_start_date" default:"null" comment:"所有权开始日期"`
	OwnershipEndDate    *time.Time `gorm:"column:ownership_end_date;type:date" json:"ownership_end_date" default:"null" comment:"所有权结束日期"`
	ContractNumber      *string    `gorm:"column:contract_number;type:varchar(100)" json:"contract_number" default:"null" comment:"合同编号"`
	Notes               *string    `gorm:"column:notes;type:text" json:"notes" default:"null" comment:"备注"`
	CreatedAt           time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Unit Unit `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"unit,omitempty"`
	// 业主拥有的停车位通过 LandlordParkingSlot 表查询（使用相同的 user_id）
}

func (Landlord) TableName() string {
	return "landlords"
}

// Ownership type constants
const (
	OwnershipTypeFull    = "full"
	OwnershipTypePartial = "partial"
)

// IsCurrentLandlord 检查是否是当前有效的业主
func (l *Landlord) IsCurrentLandlord() bool {
	now := time.Now()
	if l.OwnershipStartDate != nil && now.Before(*l.OwnershipStartDate) {
		return false
	}
	if l.OwnershipEndDate != nil && now.After(*l.OwnershipEndDate) {
		return false
	}
	return true
}
