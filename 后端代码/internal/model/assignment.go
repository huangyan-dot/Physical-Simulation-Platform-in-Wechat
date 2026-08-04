package model

import "time"

// Assignment 作业（教师发布，关联班级 + 关卡）
type Assignment struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ClassID   int64     `gorm:"index;not null" json:"class_id"`
	LevelID   int64     `gorm:"not null" json:"level_id"` // 关联哪关实验
	Title     string    `gorm:"size:128;not null" json:"title"`
	Deadline  time.Time `json:"deadline"`
	CreatedBy int64     `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AssignmentSubmission 作业提交记录。assignment_id+user_id 唯一，允许多次提交取最高分。
type AssignmentSubmission struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	AssignmentID int64      `gorm:"uniqueIndex:idx_asg_user" json:"assignment_id"`
	UserID       int64      `gorm:"uniqueIndex:idx_asg_user" json:"user_id"`
	Score        int        `gorm:"default:0" json:"score"`        // 本次得分
	BestScore    int        `gorm:"default:0" json:"best_score"`   // 历史最佳
	Passed       bool       `gorm:"default:false" json:"passed"`
	Attempts     int        `gorm:"default:0" json:"attempts"`
	SubmittedAt  *time.Time `json:"submitted_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// AssignmentView 作业列表视图行（带关卡名、实验码供前端展示）
type AssignmentView struct {
	ID           int64      `json:"id"`
	ClassID      int64      `json:"class_id"`
	LevelID      int64      `json:"level_id"`
	Title        string     `json:"title"`
	Deadline     time.Time  `json:"deadline"`
	CreatedAt    time.Time  `json:"created_at"`
	LevelName    string     `json:"level_name"`
	ExperimentCode string  `json:"experiment_code"`
	// 学生视角：提交状态
	Submitted  bool       `json:"submitted"`
	BestScore  int        `json:"best_score"`
	Attempts   int        `json:"attempts"`
	Overdue    bool       `json:"overdue"`
}

// AssignmentSubmissionView 教师查看提交明细的视图行
type AssignmentSubmissionView struct {
	UserID      int64      `json:"user_id"`
	Name        string     `json:"name"`
	StudentNo   string     `json:"student_no"`
	BestScore   int        `json:"best_score"`
	Attempts    int        `json:"attempts"`
	Passed      bool       `json:"passed"`
	SubmittedAt *time.Time `json:"submitted_at"`
}
