package model

import "time"

// DefaultDataWeight 综合得分中「测量数据」占的百分比，其余归「自测题目」。
// 默认 60:40（教师可在发布作业后自行调节）。
const DefaultDataWeight = 60

// Assignment 作业（教师发布，关联班级 + 关卡）
type Assignment struct {
	ID      int64 `gorm:"primaryKey;autoIncrement" json:"id"`
	ClassID int64 `gorm:"index;not null" json:"class_id"`
	LevelID int64 `gorm:"not null" json:"level_id"` // 关联哪关实验
	Title   string `gorm:"size:128;not null" json:"title"`
	// DataWeight 测量数据在综合得分里占的百分比（0~100），自测占 100-DataWeight。
	DataWeight int       `gorm:"default:60;not null" json:"data_weight"`
	Deadline   time.Time `json:"deadline"`
	CreatedBy  int64     `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// AssignmentSubmission 作业提交记录。assignment_id+user_id 唯一，允许多次提交取最高分。
//
// 三个分数各自独立保留最好一次：
//   - DataScore/BestDataScore：测量数据得分
//   - QuizScore/BestQuizScore：自测题目得分
//   - Score/BestScore：按作业权重算出的综合得分
//
// BestScore 不是 BestDataScore 与 BestQuizScore 的加权和，而是「综合分最高的那一次提交」
// 的综合分，这样学生看到的最佳成绩始终对应一次真实的完整作答。
type AssignmentSubmission struct {
	ID           int64 `gorm:"primaryKey;autoIncrement" json:"id"`
	AssignmentID int64 `gorm:"uniqueIndex:idx_asg_user" json:"assignment_id"`
	UserID       int64 `gorm:"uniqueIndex:idx_asg_user" json:"user_id"`

	DataScore     int `gorm:"default:0" json:"data_score"`      // 本次测量数据得分
	BestDataScore int `gorm:"default:0" json:"best_data_score"` // 测量数据历史最好
	QuizScore     int `gorm:"default:0" json:"quiz_score"`      // 本次自测得分
	BestQuizScore int `gorm:"default:0" json:"best_quiz_score"` // 自测历史最好
	QuizDone      bool `gorm:"default:false" json:"quiz_done"`  // 是否做过自测

	Score     int  `gorm:"default:0" json:"score"`      // 本次综合得分
	BestScore int  `gorm:"default:0" json:"best_score"` // 综合得分历史最好
	Passed    bool `gorm:"default:false" json:"passed"`
	Attempts  int  `gorm:"default:0" json:"attempts"`

	// 提交时所在地区（单摆等与当地 g 相关的实验用于溯源）
	RegionLabel string `gorm:"size:64" json:"region_label"`

	SubmittedAt *time.Time `json:"submitted_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// AssignmentView 作业列表视图行（带关卡名、实验码供前端展示）
type AssignmentView struct {
	ID             int64     `json:"id"`
	ClassID        int64     `json:"class_id"`
	LevelID        int64     `json:"level_id"`
	Title          string    `json:"title"`
	DataWeight     int       `json:"data_weight"`
	Deadline       time.Time `json:"deadline"`
	CreatedAt      time.Time `json:"created_at"`
	LevelName      string    `json:"level_name"`
	ExperimentCode string    `json:"experiment_code"`
	// 学生视角：提交状态
	Submitted     bool `json:"submitted"`
	BestScore     int  `json:"best_score"`
	BestDataScore int  `json:"best_data_score"`
	BestQuizScore int  `json:"best_quiz_score"`
	QuizDone      bool `json:"quiz_done"`
	Attempts      int  `json:"attempts"`
	Overdue       bool `json:"overdue"`
}

// AssignmentSubmissionView 教师查看提交明细的视图行。
// ComboScore 由 service 按作业当前权重实时算出，教师改权重后无需重算库里的数据。
type AssignmentSubmissionView struct {
	UserID      int64      `json:"user_id"`
	Name        string     `json:"name"`
	StudentNo   string     `json:"student_no"`
	DataScore   int        `json:"data_score"`  // 测量数据得分（历史最好）
	QuizScore   int        `json:"quiz_score"`  // 自测题目得分（历史最好）
	ComboScore  int        `json:"combo_score"` // 综合得分
	BestScore   int        `json:"best_score"`  // 学生自己看到的历史最佳综合分
	QuizDone    bool       `json:"quiz_done"`
	RegionLabel string     `json:"region_label"`
	Attempts    int        `json:"attempts"`
	Passed      bool       `json:"passed"`
	SubmittedAt *time.Time `json:"submitted_at"`
}
