package property

import "time"

// Facility 公共设施，例如台球室、健身房、多功能厅等
type Facility struct {
	ID uint `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" comment:"主键ID"`

	// 基本信息
	Name        string  `gorm:"column:name;type:varchar(100);not null;index:idx_name" json:"name" comment:"设施名称"`
	Type        string  `gorm:"column:type;type:varchar(20);not null;index:idx_type" json:"type" comment:"设施类型(billiard_room,game_room,meeting_room,activity_room,sky_lounge)"`
	Description *string `gorm:"column:description;type:text" json:"description,omitempty" comment:"设施描述"`

	// 可预订时间段（按工作日分别设置）
	WorkingStartTime *string `gorm:"column:working_start_time;type:varchar(8)" json:"working_start_time,omitempty" comment:"工作日（周一到周五）可预订开始时间，格式 HH:MM:SS"`
	WorkingEndTime   *string `gorm:"column:working_end_time;type:varchar(8)" json:"working_end_time,omitempty" comment:"工作日（周一到周五）可预订结束时间，格式 HH:MM:SS"`
	SaturdayStartTime *string `gorm:"column:saturday_start_time;type:varchar(8)" json:"saturday_start_time,omitempty" comment:"周六可预订开始时间，格式 HH:MM:SS"`
	SaturdayEndTime   *string `gorm:"column:saturday_end_time;type:varchar(8)" json:"saturday_end_time,omitempty" comment:"周六可预订结束时间，格式 HH:MM:SS"`
	SundayStartTime   *string `gorm:"column:sunday_start_time;type:varchar(8)" json:"sunday_start_time,omitempty" comment:"周日可预订开始时间，格式 HH:MM:SS"`
	SundayEndTime     *string `gorm:"column:sunday_end_time;type:varchar(8)" json:"sunday_end_time,omitempty" comment:"周日可预订结束时间，格式 HH:MM:SS"`
	
	// 保留旧字段以兼容（已废弃，将在迁移后删除）
	AvailableStartTime *string `gorm:"column:available_start_time;type:varchar(8)" json:"available_start_time,omitempty" comment:"已废弃：使用 working_start_time 等字段"`
	AvailableEndTime   *string `gorm:"column:available_end_time;type:varchar(8)" json:"available_end_time,omitempty" comment:"已废弃：使用 working_end_time 等字段"`
	Notice             *string `gorm:"column:notice;type:text" json:"notice,omitempty" comment:"注意事项/使用规则"`

	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at"`

	Reservations []FacilityReservation `gorm:"foreignKey:FacilityID" json:"reservations,omitempty"`
}

func (Facility) TableName() string {
	return "facilities"
}

// Facility type constants
const (
	FacilityTypeBilliardRoom = "billiard_room" // 台球室
	FacilityTypeGameRoom     = "game_room"     // 游戏室
	FacilityTypeMeetingRoom  = "meeting_room"  // 会议室
	FacilityTypeActivityRoom = "activity_room" // 活动室
	FacilityTypeSkyLounge    = "sky_lounge"    // Sky Lounge
)

// FacilityTypes returns all facility types
var FacilityTypes = []string{
	FacilityTypeBilliardRoom,
	FacilityTypeGameRoom,
	FacilityTypeMeetingRoom,
	FacilityTypeActivityRoom,
	FacilityTypeSkyLounge,
}

// FacilityReservation 公共设施预定记录
type FacilityReservation struct {
	ID         uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" comment:"主键ID"`
	FacilityID uint      `gorm:"column:facility_id;type:int unsigned;not null;index:idx_facility_id" json:"facility_id" comment:"设施ID"`
	UserID     uint      `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" comment:"预定人用户ID(主库 users 表)"`
	UnitID     *uint     `gorm:"column:unit_id;type:int unsigned;index:idx_unit_id" json:"unit_id,omitempty" comment:"预定人所属房源，可选"`
	StartTime  time.Time `gorm:"column:start_time;type:datetime;not null;index:idx_start_end" json:"start_time" comment:"预定开始时间"`
	EndTime    time.Time `gorm:"column:end_time;type:datetime;not null;index:idx_start_end" json:"end_time" comment:"预定结束时间"`

	// 状态: pending(待确认), approved(已确认), cancelled(已取消), rejected(已拒绝), completed(已完成)
	Status string `gorm:"column:status;type:varchar(20);not null;default:'pending';index:idx_status" json:"status" comment:"预定状态"`

	// 审批 & 完成信息（物业端审批/确认使用）
	ApprovedBy     *uint      `gorm:"column:approved_by;type:int unsigned" json:"approved_by,omitempty" comment:"审批通过人用户ID(主库 users 表)"`
	ApprovedAt     *time.Time `gorm:"column:approved_at;type:datetime" json:"approved_at,omitempty" comment:"审批通过时间"`
	RejectedBy     *uint      `gorm:"column:rejected_by;type:int unsigned" json:"rejected_by,omitempty" comment:"拒绝人用户ID"`
	RejectedAt     *time.Time `gorm:"column:rejected_at;type:datetime" json:"rejected_at,omitempty" comment:"拒绝时间"`
	RejectedReason *string    `gorm:"column:rejected_reason;type:text" json:"rejected_reason,omitempty" comment:"拒绝原因"`
	CancelledBy    *uint      `gorm:"column:cancelled_by;type:int unsigned" json:"cancelled_by,omitempty" comment:"取消人用户ID"`
	CancelledAt    *time.Time `gorm:"column:cancelled_at;type:datetime" json:"cancelled_at,omitempty" comment:"取消时间"`
	CompletedAt    *time.Time `gorm:"column:completed_at;type:datetime" json:"completed_at,omitempty" comment:"实际使用完成时间"`

	Notes *string `gorm:"column:notes;type:text" json:"notes,omitempty" comment:"预定备注/用途说明"`

	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime;index:idx_created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at"`

	Facility Facility `gorm:"foreignKey:FacilityID" json:"facility"`
	Unit     *Unit    `gorm:"foreignKey:UnitID" json:"unit,omitempty"`
}

func (FacilityReservation) TableName() string {
	return "facility_reservations"
}
