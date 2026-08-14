package model

import "time"

// OperationLog 操作审计表（文档 8.3）。每次提交实验、登录等都写一条。
type OperationLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"index" json:"user_id"`
	Action    string    `gorm:"size:32;index" json:"action"` // submit / login / ...
	LevelID   *int64    `gorm:"index" json:"level_id"`
	Score     *int      `json:"score"`
	Detail    string    `gorm:"type:text" json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}
