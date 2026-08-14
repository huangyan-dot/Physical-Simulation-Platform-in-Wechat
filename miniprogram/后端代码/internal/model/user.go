package model

import "time"

// User 对应 users 表（文档 8.4）
type User struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	OpenID    string    `gorm:"column:openid;uniqueIndex;size:64;not null" json:"openid"`
	Role      string    `gorm:"size:16;default:student;index" json:"role"` // student / teacher / admin
	Name      *string   `gorm:"size:64" json:"name"`
	StudentNo *string   `gorm:"column:student_no;size:32" json:"student_no"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
