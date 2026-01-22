package property

import (
	"time"
)

// GatePass 门禁通行证表
type GatePass struct {
	ID            uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	RequestID     uint      `gorm:"column:request_id;type:int unsigned;not null;uniqueIndex:uk_request_id" json:"request_id" default:"" comment:"关联的请求ID"`
	TenantID      *uint     `gorm:"column:tenant_id;type:int unsigned;index:idx_tenant_id" json:"tenant_id" default:"null" comment:"租户ID"`
	UnitID        uint      `gorm:"column:unit_id;type:int unsigned;not null;index:idx_unit_id" json:"unit_id" default:"" comment:"单位ID"`
	Type          string    `gorm:"column:type;type:varchar(20);not null;index:idx_type" json:"type" default:"" comment:"类型(pull_in,pull_out)"`
	ContactNo     string    `gorm:"column:contact_no;type:varchar(50);not null" json:"contact_no" default:"" comment:"联系电话"`
	DeliveryDate  time.Time `gorm:"column:delivery_date;type:date;not null" json:"delivery_date" default:"" comment:"配送日期"`
	ItemList      JSONB     `gorm:"column:item_list;type:text" json:"item_list" default:"null" comment:"物品列表(JSON数组)"`
	Remarks       string    `gorm:"column:remarks;type:text" json:"remarks" default:"" comment:"备注"`
	Status        string    `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"draft" comment:"状态(draft,pending,approved,rejected,expired)"`
	IsDraft       bool      `gorm:"column:is_draft;type:tinyint(1);not null;index:idx_is_draft" json:"is_draft" default:"1" comment:"是否为草稿"`
	ApprovedAt    *time.Time `gorm:"column:approved_at;type:datetime" json:"approved_at" default:"null" comment:"批准时间"`
	ApprovedBy    *uint      `gorm:"column:approved_by;type:int unsigned" json:"approved_by" default:"null" comment:"批准者用户ID"`
	ExpiresAt     *time.Time `gorm:"column:expires_at;type:datetime" json:"expires_at" default:"null" comment:"过期时间"`
	CreatedAt     time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt     time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Request Request `gorm:"foreignKey:RequestID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"request,omitempty"`
	Unit    Unit    `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"unit,omitempty"`
}

func (GatePass) TableName() string {
	return "gate_passes"
}

// Gate pass type constants
const (
	GatePassTypePullIn  = "pull_in"
	GatePassTypePullOut = "pull_out"
)

// Gate pass status constants
const (
	GatePassStatusDraft    = "draft"
	GatePassStatusPending  = "pending"
	GatePassStatusApproved = "approved"
	GatePassStatusRejected = "rejected"
	GatePassStatusExpired  = "expired"
)

// GatePassItem 物品信息
type GatePassItem struct {
	Quantity          string `json:"quantity"`           // 数量
	Description       string `json:"description"`        // 描述
	UnitOfMeasurement string `json:"unit_of_measurement"` // 计量单位
}

