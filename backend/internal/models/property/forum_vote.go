package property

import (
	"time"
)

// ForumVote 论坛投票记录表
type ForumVote struct {
	ID        uint      `gorm:"column:id;type:int unsigned;primaryKey;autoIncrement" json:"id" default:"" comment:"主键ID"`
	PostID    uint      `gorm:"column:post_id;type:int unsigned;not null;index:idx_post_user" json:"post_id" default:"" comment:"帖子ID"`
	UserID    uint      `gorm:"column:user_id;type:int unsigned;not null;index:idx_post_user" json:"user_id" default:"" comment:"投票用户ID"`
	Options   string    `gorm:"column:options;type:text;not null" json:"options" default:"" comment:"选择的选项(JSON数组)"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null;autoCreateTime" json:"created_at" default:"CURRENT_TIMESTAMP" comment:"投票时间"`
}

func (ForumVote) TableName() string {
	return "forum_votes"
}
