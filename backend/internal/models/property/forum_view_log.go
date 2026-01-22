package property

import (
	"time"
)

// ForumViewLog 论坛浏览记录表（用于防止重复刷浏览量）
type ForumViewLog struct {
	ID       uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	PostID   uint      `gorm:"column:post_id;type:int unsigned;not null;index:idx_post_user" json:"post_id" default:"" comment:"帖子ID"`
	UserID   uint      `gorm:"column:user_id;type:int unsigned;not null;index:idx_post_user" json:"user_id" default:"" comment:"用户ID"`
	ViewedAt time.Time `gorm:"column:viewed_at;type:datetime;not null;autoCreateTime;index:idx_viewed_at" json:"viewed_at" default:"CURRENT_TIMESTAMP" comment:"浏览时间"`
}

func (ForumViewLog) TableName() string {
	return "forum_view_logs"
}
