package property

import (
	"time"
)

// VehicleSticker 车辆贴纸申请表
type VehicleSticker struct {
	ID              uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" comment:"主键ID"`
	RequestID       uint       `gorm:"column:request_id;type:int unsigned;not null;uniqueIndex:uk_request_id" json:"request_id" comment:"关联的请求ID"`
	TenantID        *uint      `gorm:"column:tenant_id;type:int unsigned;index:idx_tenant_id" json:"tenant_id" comment:"租户ID"`
	UnitID          uint       `gorm:"column:unit_id;type:int unsigned;not null;index:idx_unit_id" json:"unit_id" comment:"单位ID"`
	ParkingSlotID   *uint      `gorm:"column:parking_slot_id;type:int unsigned;index:idx_parking_slot_id" json:"parking_slot_id" comment:"停车位ID"`
	VehicleBrand    string     `gorm:"column:vehicle_brand;type:varchar(100);not null" json:"vehicle_brand" comment:"车辆品牌"`
	VehicleModel    string     `gorm:"column:vehicle_model;type:varchar(100);not null" json:"vehicle_model" comment:"车辆型号"`
	VehicleYear     string     `gorm:"column:vehicle_year;type:varchar(10);not null" json:"vehicle_year" comment:"车辆年份"`
	VehicleColor    string     `gorm:"column:vehicle_color;type:varchar(50);not null" json:"vehicle_color" comment:"车辆颜色"`
	PlateNumber     string     `gorm:"column:plate_number;type:varchar(50);not null;index:idx_plate_number" json:"plate_number" comment:"车牌号"`
	Status          string     `gorm:"column:status;type:varchar(20);not null;default:'draft';index:idx_status" json:"status" comment:"状态(draft,pending,approved,rejected)"`
	IsDraft         bool       `gorm:"column:is_draft;type:tinyint(1);not null;default:1;index:idx_is_draft" json:"is_draft" comment:"是否为草稿"`
	ApprovedAt      *time.Time `gorm:"column:approved_at;type:datetime" json:"approved_at" comment:"批准时间"`
	ApprovedBy      *uint      `gorm:"column:approved_by;type:int unsigned" json:"approved_by" comment:"批准者用户ID"`
	CreatedAt       time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at"`

	// Relationships
	Request     Request     `gorm:"foreignKey:RequestID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"request,omitempty"`
	Unit        Unit        `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"unit,omitempty"`
	ParkingSlot *ParkingSlot `gorm:"foreignKey:ParkingSlotID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"parking_slot,omitempty"`
}

func (VehicleSticker) TableName() string {
	return "vehicle_stickers"
}

// Vehicle sticker status constants
const (
	VehicleStickerStatusDraft    = "draft"
	VehicleStickerStatusPending  = "pending"
	VehicleStickerStatusApproved = "approved"
	VehicleStickerStatusRejected = "rejected"
)

