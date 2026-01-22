package property

import (
	"time"
)

// VisitorRegistration 访客登记表
type VisitorRegistration struct {
	ID                uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" comment:"主键ID"`
	VisitorName       string     `gorm:"column:visitor_name;type:varchar(200);not null" json:"visitor_name" comment:"访客姓名"`
	ContactNumber     string     `gorm:"column:contact_number;type:varchar(20);not null" json:"contact_number" comment:"联系电话"`
	IDPresented       *string   `gorm:"column:id_presented;type:varchar(50)" json:"id_presented,omitempty" comment:"出示证件类型(passport,driver_license,company_id)"`
	ResidentName      string     `gorm:"column:resident_name;type:varchar(200);not null" json:"resident_name" comment:"被访住户姓名"`
	UnitID            uint       `gorm:"column:unit_id;type:int unsigned;not null;index:idx_unit_id" json:"unit_id" comment:"房号ID"`
	UnitNumber        string     `gorm:"column:unit_number;type:varchar(50);not null" json:"unit_number" comment:"房号"`
	PurposeOfVisit    string     `gorm:"column:purpose_of_visit;type:varchar(50);not null;index:idx_purpose" json:"purpose_of_visit" comment:"来访目的(personal,delivery,repair_maintenance,other)"`
	PurposeOther      *string   `gorm:"column:purpose_other;type:varchar(200)" json:"purpose_other,omitempty" comment:"其他目的说明"`
	VisitDate         time.Time `gorm:"column:visit_date;type:date;not null;index:idx_visit_date" json:"visit_date" comment:"日期"`
	TimeIn            time.Time `gorm:"column:time_in;type:datetime;not null;index:idx_time_in" json:"time_in" comment:"进入时间"`
	TimeOut           *time.Time `gorm:"column:time_out;type:datetime" json:"time_out,omitempty" comment:"离开时间"`
	VisitorPassNo     *string   `gorm:"column:visitor_pass_no;type:varchar(50);index:idx_visitor_pass_no" json:"visitor_pass_no,omitempty" comment:"访客证编号"`
	RegisteredBy      uint       `gorm:"column:registered_by;type:int unsigned;not null;index:idx_registered_by" json:"registered_by" comment:"登记人用户ID(主库users表)"`
	Status            string     `gorm:"column:status;type:varchar(20);not null;default:'registered';index:idx_status" json:"status" comment:"状态(registered,checked_in,checked_out)"`
	CreatedAt         time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at"`

	// Relationships
	Unit Unit `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"unit,omitempty"`
}

func (VisitorRegistration) TableName() string {
	return "visitor_registrations"
}

// ID presented constants
const (
	IDPresentedPassport      = "passport"
	IDPresentedDriverLicense = "driver_license"
	IDPresentedCompanyID     = "company_id"
)

// Purpose of visit constants
const (
	PurposePersonal         = "personal"
	PurposeDelivery         = "delivery"
	PurposeRepairMaintenance = "repair_maintenance"
	PurposeOther            = "other"
)

// Visitor registration status constants
const (
	VisitorRegistrationStatusRegistered = "registered"
	VisitorRegistrationStatusCheckedIn  = "checked_in"
	VisitorRegistrationStatusCheckedOut = "checked_out"
)

