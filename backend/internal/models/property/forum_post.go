package property

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// ForumPost 论坛帖子表
type ForumPost struct {
	ID           uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	PostType     string     `gorm:"column:post_type;type:varchar(50);not null;index:idx_post_type" json:"post_type" default:"" comment:"帖子类型(vote,activity,help,marketplace,social,rent)"`
	Title        string     `gorm:"column:title;type:varchar(200);not null" json:"title" default:"" comment:"标题"`
	Content      string     `gorm:"column:content;type:text;not null" json:"content" default:"" comment:"内容"`
	TemplateData JSONB      `gorm:"column:template_data;type:text" json:"template_data" default:"null" comment:"模板数据(JSON)"`
	UserID       uint       `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" default:"" comment:"发帖用户ID"`
	ViewCount    int64      `gorm:"column:view_count;type:bigint;not null" json:"view_count" default:"0" comment:"浏览量"`
	ReplyCount   int64      `gorm:"column:reply_count;type:bigint;not null" json:"reply_count" default:"0" comment:"回复数"`
	IsPinned     bool       `gorm:"column:is_pinned;type:tinyint(1);not null;index:idx_is_pinned" json:"is_pinned" default:"0" comment:"是否置顶"`
	PinnedAt     *time.Time `gorm:"column:pinned_at;type:datetime;index:idx_pinned_at" json:"pinned_at" default:"null" comment:"置顶时间"`
	IsLocked     bool       `gorm:"column:is_locked;type:tinyint(1);not null" json:"is_locked" default:"0" comment:"是否锁定(投票截止后自动锁定)"`
	IsEdited     bool       `gorm:"column:is_edited;type:tinyint(1);not null" json:"is_edited" default:"0" comment:"是否已编辑"`
	EditedAt     *time.Time `gorm:"column:edited_at;type:datetime" json:"edited_at" default:"null" comment:"最后编辑时间"`
	DeletedAt    *time.Time `gorm:"column:deleted_at;type:datetime;index:idx_deleted_at" json:"deleted_at" default:"null" comment:"删除时间(软删除)"`
	CreatedAt    time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime;index:idx_created_at" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt    time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`

	// Relationships
	Replies []ForumReply `gorm:"foreignKey:PostID;constraint:OnDelete:CASCADE" json:"replies,omitempty"`
}

func (ForumPost) TableName() string {
	return "forum_posts"
}

// Post type constants
const (
	PostTypeVote        = "vote"        // 投票贴
	PostTypeActivity    = "activity"    // 活动召集贴
	PostTypeHelp        = "help"        // 求助贴
	PostTypeMarketplace = "marketplace" // 二手物品转让贴
	PostTypeSocial      = "social"      // 交友贴
	PostTypeRent        = "rent"        // 出租/转租贴
)

// JSONB 用于存储JSON数据
type JSONB map[string]interface{}

// Value 实现driver.Valuer接口
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现sql.Scanner接口
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// VoteTemplateData 投票贴模板数据
type VoteTemplateData struct {
	Options       []string     `json:"options"`                // 投票选项
	AllowMultiple bool         `json:"allow_multiple"`         // 是否允许多选
	Deadline      *time.Time   `json:"deadline"`               // 截止时间
	VoteResults   []VoteResult `json:"vote_results,omitempty"` // 投票结果
}

// VoteResult 投票结果
type VoteResult struct {
	Option string `json:"option"` // 选项
	Count  int64  `json:"count"`  // 投票数
}

// ActivityTemplateData 活动召集贴模板数据
type ActivityTemplateData struct {
	EventTime            *time.Time `json:"event_time"`            // 活动时间
	Location             string     `json:"location"`              // 地点
	RegistrationDeadline *time.Time `json:"registration_deadline"` // 报名截止时间
	MaxParticipants      *int       `json:"max_participants"`      // 人数限制
}

// HelpTemplateData 求助贴模板数据
type HelpTemplateData struct {
	Urgency string  `json:"urgency"` // 紧急程度: low, medium, high
	Contact *string `json:"contact"` // 联系方式
}

// MarketplaceTemplateData 二手物品转让贴模板数据
type MarketplaceTemplateData struct {
	Price     string  `json:"price"`     // 价格
	Condition string  `json:"condition"` // 状态: new, like_new, good, fair
	Contact   *string `json:"contact"`   // 联系方式
}

// SocialTemplateData 交友贴模板数据
type SocialTemplateData struct {
	Introduction string   `json:"introduction"` // 自我介绍
	Interests    []string `json:"interests"`    // 兴趣爱好
	Contact      *string  `json:"contact"`      // 联系方式
}

// RentTemplateData 出租/转租贴模板数据
type RentTemplateData struct {
	RentType        string  `json:"rent_type"`         // 出租类型: unit, room, parking_slot
	UnitID          *uint   `json:"unit_id,omitempty"` // 关联的unit ID（可选）
	ParkingSlotID   *uint   `json:"parking_slot_id,omitempty"` // 关联的parking slot ID（可选）
	UnitNumber      *string `json:"unit_number,omitempty"`     // 房号（如果未关联unit）
	ParkingSlotNumber *string `json:"parking_slot_number,omitempty"` // 车位号（如果未关联parking slot）
	Price           string  `json:"price"`            // 租金
	Description     string  `json:"description"`      // 详细描述
	Contact         *string `json:"contact"`          // 联系方式
}

// IsDeleted 检查帖子是否已删除
func (p *ForumPost) IsDeleted() bool {
	return p.DeletedAt != nil
}

// IsLockedPost 检查帖子是否已锁定
func (p *ForumPost) IsLockedPost() bool {
	return p.IsLocked
}
