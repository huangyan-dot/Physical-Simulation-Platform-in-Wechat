package model

import "time"

// Experiment 实验定义（文档 8.3）。config/target 为 MySQL JSON 列。
type Experiment struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Code       string    `gorm:"size:32;uniqueIndex" json:"code"` // newton_ring / oscilloscope / pendulum
	Name       string    `gorm:"size:64" json:"name"`
	RenderMode string    `gorm:"size:32;default:mixed_3d_2d" json:"render_mode"`
	Config     JSON      `gorm:"type:json" json:"config"` // 随实验类型不同
	Target     JSON      `gorm:"type:json" json:"target"` // 评分目标
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
