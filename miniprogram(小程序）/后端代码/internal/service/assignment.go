package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/repository"
)

type AssignmentService struct {
	asgRepo      *repository.AssignmentRepository
	classRepo    *repository.ClassRepository
	levelRepo    *repository.LevelRepository
	expRepo      *repository.ExperimentRepository
	progressRepo *repository.ProgressRepository
	logRepo      *repository.OperationLogRepository
	userRepo     *repository.UserRepository
}

func NewAssignmentService(
	asgRepo *repository.AssignmentRepository,
	classRepo *repository.ClassRepository,
	levelRepo *repository.LevelRepository,
	expRepo *repository.ExperimentRepository,
	progressRepo *repository.ProgressRepository,
	logRepo *repository.OperationLogRepository,
	userRepo *repository.UserRepository,
) *AssignmentService {
	return &AssignmentService{
		asgRepo: asgRepo, classRepo: classRepo, levelRepo: levelRepo,
		expRepo: expRepo, progressRepo: progressRepo, logRepo: logRepo, userRepo: userRepo,
	}
}

// Create 发布作业（教师）
func (s *AssignmentService) Create(classID, levelID int64, title string, deadline time.Time, createdBy int64) (*model.Assignment, error) {
	// 校验关卡存在
	if _, err := s.levelRepo.FindByID(levelID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	a := &model.Assignment{
		ClassID:   classID,
		LevelID:   levelID,
		Title:     title,
		Deadline:  deadline,
		CreatedBy: createdBy,
	}
	if err := s.asgRepo.Create(a); err != nil {
		return nil, err
	}
	return a, nil
}

// ListByClass 班级作业列表
func (s *AssignmentService) ListByClass(classID int64) ([]model.AssignmentView, error) {
	rows, err := s.asgRepo.ListByClass(classID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.AssignmentView{}
	}
	return rows, nil
}

// ListByUser 学生视角：自己所有班级的作业
func (s *AssignmentService) ListByUser(userID int64) ([]model.AssignmentView, error) {
	rows, err := s.asgRepo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []model.AssignmentView{}
	}
	return rows, nil
}

// AssignmentSubmitResult 作业提交响应
type AssignmentSubmitResult struct {
	Score     int  `json:"score"`
	Passed    bool `json:"passed"`
	BestScore int  `json:"best_score"`
}

// Submit 学生提交作业（复用评分逻辑，允许多次提交取最高分）
func (s *AssignmentService) Submit(assignmentID, userID int64, readingsRaw json.RawMessage) (*AssignmentSubmitResult, error) {
	a, err := s.asgRepo.FindByID(assignmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// 校验是否是班级成员
	memberIDs, err := s.classRepo.MemberUserIDs(a.ClassID)
	if err != nil {
		return nil, err
	}
	isMember := false
	for _, id := range memberIDs {
		if id == userID {
			isMember = true
			break
		}
	}
	if !isMember {
		return nil, ErrForbidden
	}

	// 获取关卡 -> 实验 -> 评分目标
	lv, err := s.levelRepo.FindByID(a.LevelID)
	if err != nil {
		return nil, ErrNotFound
	}
	exp, err := s.expRepo.FindByID(lv.ExperimentID)
	if err != nil {
		return nil, ErrNotFound
	}

	// 评分（复用 scoring.go）
	score, passed, detail, err := scoreByExperiment(exp.Code, readingsRaw, exp.Target)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	// 写 UserProgress（与 /progress/submit 一致的逻辑）
	prog, err := s.progressRepo.Ensure(userID, a.LevelID)
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

	// 写/更新 AssignmentSubmission（允许多次提交，保留最高分）
	sub, err := s.asgRepo.FindSubmission(assignmentID, userID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if sub == nil {
		sub = &model.AssignmentSubmission{
			AssignmentID: assignmentID,
			UserID:       userID,
			Score:        score,
			BestScore:    score,
			Passed:       passed,
			Attempts:     1,
			SubmittedAt:  &now,
		}
	} else {
		sub.Attempts++
		sub.Score = score
		if score > sub.BestScore {
			sub.BestScore = score
		}
		if passed {
			sub.Passed = true
		}
		sub.SubmittedAt = &now
	}
	if err := s.asgRepo.UpsertSubmission(sub); err != nil {
		return nil, err
	}

	// 审计日志
	detailFull := fmt.Sprintf("assignment=%d %s | readings=%s", assignmentID, detail, string(readingsRaw))
	logEntry := &model.OperationLog{
		UserID: userID, Action: "assignment_submit", LevelID: &a.LevelID, Score: &score, Detail: detailFull,
	}
	_ = s.logRepo.Create(logEntry)

	return &AssignmentSubmitResult{
		Score:     score,
		Passed:    passed,
		BestScore: sub.BestScore,
	}, nil
}

// ListSubmissions 教师查看某作业的提交明细（含未交学生）
func (s *AssignmentService) ListSubmissions(assignmentID, requesterID int64, role string) ([]model.AssignmentSubmissionView, error) {
	a, err := s.asgRepo.FindByID(assignmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	// 权限：本班教师 / admin
	if role != "admin" {
		c, err := s.classRepo.FindByID(a.ClassID)
		if err != nil {
			return nil, ErrNotFound
		}
		if c.TeacherID != requesterID {
			return nil, ErrForbidden
		}
	}

	submitted, err := s.asgRepo.ListSubmissions(assignmentID)
	if err != nil {
		return nil, err
	}
	subMap := make(map[int64]*model.AssignmentSubmissionView)
	for i := range submitted {
		subMap[submitted[i].UserID] = &submitted[i]
	}

	// 补全未交学生
	allMemberIDs, err := s.asgRepo.ClassMemberIDsForAssignment(assignmentID)
	if err != nil {
		return nil, err
	}
	for _, uid := range allMemberIDs {
		if _, ok := subMap[uid]; ok {
			continue
		}
		var name, studentNo string
		if u, err := s.userRepo.FindByID(uid); err == nil {
			if u.Name != nil {
				name = *u.Name
			}
			if u.StudentNo != nil {
				studentNo = *u.StudentNo
			}
		}
		submitted = append(submitted, model.AssignmentSubmissionView{
			UserID:    uid,
			Name:      name,
			StudentNo: studentNo,
			BestScore: 0,
			Attempts:  0,
			Passed:    false,
		})
	}

	return submitted, nil
}

// Delete 删除作业（本班教师/admin）
func (s *AssignmentService) Delete(assignmentID, requesterID int64, role string) error {
	a, err := s.asgRepo.FindByID(assignmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		return err
	}
	if role != "admin" {
		c, err := s.classRepo.FindByID(a.ClassID)
		if err != nil {
			return ErrNotFound
		}
		if c.TeacherID != requesterID {
			return ErrForbidden
		}
	}
	return s.asgRepo.Delete(assignmentID)
}
