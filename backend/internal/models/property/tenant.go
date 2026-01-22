package property

import (
	"time"
)

// Tenant 租客关联表
type Tenant struct {
	ID             uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	UserID         uint      `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" default:"" comment:"用户ID(主库users表)"`
	UnitID         uint      `gorm:"column:unit_id;type:int unsigned;not null;index:idx_unit_id" json:"unit_id" default:"" comment:"房源ID"`
	LeaseStartDate time.Time `gorm:"column:lease_start_date;type:date;not null;index:idx_lease_dates,priority:1" json:"lease_start_date" default:"" comment:"租约开始日期"`
	LeaseEndDate   time.Time `gorm:"column:lease_end_date;type:date;not null;index:idx_lease_dates,priority:2" json:"lease_end_date" default:"" comment:"租约结束日期"`
	MonthlyRent    string    `gorm:"column:monthly_rent;type:decimal(10,2);not null" json:"monthly_rent" default:"0.00" comment:"月租金"`
	DepositAmount  *string   `gorm:"column:deposit_amount;type:decimal(10,2)" json:"deposit_amount" default:"null" comment:"押金金额"`
	ContractNumber *string   `gorm:"column:contract_number;type:varchar(100)" json:"contract_number" default:"null" comment:"合同编号"`
	Status         string    `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"pending" comment:"租约状态(active,expired,terminated,pending)"`
	Notes          *string   `gorm:"column:notes;type:text" json:"notes" default:"null" comment:"备注"`
	CreatedAt      time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt      time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Unit Unit `gorm:"foreignKey:UnitID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"unit,omitempty"`
}

func (Tenant) TableName() string {
	return "tenants"
}

// Tenant status constants
const (
	TenantStatusActive     = "active"
	TenantStatusExpired    = "expired"
	TenantStatusTerminated = "terminated"
	TenantStatusPending    = "pending"
)

// IsActive 检查租约是否有效
func (t *Tenant) IsActive() bool {
	return t.Status == TenantStatusActive && time.Now().Before(t.LeaseEndDate)
}

// IsExpired 检查租约是否过期
func (t *Tenant) IsExpired() bool {
	return time.Now().After(t.LeaseEndDate)
}

// DaysUntilExpiry 返回距离租约到期的天数
func (t *Tenant) DaysUntilExpiry() int {
	if t.IsExpired() {
		return 0
	}
	duration := time.Until(t.LeaseEndDate)
	return int(duration.Hours() / 24)
}
