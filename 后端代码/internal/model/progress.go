package model

import "time"

// UserProgress 用户在某关的进度（文档 8.3）。user_id+level_id 唯一。
type UserProgress struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    int64     `gorm:"uniqueIndex:idx_user_level" json:"user_id"`
	LevelID   int64     `gorm:"uniqueIndex:idx_user_level" json:"level_id"`
	BestScore int       `gorm:"default:0" json:"best_score"`
	LastScore int       `gorm:"default:0" json:"last_score"`
	Attempts  int       `gorm:"default:0" json:"attempts"`
	Passed    bool      `gorm:"default:false" json:"passed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ProgressRecord GET /progress/mine 中每关一行
type ProgressRecord struct {
	ID        int64  `json:"id"`
	LevelName string `json:"level_name"`
	Status    string `json:"status"`
	Score     int    `json:"score"`      // 最近一次得分
	BestScore int    `json:"best_score"`
	Attempts  int    `json:"attempts"`
}

// ProgressView GET /progress/mine 响应
type ProgressView struct {
	Total     int               `json:"total"`
	Passed    int               `json:"passed"`
	AvgScore  int               `json:"avg_score"`
	BestScore int               `json:"best_score"`
	Records   []ProgressRecord  `json:"records"`
}

// ClassStatRow 班级成绩单每行
type ClassStatRow struct {
	UserID    int64  `json:"user_id"`
	Name      string `json:"name"`
	StudentNo string `json:"student_no"`
	BestScore int    `json:"best_score"`
	Attempts  int    `json:"attempts"`
	Passed    bool   `json:"passed"`
}

// ClassStatsView GET /progress/class/:classId 响应
type ClassStatsView struct {
	Class   struct {
		ID      int64  `json:"id"`
		Name    string `json:"name"`
		Teacher string `json:"teacher"`
	} `json:"class"`
	Summary struct {
		AvgScore  float64 `json:"avg_score"`
		PassRate  float64 `json:"pass_rate"`
	} `json:"summary"`
	Rows []ClassStatRow `json:"rows"`
}
