package property

import (
	"time"
)

// Bill 账单表
type Bill struct {
	ID          uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	BillNumber  string     `gorm:"column:bill_number;type:varchar(50);not null;uniqueIndex:uk_bill_number" json:"bill_number" default:"" comment:"账单编号"`
	UnitID        *uint      `gorm:"column:unit_id;type:int unsigned;index:idx_unit_id" json:"unit_id" default:"null" comment:"房源ID(unit账单必填)"`
	ParkingSlotID *uint      `gorm:"column:parking_slot_id;type:int unsigned;index:idx_parking_slot_id" json:"parking_slot_id" default:"null" comment:"停车位ID(parking账单必填)"`
	UserID        uint       `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" default:"" comment:"付款人用户ID(主库users表)"`
	TenantID      *uint      `gorm:"column:tenant_id;type:int unsigned;index:idx_tenant_id" json:"tenant_id" default:"null" comment:"租客ID(可选)"`
	LandlordID    *uint      `gorm:"column:landlord_id;type:int unsigned;index:idx_landlord_id" json:"landlord_id" default:"null" comment:"业主ID(可选)"`
	SpaID         *uint      `gorm:"column:spa_id;type:int unsigned;index:idx_spa_id" json:"spa_id" default:"null" comment:"SPA用户ID(可选)"`
	BillType      string     `gorm:"column:bill_type;type:varchar(50);not null" json:"bill_type" default:"" comment:"账单类型(unit,parking)"`
	BillingMonth  string     `gorm:"column:billing_month;type:varchar(7);not null;index:idx_billing_month" json:"billing_month" default:"" comment:"账单月份(YYYY-MM)"`
	Amount      string     `gorm:"column:amount;type:decimal(10,2);not null" json:"amount" default:"0.00" comment:"金额"`
	Currency    string     `gorm:"column:currency;type:varchar(10);not null" json:"currency" default:"PHP" comment:"货币"`
	DueDate     time.Time  `gorm:"column:due_date;type:datetime;not null;index:idx_due_date" json:"due_date" default:"" comment:"到期日期"`
	IssueDate   time.Time  `gorm:"column:issue_date;type:datetime;not null" json:"issue_date" default:"" comment:"开具日期"`
	Status      string     `gorm:"column:status;type:varchar(30);not null;index:idx_status" json:"status" default:"pending" comment:"状态(pending,paid,overdue,cancelled,partial,confirming)"`
	Description *string    `gorm:"column:description;type:text" json:"description" default:"null" comment:"描述"`
	PaidAmount  string     `gorm:"column:paid_amount;type:decimal(10,2);not null" json:"paid_amount" default:"0.00" comment:"已支付金额"`
	PaidAt      *time.Time `gorm:"column:paid_at;type:datetime" json:"paid_at" default:"null" comment:"支付时间"`
	CreatedBy   *uint      `gorm:"column:created_by;type:int unsigned;index:idx_created_by" json:"created_by" default:"null" comment:"创建者用户ID"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Unit        Unit       `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"unit,omitempty"`
	ParkingSlot *ParkingSlot `gorm:"foreignKey:ParkingSlotID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"parking_slot,omitempty"`
	Items       []BillItem `gorm:"foreignKey:BillID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"items,omitempty"`
	Payments    []Payment  `gorm:"foreignKey:BillID" json:"payments,omitempty"`
}

func (Bill) TableName() string {
	return "bills"
}

// Bill status constants
const (
	BillStatusPending            = "pending"
	BillStatusPaid               = "paid"
	BillStatusOverdue            = "overdue"
	BillStatusCancelled          = "cancelled"
	BillStatusPartial            = "partial"
	BillStatusConfirming = "confirming" // 等待物业办公室确认入账
)

// Bill type constants
const (
	BillTypeUnit    = "unit"    // Unit账单
	BillTypeParking = "parking" // Parking账单
)

// IsPaid 检查账单是否已支付
func (b *Bill) IsPaid() bool {
	return b.Status == BillStatusPaid
}

// IsOverdue 检查账单是否逾期
func (b *Bill) IsOverdue() bool {
	return b.Status == BillStatusOverdue || (b.Status == BillStatusPending && time.Now().After(b.DueDate))
}

// IsPartiallyPaid 检查账单是否部分支付
func (b *Bill) IsPartiallyPaid() bool {
	return b.Status == BillStatusPartial
}

// DaysOverdue 返回逾期天数
func (b *Bill) DaysOverdue() int {
	if !b.IsOverdue() {
		return 0
	}
	duration := time.Since(b.DueDate)
	return int(duration.Hours() / 24)
}
