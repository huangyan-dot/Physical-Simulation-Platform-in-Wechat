package model

import "time"

// Class 班级（文档 8.3）
type Class struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	TeacherID int64     `gorm:"index" json:"teacher_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ClassMember 班级成员（学生-班级多对多）。user_id+class_id 唯一，防重复加入。
type ClassMember struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ClassID   int64     `gorm:"uniqueIndex:idx_class_user" json:"class_id"`
	UserID    int64     `gorm:"uniqueIndex:idx_class_user" json:"user_id"`
	JoinedAt  time.Time `json:"joined_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ClassView GET /classes 的视图行：班级 + 教师名 + 成员数
type ClassView struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	TeacherID   int64  `json:"teacher_id"`
	TeacherName string `json:"teacher_name"`
	MemberCount int    `json:"member_count"`
}
