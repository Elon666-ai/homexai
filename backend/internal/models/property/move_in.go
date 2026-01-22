package property

import (
	"time"
)

// MoveIn 入住申请表
type MoveIn struct {
	ID                          uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" comment:"主键ID"`
	RequestID                   uint       `gorm:"column:request_id;type:int unsigned;not null;uniqueIndex:uk_request_id" json:"request_id" comment:"关联的请求ID"`
	TenantID                    *uint      `gorm:"column:tenant_id;type:int unsigned;index:idx_tenant_id" json:"tenant_id" comment:"租户ID"`
	UnitID                      uint       `gorm:"column:unit_id;type:int unsigned;not null;index:idx_unit_id" json:"unit_id" comment:"单位ID"`
	ResidentName                string     `gorm:"column:resident_name;type:varchar(100);not null" json:"resident_name" comment:"住户姓名"`
	BuildingTower               *string    `gorm:"column:building_tower;type:varchar(50)" json:"building_tower,omitempty" comment:"楼栋"`
	MobileNumber                string     `gorm:"column:mobile_number;type:varchar(20);not null" json:"mobile_number" comment:"联系电话"`
	EmailAddress                *string    `gorm:"column:email_address;type:varchar(100)" json:"email_address,omitempty" comment:"邮箱地址"`
	MoveInDate                  time.Time  `gorm:"column:move_in_date;type:date;not null;index:idx_move_in_date" json:"move_in_date" comment:"入住日期"`
	TypeOfOccupancy             string     `gorm:"column:type_of_occupancy;type:varchar(20);not null;index:idx_type_of_occupancy" json:"type_of_occupancy" comment:"入住类型(owner,tenant)"`
	NumberOfOccupants           int        `gorm:"column:number_of_occupants;type:int unsigned;not null" json:"number_of_occupants" comment:"入住人数"`
	OccupantNames               *string    `gorm:"column:occupant_names;type:text" json:"occupant_names,omitempty" comment:"入住人姓名列表（逗号分隔）"`
	WillBringFurniture          bool       `gorm:"column:will_bring_furniture;type:tinyint(1);not null;default:0" json:"will_bring_furniture" comment:"是否搬运家具/家电"`
	EstimatedMoveInTime         *string    `gorm:"column:estimated_move_in_time;type:varchar(50)" json:"estimated_move_in_time,omitempty" comment:"预计时间"`
	MovingCompanyName           *string    `gorm:"column:moving_company_name;type:varchar(100)" json:"moving_company_name,omitempty" comment:"搬家公司"`
	VehiclePlateNo              *string    `gorm:"column:vehicle_plate_no;type:varchar(20)" json:"vehicle_plate_no,omitempty" comment:"车牌号"`
	AgreeToHouseRules           bool       `gorm:"column:agree_to_house_rules;type:tinyint(1);not null;default:0" json:"agree_to_house_rules" comment:"同意遵守公寓规则"`
	ResponsibleForDamage        bool       `gorm:"column:responsible_for_damage;type:tinyint(1);not null;default:0" json:"responsible_for_damage" comment:"对入住期间的损坏负责"`
	UnderstandSubjectToApproval bool       `gorm:"column:understand_subject_to_approval;type:tinyint(1);not null;default:0" json:"understand_subject_to_approval" comment:"理解入住需管理员批准"`
	Status                      string     `gorm:"column:status;type:varchar(20);not null;default:'draft';index:idx_status" json:"status" comment:"状态(draft,pending,approved,rejected)"`
	IsDraft                     bool       `gorm:"column:is_draft;type:tinyint(1);not null;default:1;index:idx_is_draft" json:"is_draft" comment:"是否为草稿"`
	ApprovedAt                  *time.Time `gorm:"column:approved_at;type:datetime" json:"approved_at" comment:"批准时间"`
	ApprovedBy                  *uint      `gorm:"column:approved_by;type:int unsigned" json:"approved_by" comment:"批准者用户ID"`
	CreatedAt                   time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at"`
	UpdatedAt                   time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at"`

	// Relationships
	Request Request `gorm:"foreignKey:RequestID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"request,omitempty"`
	Unit    Unit    `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"unit,omitempty"`
}

func (MoveIn) TableName() string {
	return "move_ins"
}

// Type of occupancy constants
const (
	OccupancyTypeOwner  = "owner"
	OccupancyTypeTenant = "tenant"
)

// Move-in status constants
const (
	MoveInStatusDraft     = "draft"
	MoveInStatusPending   = "pending"
	MoveInStatusApproved  = "approved"
	MoveInStatusRejected  = "rejected"
	MoveInStatusCompleted = "completed"
)
