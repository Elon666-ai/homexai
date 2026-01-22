package master

import (
	"time"
)

// ImportTask 导入任务表
type ImportTask struct {
	ID          uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	PropertyID  uint       `gorm:"column:property_id;type:int unsigned;not null;index:idx_property_id" json:"property_id" default:"" comment:"物业ID"`
	Subdomain   string     `gorm:"column:subdomain;type:varchar(50);not null;index:idx_subdomain" json:"subdomain" default:"" comment:"子域名"`
	FileName    string     `gorm:"column:file_name;type:varchar(255);not null" json:"file_name" default:"" comment:"文件名"`
	FilePath    string     `gorm:"column:file_path;type:varchar(500);not null" json:"file_path" default:"" comment:"文件路径"`
	TaskType    string     `gorm:"column:task_type;type:varchar(20);not null" json:"task_type" default:"unit" comment:"任务类型(unit,parking,user,bill,landlord,tenant,other)"`
	TotalRows   int        `gorm:"column:total_rows;type:int;not null" json:"total_rows" default:"0" comment:"总行数"`
	SuccessRows int        `gorm:"column:success_rows;type:int;not null" json:"success_rows" default:"0" comment:"成功行数"`
	FailedRows  int        `gorm:"column:failed_rows;type:int;not null" json:"failed_rows" default:"0" comment:"失败行数"`
	Status      string     `gorm:"column:status;type:varchar(20);not null;index:idx_status" json:"status" default:"pending" comment:"任务状态(pending,uploaded,processing,completed,failed)"`
	ErrorLog    *string    `gorm:"column:error_log;type:text" json:"error_log" default:"null" comment:"错误日志"`
	CreatedBy   uint       `gorm:"column:created_by;type:int unsigned;not null" json:"created_by" default:"" comment:"创建者用户ID"`
	CreatedAt   time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime;index:idx_created_at" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`
	CompletedAt *time.Time `gorm:"column:completed_at;type:datetime" json:"completed_at" default:"null" comment:"完成时间"`

	// Relationships
	Property Property `gorm:"foreignKey:PropertyID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"property,omitempty"`
	Creator  User     `gorm:"foreignKey:CreatedBy;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"creator,omitempty"`
}

func (ImportTask) TableName() string {
	return "import_tasks"
}

// Import task type constants
const (
	ImportTaskTypeUnit     = "unit"
	ImportTaskTypeParking  = "parking"
	ImportTaskTypeUser     = "user"
	ImportTaskTypeBill     = "bill"
	ImportTaskTypeLandlord = "landlord"
	ImportTaskTypeTenant   = "tenant"
	ImportTaskTypeOther    = "other"
)

// Import task status constants
const (
	ImportTaskStatusPending    = "pending"
	ImportTaskStatusUploaded   = "uploaded"    // File uploaded, ready for import
	ImportTaskStatusProcessing = "processing"
	ImportTaskStatusCompleted  = "completed"
	ImportTaskStatusFailed     = "failed"
)
