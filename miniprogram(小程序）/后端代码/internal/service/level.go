package service

import (
	"errors"

	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/repository"
)

type LevelService struct {
	levelRepo     *repository.LevelRepository
	experimentRepo *repository.ExperimentRepository
	progressRepo  *repository.ProgressRepository
}

func NewLevelService(levelRepo *repository.LevelRepository, expRepo *repository.ExperimentRepository, progressRepo *repository.ProgressRepository) *LevelService {
	return &LevelService{levelRepo: levelRepo, experimentRepo: expRepo, progressRepo: progressRepo}
}

// ListForUser 个性化关卡列表（带 status + best_score）
func (s *LevelService) ListForUser(userID int64) ([]model.LevelView, error) {
	levels, err := s.levelRepo.FindAll()
	if err != nil {
		return nil, err
	}
	prog, _ := s.progressRepo.FindByUser(userID)
	progMap := make(map[int64]*model.UserProgress, len(prog))
	for i := range prog {
		progMap[prog[i].LevelID] = &prog[i]
	}

	views := make([]model.LevelView, 0, len(levels))
	for _, lv := range levels {
		exp, err := s.experimentRepo.FindByID(lv.ExperimentID)
		if err != nil {
			return nil, err
		}
		best := 0
		if p, ok := progMap[lv.ID]; ok {
			best = p.BestScore
		}
		views = append(views, model.LevelView{
			ID:             lv.ID,
			Name:           lv.Name,
			ExperimentCode: exp.Code,
			Status:         computeStatus(lv, progMap),
			Difficulty:     lv.Difficulty,
			BestScore:      best,
		})
	}
	return views, nil
}

// GetExperiment GET /experiments/:id（:id 是 level_id，契约 §9）
func (s *LevelService) GetExperiment(levelID int64) (*model.Experiment, error) {
	lv, err := s.levelRepo.FindByID(levelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	exp, err := s.experimentRepo.FindByID(lv.ExperimentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	// 契约 §9：响应 id 返回 level_id
	exp.ID = levelID
	return exp, nil
}

// computeStatus 计算某关对当前用户的解锁状态
func computeStatus(lv model.Level, prog map[int64]*model.UserProgress) string {
	if p, ok := prog[lv.ID]; ok {
		if p.Passed {
			return "passed"
		}
		if p.Attempts > 0 {
			return "in_progress"
		}
	}
	if lv.PrereqLevelID == nil {
		return "unlocked"
	}
	if pre, ok := prog[*lv.PrereqLevelID]; ok && pre.Passed {
		return "unlocked"
	}
	return "locked"
}
