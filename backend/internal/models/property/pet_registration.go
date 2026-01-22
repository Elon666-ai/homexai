package property

import (
	"time"
)

// PetRegistration 宠物登记表
type PetRegistration struct {
	ID                    uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" comment:"主键ID"`
	RequestID             uint       `gorm:"column:request_id;type:int unsigned;not null;uniqueIndex:uk_request_id" json:"request_id" comment:"关联的请求ID"`
	TenantID              *uint      `gorm:"column:tenant_id;type:int unsigned;index:idx_tenant_id" json:"tenant_id" comment:"租户ID"`
	UnitID                uint       `gorm:"column:unit_id;type:int unsigned;not null;index:idx_unit_id" json:"unit_id" comment:"单位ID"`
	ResidentName          string     `gorm:"column:resident_name;type:varchar(100);not null" json:"resident_name" comment:"住户姓名"`
	MobileNumber          string     `gorm:"column:mobile_number;type:varchar(20);not null" json:"mobile_number" comment:"联系电话"`
	EmailAddress          string     `gorm:"column:email_address;type:varchar(100);not null" json:"email_address" comment:"邮箱地址"`
	PetName               string     `gorm:"column:pet_name;type:varchar(100);not null" json:"pet_name" comment:"宠物名"`
	PetType               string     `gorm:"column:pet_type;type:varchar(20);not null;index:idx_pet_type" json:"pet_type" comment:"宠物类型(dog,cat)"`
	Breed                 *string    `gorm:"column:breed;type:varchar(100)" json:"breed,omitempty" comment:"品种"`
	Weight                string     `gorm:"column:weight;type:varchar(20);not null" json:"weight" comment:"体重(kg)"`
	ColorMarkings         *string    `gorm:"column:color_markings;type:varchar(200)" json:"color_markings,omitempty" comment:"颜色特征"`
	RabiesVaccinated      string     `gorm:"column:rabies_vaccinated;type:varchar(10);not null" json:"rabies_vaccinated" comment:"是否已接种狂犬疫苗(yes,no)"`
	IsNonAggressive       bool       `gorm:"column:is_non_aggressive;type:tinyint(1);not null;default:0" json:"is_non_aggressive" comment:"宠物是否不具攻击性"`
	WillKeepLeashed       bool       `gorm:"column:will_keep_leashed;type:tinyint(1);not null;default:0" json:"will_keep_leashed" comment:"是否会在公共区域使用牵引绳或携带"`
	AgreeToRules          bool       `gorm:"column:agree_to_rules;type:tinyint(1);not null;default:0" json:"agree_to_rules" comment:"是否同意遵守公寓宠物规则"`
	Status                string     `gorm:"column:status;type:varchar(20);not null;default:'draft';index:idx_status" json:"status" comment:"状态(draft,pending,approved,rejected)"`
	IsDraft               bool       `gorm:"column:is_draft;type:tinyint(1);not null;default:1;index:idx_is_draft" json:"is_draft" comment:"是否为草稿"`
	ApprovedAt            *time.Time `gorm:"column:approved_at;type:datetime" json:"approved_at" comment:"批准时间"`
	ApprovedBy            *uint      `gorm:"column:approved_by;type:int unsigned" json:"approved_by" comment:"批准者用户ID"`
	CreatedAt             time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at"`

	// Relationships
	Request Request `gorm:"foreignKey:RequestID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"request,omitempty"`
	Unit    Unit    `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"unit,omitempty"`
}

func (PetRegistration) TableName() string {
	return "pet_registrations"
}

// Pet type constants
const (
	PetTypeDog = "dog"
	PetTypeCat = "cat"
)

// Rabies vaccinated constants
const (
	RabiesVaccinatedYes = "yes"
	RabiesVaccinatedNo  = "no"
)

// Pet registration status constants
const (
	PetRegistrationStatusDraft    = "draft"
	PetRegistrationStatusPending  = "pending"
	PetRegistrationStatusApproved = "approved"
	PetRegistrationStatusRejected = "rejected"
)

