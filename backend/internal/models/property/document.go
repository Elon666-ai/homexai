package property

import (
	"time"
)

// Document 通用文档表（支持多种实体类型的文档上传）
// 可用于：SPA授权文档、租客缴费凭证、业主合同、维修请求附件等
type Document struct {
	ID           uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	EntityType   string     `gorm:"column:entity_type;type:varchar(50);not null;index:idx_entity" json:"entity_type" default:"" comment:"实体类型(spa,tenant,landlord,payment,request,bill,etc)"`
	EntityID     uint       `gorm:"column:entity_id;type:int unsigned;not null;index:idx_entity" json:"entity_id" default:"" comment:"实体ID"`
	DocumentType string     `gorm:"column:document_type;type:varchar(50);not null;index:idx_doc_type" json:"document_type" default:"" comment:"文档类型(authorization,payment_receipt,contract,id_copy,photo,etc)"`
	DocumentName string     `gorm:"column:document_name;type:varchar(255);not null" json:"document_name" default:"" comment:"文档名称"`
	DocumentPath string     `gorm:"column:document_path;type:varchar(500);not null" json:"document_path" default:"" comment:"文档存储路径"`
	FileSize     *int64     `gorm:"column:file_size;type:bigint" json:"file_size" default:"null" comment:"文件大小(bytes)"`
	MimeType     *string    `gorm:"column:mime_type;type:varchar(100)" json:"mime_type" default:"null" comment:"MIME类型"`
	UploadedBy   uint       `gorm:"column:uploaded_by;type:int unsigned;not null;index:idx_uploaded_by" json:"uploaded_by" default:"" comment:"上传者用户ID"`
	UploadedAt   time.Time  `gorm:"column:uploaded_at;type:datetime;not null;autoCreateTime" json:"uploaded_at" default:"CURRENT_TIMESTAMP" comment:"上传时间"`
	ExpiresAt    *time.Time `gorm:"column:expires_at;type:date" json:"expires_at" default:"null" comment:"文档过期时间"`
	Notes        *string    `gorm:"column:notes;type:text" json:"notes" default:"null" comment:"备注"`
	IsActive     bool       `gorm:"column:is_active;type:tinyint(1);not null" json:"is_active" default:"1" comment:"是否有效"`
	CreatedAt    time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`
}

func (Document) TableName() string {
	return "documents"
}

// Entity type constants
const (
	DocEntitySPA          = "spa"          // SPA授权文档
	DocEntityTenant       = "tenant"       // 租客文档
	DocEntityLandlord     = "landlord"     // 业主文档
	DocEntityPayment      = "payment"      // 支付凭证
	DocEntityBill         = "bill"         // 账单附件
	DocEntityRequest      = "request"      // 请求附件
	DocEntityUnit         = "unit"         // 房源文档
	DocEntityParking      = "parking"      // 停车位文档
	DocEntityAnnouncement = "announcement" // 公告附件
	DocEntityComplaint    = "complaint"    // 投诉附件
	DocEntityForumPost    = "forum_post"   // 论坛帖子图片
	DocEntityFacility     = "facility"     // 设施照片
)

// Document type constants
const (
	DocTypeAuthorization      = "authorization"        // 授权书
	DocTypePaymentReceipt     = "payment_receipt"      // 支付凭证
	DocTypeContract           = "contract"             // 合同
	DocTypeIDCopy             = "id_copy"              // 身份证复印件
	DocTypeNotarized          = "notarized"            // 公证文件
	DocTypePhoto              = "photo"                // 照片
	DocTypeInvoice            = "invoice"              // 发票
	DocTypePropertyCertificate = "property_certificate" // 房产证
	DocTypeOwnershipCertificate = "ownership_certificate" // 产权证
	DocTypeVehicleCROR         = "vehicle_cr_or"        // 汽车CR/OR
	DocTypeOther              = "other"                // 其他
)

// IsExpired 检查文档是否过期
func (d *Document) IsExpired() bool {
	if d.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*d.ExpiresAt)
}

// IsValid 检查文档是否有效
func (d *Document) IsValid() bool {
	return d.IsActive && !d.IsExpired()
}
