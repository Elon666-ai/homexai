package master

import (
	"time"
)

// Property 物业信息表
type Property struct {
	ID              uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	Name            string    `gorm:"column:name;type:varchar(200);not null" json:"name" default:"" comment:"物业名称"`
	Subdomain       string    `gorm:"column:subdomain;type:varchar(50);not null;uniqueIndex:uk_subdomain" json:"subdomain" default:"" comment:"子域名"`
	DBName          string    `gorm:"column:db_name;type:varchar(100);not null" json:"db_name" default:"" comment:"数据库名称"`
	Status          string    `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"active" comment:"状态(active,inactive,suspended)"`
	Category        string    `gorm:"column:category;type:varchar(20);not null" json:"category" default:"residential" comment:"物业类别(commercial_office,warehouse,residential)"`
	Province        string    `gorm:"column:province;type:varchar(100);not null" json:"province" default:"" comment:"省份"`
	City            *string   `gorm:"column:city;type:varchar(100)" json:"city" default:"null" comment:"城市"`
	District        *string   `gorm:"column:district;type:varchar(100)" json:"district" default:"null" comment:"区"`
	AreaName        *string   `gorm:"column:area_name;type:varchar(100)" json:"area_name" default:"null" comment:"片区名称"`
	Address         *string   `gorm:"column:address;type:text;not null" json:"address" default:"" comment:"物业地址"`
	Developer       string    `gorm:"column:developer;type:varchar(200);not null" json:"developer" default:"" comment:"开发商"`
	HandoverDate    time.Time `gorm:"column:handover_date;type:date;not null" json:"handover_date" comment:"交房时间"`
	PropertyManagementCompany *string `gorm:"column:property_management_company;type:varchar(200)" json:"property_management_company" default:"null" comment:"物业管理公司"`
	Country         string    `gorm:"column:country;type:varchar(100);not null" json:"country" default:"Philippines" comment:"国家"`
	PostalCode      *string   `gorm:"column:postal_code;type:varchar(20)" json:"postal_code" default:"null" comment:"邮政编码"`
	ContactEmail    *string   `gorm:"column:contact_email;type:varchar(255)" json:"contact_email" default:"null" comment:"联系邮箱"`
	ContactPhone    *string   `gorm:"column:contact_phone;type:varchar(20)" json:"contact_phone" default:"null" comment:"联系电话"`
	Website         *string   `gorm:"column:website;type:varchar(255)" json:"website" default:"null" comment:"网站"`
	Description     *string   `gorm:"column:description;type:text" json:"description" default:"null" comment:"描述"`
	LogoURL         *string   `gorm:"column:logo_url;type:varchar(500)" json:"logo_url" default:"null" comment:"Logo图片URL"`
	Timezone        string    `gorm:"column:timezone;type:varchar(50);not null" json:"timezone" default:"Asia/Manila" comment:"时区"`
	DefaultLanguage string    `gorm:"column:default_language;type:varchar(10);not null" json:"default_language" default:"en" comment:"默认语言(en,zh-CN,zh-TW,tl)"`
	CreatedAt       time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt       time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	DBMapping *PropertyDBMapping `gorm:"foreignKey:PropertyID" json:"db_mapping,omitempty"`
	UserRoles []UserRole         `gorm:"foreignKey:PropertyID" json:"user_roles,omitempty"`
}

func (Property) TableName() string {
	return "properties"
}

// IsActive 检查物业是否激活
func (p *Property) IsActive() bool {
	return p.Status == "active"
}

// GetFullAddress 获取完整地址
func (p *Property) GetFullAddress() string {
	address := ""
	if p.Address != nil {
		address = *p.Address
	}
	if p.City != nil && *p.City != "" {
		if address != "" {
			address += ", "
		}
		address += *p.City
	}
	if p.Country != "" {
		if address != "" {
			address += ", "
		}
		address += p.Country
	}
	if p.PostalCode != nil && *p.PostalCode != "" {
		if address != "" {
			address += " "
		}
		address += *p.PostalCode
	}
	return address
}

