package property

import (
	"time"
)

// Payment 支付记录表
type Payment struct {
	ID              uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	PaymentNumber   string    `gorm:"column:payment_number;type:varchar(50);not null;uniqueIndex:uk_payment_number" json:"payment_number" default:"" comment:"支付编号"`
	BillID          uint      `gorm:"column:bill_id;type:int unsigned;not null;index:idx_bill_id" json:"bill_id" default:"" comment:"账单ID"`
	UserID          uint      `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" default:"" comment:"支付用户ID(主库users表)"`
	Amount          string    `gorm:"column:amount;type:decimal(10,2);not null" json:"amount" default:"0.00" comment:"支付金额"`
	Currency        string    `gorm:"column:currency;type:varchar(10);not null" json:"currency" default:"PHP" comment:"货币"`
	PaymentMethod   string    `gorm:"column:payment_method;type:varchar(30);not null" json:"payment_method" default:"" comment:"支付方式(cash,bank_transfer,credit_card,debit_card,gcash,paymaya,check,other)"`
	PaymentDate     time.Time `gorm:"column:payment_date;type:datetime;not null;index:idx_payment_date" json:"payment_date" default:"" comment:"支付日期"`
	Status          string    `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"pending" comment:"状态(pending,completed,failed,refunded,cancelled)"`
	TransactionID   *string   `gorm:"column:transaction_id;type:varchar(255)" json:"transaction_id" default:"null" comment:"交易ID"`
	ReferenceNumber *string   `gorm:"column:reference_number;type:varchar(255)" json:"reference_number" default:"null" comment:"参考号"`
	Notes           *string   `gorm:"column:notes;type:text" json:"notes" default:"null" comment:"备注"`
	ReceiptFile     *string   `gorm:"column:receipt_file;type:varchar(500)" json:"receipt_file" default:"null" comment:"收据文件路径"`
	CreatedAt       time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt       time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Bill Bill `gorm:"foreignKey:BillID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"bill,omitempty"`
}

func (Payment) TableName() string {
	return "payments"
}

// Payment method constants
const (
	PaymentMethodCash         = "cash"
	PaymentMethodBankTransfer = "bank_transfer"
	PaymentMethodCreditCard   = "credit_card"
	PaymentMethodDebitCard    = "debit_card"
	PaymentMethodGCash        = "gcash"
	PaymentMethodPayMaya      = "paymaya"
	PaymentMethodCheck        = "check"
	PaymentMethodOther        = "other"
)

// Payment status constants
const (
	PaymentStatusPending   = "pending"
	PaymentStatusCompleted = "completed"
	PaymentStatusFailed    = "failed"
	PaymentStatusRefunded  = "refunded"
	PaymentStatusCancelled = "cancelled"
)

// IsCompleted 检查支付是否完成
func (p *Payment) IsCompleted() bool {
	return p.Status == PaymentStatusCompleted
}

// IsFailed 检查支付是否失败
func (p *Payment) IsFailed() bool {
	return p.Status == PaymentStatusFailed
}

// IsRefunded 检查支付是否已退款
func (p *Payment) IsRefunded() bool {
	return p.Status == PaymentStatusRefunded
}

// IsCancelled 检查支付是否已取消
func (p *Payment) IsCancelled() bool {
	return p.Status == PaymentStatusCancelled
}
