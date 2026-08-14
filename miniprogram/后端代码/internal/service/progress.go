package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/repository"
)

type ProgressService struct {
	progressRepo *repository.ProgressRepository
	levelRepo    *repository.LevelRepository
	expRepo      *repository.ExperimentRepository
	logRepo      *repository.OperationLogRepository
	classRepo    *repository.ClassRepository
	userRepo     *repository.UserRepository
}

func NewProgressService(
	progressRepo *repository.ProgressRepository,
	levelRepo *repository.LevelRepository,
	expRepo *repository.ExperimentRepository,
	logRepo *repository.OperationLogRepository,
	classRepo *repository.ClassRepository,
	userRepo *repository.UserRepository,
) *ProgressService {
	return &ProgressService{
		progressRepo: progressRepo, levelRepo: levelRepo, expRepo: expRepo,
		logRepo: logRepo, classRepo: classRepo, userRepo: userRepo,
	}
}

// Mine GET /progress/mine
// role 为 teacher/admin 时同样不做解锁限制，与 /levels 保持一致。
func (s *ProgressService) Mine(userID int64, role string) (*model.ProgressView, error) {
	levels, err := s.levelRepo.FindAll()
	if err != nil {
		return nil, err
	}
	prog, _ := s.progressRepo.FindByUser(userID)
	progMap := make(map[int64]*model.UserProgress, len(prog))
	for i := range prog {
		progMap[prog[i].LevelID] = &prog[i]
	}

	view := &model.ProgressView{Total: len(levels), Records: []model.ProgressRecord{}}
	passedCnt, sumBest, maxBest := 0, 0, 0
	for _, lv := range levels {
		status := computeStatus(lv, progMap, role)
		rec := model.ProgressRecord{ID: lv.ID, LevelName: lv.Name, Status: status, Score: 0, BestScore: 0, Attempts: 0}
		if p, ok := progMap[lv.ID]; ok {
			rec.Score = p.LastScore
			rec.BestScore = p.BestScore
			rec.Attempts = p.Attempts
			if p.Passed {
				passedCnt++
			}
			if p.BestScore > maxBest {
				maxBest = p.BestScore
			}
			sumBest += p.BestScore
		}
		view.Records = append(view.Records, rec)
	}
	view.Passed = passedCnt
	view.BestScore = maxBest
	if len(levels) > 0 {
		view.AvgScore = int(math.Round(float64(sumBest) / float64(len(levels))))
	}
	return view, nil
}

// ClassProgress GET /progress/class/:classId
func (s *ProgressService) ClassProgress(classID, requesterID int64, role string) (*model.ClassStatsView, error) {
	// 权限：本班教师 / admin
	c, err := s.classRepo.FindByID(classID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if role != "admin" && c.TeacherID != requesterID {
		return nil, ErrForbidden
	}

	memberIDs, err := s.classRepo.MemberUserIDs(classID)
	if err != nil {
		return nil, err
	}
	allProg, err := s.progressRepo.FindByUsers(memberIDs)
	if err != nil {
		return nil, err
	}
	// 按 user 聚合
	type agg struct {
		bestScore int
		attempts int
		passed   bool
	}
	aggMap := make(map[int64]*agg)
	for _, p := range allProg {
		a := aggMap[p.UserID]
		if a == nil {
			a = &agg{}
			aggMap[p.UserID] = a
		}
		if p.BestScore > a.bestScore {
			a.bestScore = p.BestScore
		}
		a.attempts += p.Attempts
		if p.Passed {
			a.passed = true
		}
	}

	// 教师名
	teacherName := ""
	if t, err := s.userRepo.FindByID(c.TeacherID); err == nil && t.Name != nil {
		teacherName = *t.Name
	}

	view := &model.ClassStatsView{}
	view.Class.ID = c.ID
	view.Class.Name = c.Name
	view.Class.Teacher = teacherName
	view.Rows = []model.ClassStatRow{}

	passCnt := 0
	scoreSum := 0
	for _, uid := range memberIDs {
		var name, studentNo string
		if u, err := s.userRepo.FindByID(uid); err == nil {
			if u.Name != nil {
				name = *u.Name
			}
			if u.StudentNo != nil {
				studentNo = *u.StudentNo
			}
		}
		row := model.ClassStatRow{UserID: uid, Name: name, StudentNo: studentNo}
		if a, ok := aggMap[uid]; ok {
			row.BestScore = a.bestScore
			row.Attempts = a.attempts
			row.Passed = a.passed
			if a.passed {
				passCnt++
			}
			scoreSum += a.bestScore
		}
		view.Rows = append(view.Rows, row)
	}

	if len(memberIDs) > 0 {
		view.Summary.AvgScore = math.Round(float64(scoreSum)/float64(len(memberIDs))*10) / 10
		view.Summary.PassRate = math.Round(float64(passCnt)/float64(len(memberIDs))*1000) / 1000
	}
	return view, nil
}

// SubmitResult POST /progress/submit 响应
type SubmitResult struct {
	Score           int   `json:"score"`
	Passed          bool  `json:"passed"`
	BestScore       int   `json:"best_score"`
	UnlockedLevelID *int64 `json:"unlocked_level_id"`
}

// Submit POST /progress/submit
func (s *ProgressService) Submit(userID, levelID int64, experimentCode string, readingsRaw json.RawMessage) (*SubmitResult, error) {
	lv, err := s.levelRepo.FindByID(levelID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	exp, err := s.expRepo.FindByID(lv.ExperimentID)
	if err != nil {
		return nil, ErrNotFound
	}

	score, passed, detail, err := scoreByExperiment(exp.Code, readingsRaw, exp.Target)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	// 写进度
	prog, err := s.progressRepo.Ensure(userID, levelID)
	if err != nil {
		return nil, err
	}
	prog.Attempts++
	prog.LastScore = score
	if score > prog.BestScore {
		prog.BestScore = score
	}
	if passed {
		prog.Passed = true
	}
	if err := s.progressRepo.Upsert(prog); err != nil {
		return nil, err
	}

	// 审计日志
	detailFull := fmt.Sprintf("%s | readings=%s", detail, string(readingsRaw))
	logEntry := &model.OperationLog{
		UserID: userID, Action: "submit", LevelID: &levelID, Score: &score, Detail: detailFull,
	}
	_ = s.logRepo.Create(logEntry)

	// 解锁下一关
	var unlocked *int64
	if passed {
		if next, err := s.levelRepo.FindNextAfter(levelID); err == nil {
			unlocked = &next.ID
		}
	}

	return &SubmitResult{
		Score:           score,
		Passed:          passed,
		BestScore:       prog.BestScore,
		UnlockedLevelID: unlocked,
	}, nil
}
