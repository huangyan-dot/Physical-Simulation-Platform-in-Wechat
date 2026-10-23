package repository

import (
	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
)

type ProgressRepository struct {
	db *gorm.DB
}

func NewProgressRepository(db *gorm.DB) *ProgressRepository {
	return &ProgressRepository{db: db}
}

// FindByUser 某用户全部进度
func (r *ProgressRepository) FindByUser(userID int64) ([]model.UserProgress, error) {
	var ps []model.UserProgress
	err := r.db.Where("user_id = ?", userID).Find(&ps).Error
	return ps, err
}

// FindByUserLevel 某用户某关进度，未找到返回 nil,nil
func (r *ProgressRepository) FindByUserLevel(userID, levelID int64) (*model.UserProgress, error) {
	var p model.UserProgress
	err := r.db.Where("user_id = ? AND level_id = ?", userID, levelID).First(&p).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

// FindByUsers 批量查多用户进度（班级成绩单用）
func (r *ProgressRepository) FindByUsers(userIDs []int64) ([]model.UserProgress, error) {
	var ps []model.UserProgress
	if len(userIDs) == 0 {
		return ps, nil
	}
	err := r.db.Where("user_id IN ?", userIDs).Find(&ps).Error
	return ps, err
}

// Upsert 提交后写回：attempts+1、更新 last/best/passed
func (r *ProgressRepository) Upsert(p *model.UserProgress) error {
	return r.db.Save(p).Error // Save：有主键更新，无主键插入
}

// Ensure 创建一条空进度记录（attempts=0）
func (r *ProgressRepository) Ensure(userID, levelID int64) (*model.UserProgress, error) {
	p, err := r.FindByUserLevel(userID, levelID)
	if err != nil {
		return nil, err
	}
	if p != nil {
		return p, nil
	}
	p = &model.UserProgress{UserID: userID, LevelID: levelID}
	if err := r.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}
