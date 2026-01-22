package property

import (
	"time"
)

// WorkPermit 工作许可证表
type WorkPermit struct {
	ID                  uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	RequestID           uint       `gorm:"column:request_id;type:int unsigned;not null;uniqueIndex:uk_request_id" json:"request_id" default:"" comment:"关联的请求ID"`
	TenantID            *uint      `gorm:"column:tenant_id;type:int unsigned;index:idx_tenant_id" json:"tenant_id" default:"null" comment:"租户ID"`
	UnitID              uint       `gorm:"column:unit_id;type:int unsigned;not null;index:idx_unit_id" json:"unit_id" default:"" comment:"单位ID"`
	DateOfApplication   time.Time  `gorm:"column:date_of_application;type:date;not null" json:"date_of_application" default:"" comment:"申请日期"`
	WorkDescriptions    string     `gorm:"column:work_descriptions;type:varchar(200)" json:"work_descriptions" default:"" comment:"工作描述(多个用逗号分隔:noisy_work,dusty_work,hot_work)"`
	WorkType            string     `gorm:"column:work_type;type:varchar(50);not null" json:"work_type" default:"" comment:"工作类型(auxiliary,carpentry,electrical,fdas,fire_protection,housekeeping,inspection,masonry,mechanical,painting,plumbing,others)"`
	FromTime            time.Time  `gorm:"column:from_time;type:datetime;not null" json:"from_time" default:"" comment:"开始时间"`
	EndTime             time.Time  `gorm:"column:end_time;type:datetime;not null" json:"end_time" default:"" comment:"结束时间"`
	DescriptionOfWork   string     `gorm:"column:description_of_work;type:text;not null" json:"description_of_work" default:"" comment:"工作描述详情"`
	Personnel           JSONB      `gorm:"column:personnel;type:text" json:"personnel" default:"null" comment:"人员列表(JSON数组)"`
	PowerToolsMaterials JSONB      `gorm:"column:power_tools_materials;type:text" json:"power_tools_materials" default:"null" comment:"工具/材料列表(JSON数组)"`
	Status              string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"draft" comment:"状态(draft,pending,approved,rejected,expired)"`
	IsDraft             bool       `gorm:"column:is_draft;type:tinyint(1);not null;index:idx_is_draft" json:"is_draft" default:"1" comment:"是否为草稿"`
	ApprovedAt          *time.Time `gorm:"column:approved_at;type:datetime" json:"approved_at" default:"null" comment:"批准时间"`
	ApprovedBy          *uint      `gorm:"column:approved_by;type:int unsigned" json:"approved_by" default:"null" comment:"批准者用户ID"`
	ExpiresAt           *time.Time `gorm:"column:expires_at;type:datetime" json:"expires_at" default:"null" comment:"过期时间"`
	CreatedAt           time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt           time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Request Request `gorm:"foreignKey:RequestID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"request,omitempty"`
	Unit    Unit    `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"unit,omitempty"`
}

func (WorkPermit) TableName() string {
	return "work_permits"
}

// Work description constants
const (
	WorkDescNoisy = "noisy_work"
	WorkDescDusty = "dusty_work"
	WorkDescHot   = "hot_work"
)

// Work type constants
const (
	WorkTypeAuxiliary      = "auxiliary"
	WorkTypeCarpentry      = "carpentry"
	WorkTypeElectrical     = "electrical"
	WorkTypeFDAS           = "fdas"
	WorkTypeFireProtection = "fire_protection"
	WorkTypeHousekeeping   = "housekeeping"
	WorkTypeInspection     = "inspection"
	WorkTypeMasonry        = "masonry"
	WorkTypeMechanical     = "mechanical"
	WorkTypePainting       = "painting"
	WorkTypePlumbing       = "plumbing"
	WorkTypeOthers         = "others"
)

// Work permit status constants
const (
	WorkPermitStatusDraft    = "draft"
	WorkPermitStatusPending  = "pending"
	WorkPermitStatusApproved = "approved"
	WorkPermitStatusRejected = "rejected"
	WorkPermitStatusExpired  = "expired"
)

// Personnel 人员信息
type Personnel struct {
	Name        string `json:"name"`         // 姓名
	CompanyName string `json:"company_name"` // 公司名称
}

// PowerToolMaterial 工具/材料信息
type PowerToolMaterial struct {
	Description string `json:"description"` // 描述
	Quantity    string `json:"quantity"`    // 数量
}

