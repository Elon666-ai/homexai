package property

import (
	"time"
)

// RequestTrace 请求流程轨迹表
type RequestTrace struct {
	ID           uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	RequestID    uint      `gorm:"column:request_id;type:int unsigned;not null;index:idx_request_id" json:"request_id" default:"" comment:"请求ID"`
	Action       string    `gorm:"column:action;type:varchar(50);not null" json:"action" default:"" comment:"操作类型"`
	FromStatus   *string   `gorm:"column:from_status;type:varchar(20)" json:"from_status" default:"null" comment:"原状态"`
	ToStatus     *string   `gorm:"column:to_status;type:varchar(20)" json:"to_status" default:"null" comment:"新状态"`
	OperatorID   uint      `gorm:"column:operator_id;type:int unsigned;not null;index:idx_operator_id" json:"operator_id" default:"" comment:"操作者用户ID(主库users表)"`
	OperatorName string    `gorm:"column:operator_name;type:varchar(100);not null" json:"operator_name" default:"" comment:"操作者姓名"`
	OperatorRole string    `gorm:"column:operator_role;type:varchar(50);not null" json:"operator_role" default:"" comment:"操作者角色"`
	Remark       *string   `gorm:"column:remark;type:text" json:"remark" default:"null" comment:"备注说明"`
	IPAddress    *string   `gorm:"column:ip_address;type:varchar(45)" json:"ip_address" default:"null" comment:"IP地址"`
	UserAgent    *string   `gorm:"column:user_agent;type:varchar(255)" json:"user_agent" default:"null" comment:"用户代理"`
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime;index:idx_created_at" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`

	// Relationships
	Request *Request `gorm:"foreignKey:RequestID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"request,omitempty"`
}

func (RequestTrace) TableName() string {
	return "request_traces"
}

// Trace action constants
const (
	TraceActionCreated        = "created"         // 创建请求
	TraceActionStatusChanged  = "status_changed"  // 状态变更
	TraceActionAssigned       = "assigned"        // 分配处理人
	TraceActionReassigned     = "reassigned"      // 重新分配
	TraceActionPriorityChanged = "priority_changed" // 优先级变更
	TraceActionCommented      = "commented"       // 添加备注
	TraceActionAttachmentAdded = "attachment_added" // 添加附件
	TraceActionAttachmentRemoved = "attachment_removed" // 删除附件
	TraceActionResolved       = "resolved"        // 解决
	TraceActionReopened       = "reopened"        // 重新打开
	TraceActionCancelled      = "cancelled"       // 取消
	TraceActionRejected       = "rejected"        // 拒绝
	TraceActionResubmitted    = "resubmitted"     // 重新提交
	TraceActionApproved       = "approved"        // 批准
)

// GetActionLabel returns human-readable label for action
func GetActionLabel(action string) string {
	labels := map[string]string{
		TraceActionCreated:        "Request Created",
		TraceActionStatusChanged:  "Status Changed",
		TraceActionAssigned:       "Assigned",
		TraceActionReassigned:     "Reassigned",
		TraceActionPriorityChanged: "Priority Changed",
		TraceActionCommented:      "Comment Added",
		TraceActionAttachmentAdded: "Attachment Added",
		TraceActionAttachmentRemoved: "Attachment Removed",
		TraceActionResolved:       "Resolved",
		TraceActionReopened:       "Reopened",
		TraceActionCancelled:      "Cancelled",
		TraceActionRejected:       "Rejected",
		TraceActionResubmitted:    "Resubmitted",
		TraceActionApproved:       "Approved",
	}
	if label, ok := labels[action]; ok {
		return label
	}
	return action
}
