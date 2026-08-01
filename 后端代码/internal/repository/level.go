package repository

import (
	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
)

type LevelRepository struct {
	db *gorm.DB
}

func NewLevelRepository(db *gorm.DB) *LevelRepository {
	return &LevelRepository{db: db}
}

// FindAll 按 order_no 升序返回全部关卡
func (r *LevelRepository) FindAll() ([]model.Level, error) {
	var levels []model.Level
	err := r.db.Order("order_no ASC").Find(&levels).Error
	return levels, err
}

func (r *LevelRepository) FindByID(id int64) (*model.Level, error) {
	var lv model.Level
	if err := r.db.First(&lv, id).Error; err != nil {
		return nil, err
	}
	return &lv, nil
}

// FindNextAfter 取 order_no 大于当前关且最小的下一关（用于解锁）
func (r *LevelRepository) FindNextAfter(levelID int64) (*model.Level, error) {
	var cur model.Level
	if err := r.db.First(&cur, levelID).Error; err != nil {
		return nil, err
	}
	var next model.Level
	err := r.db.Where("order_no > ?", cur.OrderNo).Order("order_no ASC").First(&next).Error
	return &next, err
}

func (r *LevelRepository) Create(lv *model.Level) error {
	return r.db.Create(lv).Error
}

func (r *LevelRepository) Count() (int64, error) {
	var n int64
	err := r.db.Model(&model.Level{}).Count(&n).Error
	return n, err
}
