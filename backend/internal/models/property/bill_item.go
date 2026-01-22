package property

import (
	"time"
)

// BillItem 账单项表
type BillItem struct {
	ID          uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	BillID      uint      `gorm:"column:bill_id;type:int unsigned;not null;index:idx_bill_id" json:"bill_id" default:"" comment:"账单ID"`
	ItemType    string    `gorm:"column:item_type;type:varchar(50);not null;index:idx_item_type" json:"item_type" default:"" comment:"项目类型(association_dues,insurance_common_area,rpt_common_area,water_charge,electricity_charge,arrears,penalty,service_fee,parking_management_fee)"`
	Description string    `gorm:"column:description;type:varchar(255)" json:"description" default:"" comment:"项目描述"`
	Amount      string    `gorm:"column:amount;type:decimal(10,2);not null" json:"amount" default:"0.00" comment:"金额"`
	Currency    string    `gorm:"column:currency;type:varchar(10);not null" json:"currency" default:"PHP" comment:"货币"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Bill Bill `gorm:"foreignKey:BillID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"bill,omitempty"`
}

func (BillItem) TableName() string {
	return "bill_items"
}

// Bill item type constants for unit bills
const (
	BillItemTypeAssociationDues    = "association_dues"     // Association Dues
	BillItemTypeInsuranceCommonArea = "insurance_common_area" // Insurance - Common Area
	BillItemTypeRPTCommonArea       = "rpt_common_area"     // RPT - Common Area
	BillItemTypeWaterCharge         = "water_charge"        // Water Charge
	BillItemTypeElectricityCharge   = "electricity_charge"  // Electricity Charge
	BillItemTypeArrears             = "arrears"            // 欠费
	BillItemTypePenalty             = "penalty"            // 罚金
	BillItemTypeServiceFee          = "service_fee"        // 服务市场订单费
	BillItemTypeParkingMgmtFee      = "parking_management_fee" // 停车位管理费
)

