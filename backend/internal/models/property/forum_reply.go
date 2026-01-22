package property

import (
	"time"
)

// ForumReply 论坛回复表
type ForumReply struct {
	ID        uint       `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	PostID    uint       `gorm:"column:post_id;type:int unsigned;not null;index:idx_post_id" json:"post_id" default:"" comment:"帖子ID"`
	Content   string     `gorm:"column:content;type:text;not null" json:"content" default:"" comment:"回复内容"`
	UserID    uint       `gorm:"column:user_id;type:int unsigned;not null;index:idx_user_id" json:"user_id" default:"" comment:"回复用户ID"`
	CreatedAt time.Time  `gorm:"column:created_at;type:datetime;not null;autoCreateTime;index:idx_created_at" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"创建时间"`
	UpdatedAt time.Time  `gorm:"column:updated_at;type:datetime;not null;autoUpdateTime" json:"updated_at" default:"CURRENT_TIMESTAMP" comment:"更新时间"`
	DeletedAt *time.Time `gorm:"column:deleted_at;type:datetime;index:idx_deleted_at" json:"deleted_at" default:"null" comment:"删除时间(软删除)"`

	// Relationships (not mapped to DB columns)
	Post *ForumPost `gorm:"foreignKey:PostID" json:"post,omitempty"`
}

func (ForumReply) TableName() string {
	return "forum_replies"
}

// IsDeleted 检查回复是否已删除
func (r *ForumReply) IsDeleted() bool {
	return r.DeletedAt != nil
}
