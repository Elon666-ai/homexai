package property

import (
	"time"
)

// HouseholdStaffRegistration 家政人员登记表
type HouseholdStaffRegistration struct {
	ID                    uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" comment:"主键ID"`
	RequestID             uint       `gorm:"column:request_id;type:int unsigned;not null;uniqueIndex:uk_request_id" json:"request_id" comment:"关联的请求ID"`
	TenantID              *uint      `gorm:"column:tenant_id;type:int unsigned;index:idx_tenant_id" json:"tenant_id" comment:"租户ID"`
	UnitID                uint       `gorm:"column:unit_id;type:int unsigned;not null;index:idx_unit_id" json:"unit_id" comment:"单位ID"`
	ResidentName          string     `gorm:"column:resident_name;type:varchar(100);not null" json:"resident_name" comment:"住户姓名"`
	LastName              string     `gorm:"column:last_name;type:varchar(100);not null" json:"last_name" comment:"姓氏"`
	FirstName             string     `gorm:"column:first_name;type:varchar(100);not null" json:"first_name" comment:"名字"`
	MiddleName            *string    `gorm:"column:middle_name;type:varchar(100)" json:"middle_name,omitempty" comment:"中间名"`
	Gender                string     `gorm:"column:gender;type:varchar(20);not null;index:idx_gender" json:"gender" comment:"性别(male,female,other)"`
	Designation           string     `gorm:"column:designation;type:varchar(50);not null;index:idx_designation" json:"designation" comment:"职位(driver,housekeeper,other)"`
	StayInStayOut         string     `gorm:"column:stay_in_stay_out;type:varchar(20);not null" json:"stay_in_stay_out" comment:"住宿类型(stay_in,stay_out)"`
	EmployeeMobileNumber  string     `gorm:"column:employee_mobile_number;type:varchar(20);not null" json:"employee_mobile_number" comment:"员工手机号"`
	EmployeeAddress       string     `gorm:"column:employee_address;type:varchar(500);not null" json:"employee_address" comment:"员工地址"`
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

func (HouseholdStaffRegistration) TableName() string {
	return "household_staff_registrations"
}

// Gender constants
const (
	GenderMale   = "male"
	GenderFemale = "female"
	GenderOther  = "other"
)

// Designation constants
const (
	DesignationDriver    = "driver"
	DesignationHousekeeper = "housekeeper"
	DesignationOther     = "other"
)

// StayInStayOut constants
const (
	StayInStayOutIn  = "stay_in"
	StayInStayOutOut = "stay_out"
)

// HouseholdStaffRegistrationStatus constants
const (
	HouseholdStaffRegistrationStatusDraft    = "draft"
	HouseholdStaffRegistrationStatusPending  = "pending"
	HouseholdStaffRegistrationStatusApproved = "approved"
	HouseholdStaffRegistrationStatusRejected = "rejected"
)

