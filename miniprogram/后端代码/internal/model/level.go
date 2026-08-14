package model

import "time"

// Level 实验关卡（文档 8.3）。一个 experiment 可对应一个 level；level 串成解锁链。
type Level struct {
	ID            int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ExperimentID  int64     `gorm:"index" json:"experiment_id"`
	Name          string    `gorm:"size:64" json:"name"`
	OrderNo       int       `gorm:"index" json:"order_no"` // 1,2,3... 解锁顺序
	Difficulty    int       `json:"difficulty"`
	PrereqLevelID *int64    `gorm:"index" json:"prereq_level_id"` // nil 表示首关
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// LevelView GET /levels 的视图行：关卡 + experiment_code + 个性化 status/best_score
type LevelView struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	ExperimentCode string `json:"experiment_code"`
	Status         string `json:"status"`     // locked / unlocked / in_progress / passed
	Difficulty     int    `json:"difficulty"`
	BestScore      int    `json:"best_score"`
}
