package repository

import (
	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
)

type ExperimentRepository struct {
	db *gorm.DB
}

func NewExperimentRepository(db *gorm.DB) *ExperimentRepository {
	return &ExperimentRepository{db: db}
}

func (r *ExperimentRepository) FindByID(id int64) (*model.Experiment, error) {
	var e model.Experiment
	if err := r.db.First(&e, id).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ExperimentRepository) FindByCode(code string) (*model.Experiment, error) {
	var e model.Experiment
	if err := r.db.Where("code = ?", code).First(&e).Error; err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *ExperimentRepository) Create(e *model.Experiment) error {
	return r.db.Create(e).Error
}

func (r *ExperimentRepository) Count() (int64, error) {
	var n int64
	err := r.db.Model(&model.Experiment{}).Count(&n).Error
	return n, err
}

// FindAllMeta 全部实验的元数据列表（契约 §17：不含 config/target）
func (r *ExperimentRepository) FindAllMeta() ([]model.ExperimentSummary, error) {
	var rows []model.ExperimentSummary
	err := r.db.Model(&model.Experiment{}).
		Select("id", "code", "name", "render_mode").
		Order("id ASC").
		Scan(&rows).Error
	return rows, err
}
