package property

import (
	"time"
)

// ServiceListing 服务列表表
type ServiceListing struct {
	ID          uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	ServiceType string     `gorm:"column:service_type;type:varchar(50);not null;index:idx_service_type" json:"service_type" default:"" comment:"服务类型(renovation,cleaning,repair,moving,other)"`
	Title       string     `gorm:"column:title;type:varchar(200);not null" json:"title" default:"" comment:"服务标题"`
	Description *string    `gorm:"column:description;type:text" json:"description" default:"null" comment:"服务描述"`
	Price       string     `gorm:"column:price;type:decimal(10,2);not null" json:"price" default:"0.00" comment:"收费标准"`
	Currency    string     `gorm:"column:currency;type:varchar(10);not null" json:"currency" default:"PHP" comment:"货币"`
	IsPropertyService bool `gorm:"column:is_property_service;type:tinyint(1);not null;default:1;index:idx_is_property_service" json:"is_property_service" default:"1" comment:"是否物业自身服务(1-是,0-否,第三方服务)"`
	ThirdPartyContact *string `gorm:"column:third_party_contact;type:varchar(255)" json:"third_party_contact" default:"null" comment:"第三方联系方式(仅第三方服务时使用)"`
	Status      string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"active" comment:"状态(active,inactive)"`
	CreatedBy   *uint      `gorm:"column:created_by;type:int unsigned;index:idx_created_by" json:"created_by" default:"null" comment:"创建者用户ID"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Orders []ServiceOrder `gorm:"foreignKey:ServiceListingID" json:"orders,omitempty"`
}

func (ServiceListing) TableName() string {
	return "service_listings"
}

// Service type constants
const (
	ServiceTypeRenovation = "renovation" // 装修
	ServiceTypeCleaning   = "cleaning"   // 保洁
	ServiceTypeRepair     = "repair"     // 维修
	ServiceTypeMoving     = "moving"     // 搬家
	ServiceTypeOther      = "other"      // 其他
)

// Service listing status constants
const (
	ServiceListingStatusActive   = "active"   // 上架
	ServiceListingStatusInactive = "inactive" // 下架
)

// IsActive 检查服务是否上架
func (s *ServiceListing) IsActive() bool {
	return s.Status == ServiceListingStatusActive
}

// IsPropertyService 检查是否是物业自身服务
func (s *ServiceListing) IsPropertyOwnedService() bool {
	return s.IsPropertyService
}

