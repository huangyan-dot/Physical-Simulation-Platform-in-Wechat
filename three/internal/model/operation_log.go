package model

import "time"

// OperationLog 操作审计表（文档 8.3）。每次提交实验、登录、班级操作等都写一条。
type OperationLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"index;index:idx_user_time,priority:1" json:"user_id"`
	Action    string    `gorm:"size:32;index;index:idx_action_time,priority:1" json:"action"` // login / submit / class.create / class.delete / class.rename / class.member.add / class.member.remove
	LevelID   *int64    `gorm:"index" json:"level_id"`
	Score     *int      `json:"score"`
	Detail    string    `gorm:"type:text" json:"detail"`
	CreatedAt time.Time `gorm:"index:idx_user_time,priority:2;index:idx_action_time,priority:2" json:"created_at"`
}