// Property category constants
const (
	PropertyCategoryCommercialOffice = "commercial_office"
	PropertyCategoryWarehouse        = "warehouse"
	PropertyCategoryResidential      = "residential"
)

// Philippine provinces and cities data
var PhilippineProvinces = []string{
	"Abra", "Agusan del Norte", "Agusan del Sur", "Aklan", "Albay", "Antique", "Apayao", "Aurora",
	"Basilan", "Bataan", "Batanes", "Batangas", "Benguet", "Biliran", "Bohol", "Bukidnon",
	"Bulacan", "Cagayan", "Camarines Norte", "Camarines Sur", "Camiguin", "Capiz", "Catanduanes",
	"Cavite", "Cebu", "Compostela Valley", "Cotabato", "Davao del Norte", "Davao del Sur",
	"Davao Occidental", "Davao Oriental", "Dinagat Islands", "Eastern Samar", "Guimaras",
	"Ifugao", "Ilocos Norte", "Ilocos Sur", "Iloilo", "Isabela", "Kalinga", "La Union",
	"Laguna", "Lanao del Norte", "Lanao del Sur", "Leyte", "Maguindanao", "Marinduque",
	"Masbate", "Metro Manila", "Misamis Occidental", "Misamis Oriental", "Mountain Province",
	"Negros Occidental", "Negros Oriental", "Northern Samar", "Nueva Ecija", "Nueva Vizcaya",
	"Oriental Mindoro", "Occidental Mindoro", "Palawan", "Pampanga", "Pangasinan",
	"Quezon", "Quirino", "Rizal", "Romblon", "Samar", "Sarangani", "Siquijor", "Sorsogon",
	"South Cotabato", "Southern Leyte", "Sultan Kudarat", "Sulu", "Surigao del Norte",
	"Surigao del Sur", "Tarlac", "Tawi-Tawi", "Zambales", "Zamboanga del Norte",
	"Zamboanga del Sur", "Zamboanga Sibugay",
}

var PhilippineCitiesByProvince = map[string][]string{
	"Metro Manila": {
		"Caloocan", "Las Piñas", "Makati", "Malabon", "Mandaluyong", "Manila", "Marikina",
		"Muntinlupa", "Navotas", "Parañaque", "Pasay", "Pasig", "Pateros", "Quezon City",
		"San Juan", "Taguig", "Valenzuela",
	},
	"Cebu": {
		"Bogo", "Cebu City", "Danao", "Lapu-Lapu", "Mandaue", "Talisay", "Toledo",
	},
	"Davao del Sur": {
		"Davao City", "Digos", "Mati",
	},
	"Laguna": {
		"Biñan", "Cabuyao", "Calamba", "San Pablo", "Santa Rosa", "Sta. Rosa",
	},
	"Batangas": {
		"Batangas City", "Lipa", "Tanauan",
	},
	"Bulacan": {
		"Malolos", "Meycauayan", "San Jose del Monte",
	},
	"Pampanga": {
		"Angeles", "San Fernando",
	},
	"Cavite": {
		"Bacoor", "Dasmariñas", "General Trias", "Imus", "Tagaytay", "Trece Martires",
	},
	"Rizal": {
		"Antipolo", "Cainta", "Rodriguez", "San Mateo", "Taytay",
	},
	"Pangasinan": {
		"Dagupan", "San Carlos", "Urdaneta",
	},
	"Negros Occidental": {
		"Bacolod", "Cadiz", "Escalante", "Himamaylan", "Kabankalan", "La Carlota",
		"Passi", "Roxas", "Sagay", "San Carlos", "Silay", "Talisay", "Victorias",
	},
	"Iloilo": {
		"Iloilo City", "Passi",
	},
}

// Common developers list
var CommonDevelopers = []string{
	"SM Development Corporation",
	"Ayala Land, Inc.",
	"Megaworld Corporation",
	"DMCI Homes",
	"Filinvest Land, Inc.",
	"Shang Properties",
	"Robinsons Land Corporation",
	"Vista Land & Lifescapes",
	"Suntrust Properties",
	"Empire East Land Holdings",
	"Pacific Concord Realty",
	"Other",
}
