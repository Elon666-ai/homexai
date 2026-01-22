package property

import (
	"time"
)

// MoveOut 搬出申请表
type MoveOut struct {
	ID                          uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" comment:"主键ID"`
	RequestID                   uint       `gorm:"column:request_id;type:int unsigned;not null;uniqueIndex:uk_request_id" json:"request_id" comment:"关联的请求ID"`
	TenantID                    *uint      `gorm:"column:tenant_id;type:int unsigned;index:idx_tenant_id" json:"tenant_id" comment:"租户ID"`
	UnitID                      uint       `gorm:"column:unit_id;type:int unsigned;not null;index:idx_unit_id" json:"unit_id" comment:"单位ID"`
	ResidentName                string     `gorm:"column:resident_name;type:varchar(100);not null" json:"resident_name" comment:"住户姓名"`
	BuildingTower               *string   `gorm:"column:building_tower;type:varchar(50)" json:"building_tower,omitempty" comment:"楼栋"`
	MobileNumber                string     `gorm:"column:mobile_number;type:varchar(20);not null" json:"mobile_number" comment:"联系电话"`
	EmailAddress                *string   `gorm:"column:email_address;type:varchar(100)" json:"email_address,omitempty" comment:"邮箱地址"`
	MoveOutDate                 time.Time `gorm:"column:move_out_date;type:date;not null;index:idx_move_out_date" json:"move_out_date" comment:"搬出日期"`
	OccupancyType               string     `gorm:"column:occupancy_type;type:varchar(20);not null;index:idx_occupancy_type" json:"occupancy_type" comment:"入住类型(owner,tenant)"`
	PrimaryOccupant             *string    `gorm:"column:primary_occupant;type:varchar(100)" json:"primary_occupant,omitempty" comment:"主要入住人"`
	OccupantNames               *string    `gorm:"column:occupant_names;type:text" json:"occupant_names,omitempty" comment:"搬出入住人姓名列表（逗号分隔）"`
	ReasonForMoveOut            *string   `gorm:"column:reason_for_move_out;type:text" json:"reason_for_move_out,omitempty" comment:"搬出原因"`
	AllKeysReturned             string     `gorm:"column:all_keys_returned;type:varchar(10);not null" json:"all_keys_returned" comment:"钥匙是否已归还(yes,no)"`
	UnitConditionUponMoveOut    string     `gorm:"column:unit_condition_upon_move_out;type:varchar(50);not null" json:"unit_condition_upon_move_out" comment:"房屋状况(good,minor_damage,major_damage)"`
	AllUtilityBillsSettled      bool       `gorm:"column:all_utility_bills_settled;type:tinyint(1);not null;default:0" json:"all_utility_bills_settled" comment:"所有水电费已结清"`
	AuthorizeInspection         bool       `gorm:"column:authorize_inspection;type:tinyint(1);not null;default:0" json:"authorize_inspection" comment:"授权物业管理检查房屋"`
	UnderstandDepositDeduction  bool       `gorm:"column:understand_deposit_deduction;type:tinyint(1);not null;default:0" json:"understand_deposit_deduction" comment:"理解可能从押金中扣除费用"`
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

func (MoveOut) TableName() string {
	return "move_outs"
}

// Keys returned constants
const (
	KeysReturnedYes = "yes"
	KeysReturnedNo  = "no"
)

// Unit condition constants
const (
	UnitConditionGood        = "good"
	UnitConditionMinorDamage = "minor_damage"
	UnitConditionMajorDamage = "major_damage"
)

// Move-out status constants
const (
	MoveOutStatusDraft    = "draft"
	MoveOutStatusPending  = "pending"
	MoveOutStatusApproved = "approved"
	MoveOutStatusRejected = "rejected"
)

