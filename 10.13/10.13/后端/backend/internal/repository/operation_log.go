package repository

import (
	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
)

type OperationLogRepository struct {
	db *gorm.DB
}

func NewOperationLogRepository(db *gorm.DB) *OperationLogRepository {
	return &OperationLogRepository{db: db}
}

func (r *OperationLogRepository) Create(l *model.OperationLog) error {
	return r.db.Create(l).Error
}

// LogQuery 审计查询过滤参数。零值表示不过滤该字段。
type LogQuery struct {
	UserID  int64
	Action  string
	LevelID int64
	Page    int // 从 1 起
	Size    int // 每页条数
}

// List 分页查询审计日志，按创建时间倒序。返回当前页 + 总数。
func (r *OperationLogRepository) List(q LogQuery) ([]model.OperationLog, int64, error) {
	tx := r.db.Model(&model.OperationLog{})
	if q.UserID > 0 {
		tx = tx.Where("user_id = ?", q.UserID)
	}
	if q.Action != "" {
		tx = tx.Where("action = ?", q.Action)
	}
	if q.LevelID > 0 {
		tx = tx.Where("level_id = ?", q.LevelID)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if q.Page < 1 {
		q.Page = 1
	}
	if q.Size <= 0 || q.Size > 200 {
		q.Size = 50
	}

	var logs []model.OperationLog
	err := tx.Order("created_at DESC").
		Offset((q.Page - 1) * q.Size).Limit(q.Size).
		Find(&logs).Error
	return logs, total, err
}
