package property

import (
	"time"
)

// SPAParkingSlot SPA-停车位关联表（多对多）
// 一个SPA可以代理多个停车位
type SPAParkingSlot struct {
	ID                     uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	SpaUserID              uint       `gorm:"column:spa_user_id;type:int unsigned;not null;uniqueIndex:uk_spa_parking,priority:1;index:idx_spa_user_id" json:"spa_user_id" default:"" comment:"SPA userID"`
	ParkingSlotID          uint       `gorm:"column:parking_slot_id;type:int unsigned;not null;uniqueIndex:uk_spa_parking,priority:2;index:idx_parking_slot_id" json:"parking_slot_id" default:"" comment:"停车位ID"`
	AuthorizationStartDate *time.Time `gorm:"column:authorization_start_date;type:date" json:"authorization_start_date" default:"null" comment:"授权开始日期"`
	AuthorizationEndDate   *time.Time `gorm:"column:authorization_end_date;type:date" json:"authorization_end_date" default:"null" comment:"授权结束日期"`
	AuthorizationDocument  *string    `gorm:"column:authorization_document;type:varchar(255)" json:"authorization_document" default:"null" comment:"授权文件路径"`
	Scope                  string     `gorm:"column:scope;type:varchar(20);not null" json:"scope" default:"full" comment:"授权范围(full,limited)"`
	Notes                  *string    `gorm:"column:notes;type:text" json:"notes" default:"null" comment:"备注"`
	IsActive               bool       `gorm:"column:is_active;type:tinyint(1);not null;index:idx_is_active" json:"is_active" default:"1" comment:"是否激活"`

	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`

	// Relationships
	ParkingSlot ParkingSlot `gorm:"foreignKey:ParkingSlotID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"parking_slot,omitempty"`
}

func (SPAParkingSlot) TableName() string {
	return "spa_parking_slots"
}
