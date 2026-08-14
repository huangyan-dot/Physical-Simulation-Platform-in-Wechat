package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
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

// Create 发布作业（教师）。dataWeight<=0 时用默认 60。
func (s *AssignmentService) Create(classID, levelID int64, title string, deadline time.Time, dataWeight int, createdBy int64) (*model.Assignment, error) {
	// 校验关卡存在
	if _, err := s.levelRepo.FindByID(levelID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if dataWeight <= 0 {
		dataWeight = model.DefaultDataWeight
	}
	if dataWeight > 100 {
		dataWeight = 100
	}
	a := &model.Assignment{
		ClassID:    classID,
		LevelID:    levelID,
		Title:      title,
		DataWeight: dataWeight,
		Deadline:   deadline,
		CreatedBy:  createdBy,
	}
	if err := s.asgRepo.Create(a); err != nil {
		return nil, err
	}
	return a, nil
}

// SetDataWeight 教师调节综合得分比例（测量数据占 weight%，自测占 100-weight%）。
// 下限取 5 而非 0：0 在库里与「未设置」无法区分，会被 effectiveDataWeight 当成默认 60。
func (s *AssignmentService) SetDataWeight(assignmentID, requesterID int64, role string, weight int) error {
	if weight < 5 || weight > 100 {
		return fmt.Errorf("%w: 比例需在 5~100 之间", ErrBadRequest)
	}
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
	return s.asgRepo.UpdateDataWeight(assignmentID, weight)
}

// effectiveDataWeight 归一化作业上的数据权重。
// 老数据（data_weight 列加上之前建的行）读出来是 0，那是「未设置」而不是
// 「数据占 0%」，必须回落到默认 60，否则综合分会变成只看自测。
// 教师若真想让自测占满，SetDataWeight 允许显式存 0，此处无法区分，
// 因此 SetDataWeight 把 0 也当未设置处理，最小可设值为 5。
func effectiveDataWeight(w int) int {
	if w <= 0 {
		return model.DefaultDataWeight
	}
	if w > 100 {
		return 100
	}
	return w
}

// comboScore 按权重合成综合得分：数据分×w% + 自测分×(100-w)%
func comboScore(dataScore, quizScore, dataWeight int) int {
	w := effectiveDataWeight(dataWeight)
	total := float64(dataScore)*float64(w)/100 +
		float64(quizScore)*float64(100-w)/100
	return int(math.Round(total))
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
	// 本次提交
	DataScore int `json:"data_score"`
	QuizScore int `json:"quiz_score"`
	Score     int `json:"score"` // 综合得分
	// 历史最好
	BestDataScore int  `json:"best_data_score"`
	BestQuizScore int  `json:"best_quiz_score"`
	BestScore     int  `json:"best_score"`
	Passed        bool `json:"passed"`
	// 权重回显，前端据此展示「数据 60% + 自测 40%」
	DataWeight int  `json:"data_weight"`
	QuizDone   bool `json:"quiz_done"`
}

// AssignmentSubmitInput 学生一次提交的内容。
//
// 两段式提交：
//  1. 做完测量 -> Readings 非空，QuizScore 为 nil（自测分沿用历史值，通常是 0）
//  2. 做完自测 -> QuizScore 非 nil，Readings 为空（数据分沿用历史最好值）
//
// 也允许一次带上两者。任意一段都会重算综合分。
type AssignmentSubmitInput struct {
	Readings    json.RawMessage
	QuizScore   *int
	RegionLabel string
}

// Submit 学生提交作业（允许多次提交，各项分数分别取历史最高）
func (s *AssignmentService) Submit(assignmentID, userID int64, in AssignmentSubmitInput) (*AssignmentSubmitResult, error) {
	a, err := s.asgRepo.FindByID(assignmentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if len(in.Readings) == 0 && in.QuizScore == nil {
		return nil, fmt.Errorf("%w: 需要提交测量数据或自测成绩", ErrBadRequest)
	}
	if in.QuizScore != nil && (*in.QuizScore < 0 || *in.QuizScore > 100) {
		return nil, fmt.Errorf("%w: 自测得分需在 0~100 之间", ErrBadRequest)
	}

	// 截止后不再接收提交
	if !a.Deadline.IsZero() && time.Now().After(a.Deadline) {
		return nil, fmt.Errorf("%w: 作业已过截止时间", ErrBadRequest)
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

	// 读已有提交记录，未提交过的部分沿用历史值
	sub, err := s.asgRepo.FindSubmission(assignmentID, userID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		sub = &model.AssignmentSubmission{
			AssignmentID: assignmentID,
			UserID:       userID,
		}
	}

	// ---- 测量数据评分 ----
	dataScore := sub.BestDataScore // 本次没交数据时，综合分沿用历史最好数据分
	dataPassed := sub.Passed
	detail := "仅提交自测"
	if len(in.Readings) > 0 {
		lv, err := s.levelRepo.FindByID(a.LevelID)
		if err != nil {
			return nil, ErrNotFound
		}
		exp, err := s.expRepo.FindByID(lv.ExperimentID)
		if err != nil {
			return nil, ErrNotFound
		}
		sc, passed, d, err := scoreByExperiment(exp.Code, in.Readings, exp.Target)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
		}
		dataScore, dataPassed, detail = sc, passed, d

		// 同步 UserProgress（与 /progress/submit 一致：进度只反映测量数据）
		prog, err := s.progressRepo.Ensure(userID, a.LevelID)
		if err != nil {
			return nil, err
		}
		prog.Attempts++
		prog.LastScore = sc
		if sc > prog.BestScore {
			prog.BestScore = sc
		}
		if passed {
			prog.Passed = true
		}
		if err := s.progressRepo.Upsert(prog); err != nil {
			return nil, err
		}

		sub.DataScore = sc
		if sc > sub.BestDataScore {
			sub.BestDataScore = sc
		}
		if in.RegionLabel != "" {
			sub.RegionLabel = in.RegionLabel
		}
	}

	// ---- 自测评分 ----
	quizScore := sub.BestQuizScore // 本次没交自测时沿用历史最好自测分
	if in.QuizScore != nil {
		quizScore = *in.QuizScore
		sub.QuizScore = quizScore
		sub.QuizDone = true
		if quizScore > sub.BestQuizScore {
			sub.BestQuizScore = quizScore
		}
	}

	// ---- 综合得分 ----
	combo := comboScore(dataScore, quizScore, a.DataWeight)
	sub.Score = combo
	if combo > sub.BestScore {
		sub.BestScore = combo
	}
	if dataPassed {
		sub.Passed = true
	}
	sub.Attempts++
	now := time.Now()
	sub.SubmittedAt = &now

	if err := s.asgRepo.UpsertSubmission(sub); err != nil {
		return nil, err
	}

	// 审计日志
	detailFull := fmt.Sprintf("assignment=%d %s | data=%d quiz=%d combo=%d(w=%d) | readings=%s",
		assignmentID, detail, dataScore, quizScore, combo, a.DataWeight, string(in.Readings))
	logEntry := &model.OperationLog{
		UserID: userID, Action: "assignment_submit", LevelID: &a.LevelID, Score: &combo, Detail: detailFull,
	}
	_ = s.logRepo.Create(logEntry)

	return &AssignmentSubmitResult{
		DataScore:     dataScore,
		QuizScore:     quizScore,
		Score:         combo,
		BestDataScore: sub.BestDataScore,
		BestQuizScore: sub.BestQuizScore,
		BestScore:     sub.BestScore,
		Passed:        sub.Passed,
		DataWeight:    effectiveDataWeight(a.DataWeight),
		QuizDone:      sub.QuizDone,
	}, nil
}

// AssignmentSubmissionsResult 教师查提交明细的响应（带当前权重）
type AssignmentSubmissionsResult struct {
	Title      string                           `json:"title"`
	DataWeight int                              `json:"data_weight"`
	QuizWeight int                              `json:"quiz_weight"`
	Rows       []model.AssignmentSubmissionView `json:"rows"`
}

// ListSubmissions 教师查看某作业的提交明细（含未交学生）。
// 综合分按作业「当前」权重实时算出，教师改权重后立刻生效，无需重算历史数据。
func (s *AssignmentService) ListSubmissions(assignmentID, requesterID int64, role string) (*AssignmentSubmissionsResult, error) {
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
	seen := make(map[int64]bool, len(submitted))
	for i := range submitted {
		seen[submitted[i].UserID] = true
		// 按当前权重重算综合分
		submitted[i].ComboScore = comboScore(
			submitted[i].DataScore, submitted[i].QuizScore, a.DataWeight)
	}

	// 补全未交学生
	allMemberIDs, err := s.asgRepo.ClassMemberIDsForAssignment(assignmentID)
	if err != nil {
		return nil, err
	}
	for _, uid := range allMemberIDs {
		if seen[uid] {
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
		})
	}

	// 综合分从高到低，未交的沉底
	sort.SliceStable(submitted, func(i, j int) bool {
		return submitted[i].ComboScore > submitted[j].ComboScore
	})

	return &AssignmentSubmissionsResult{
		Title:      a.Title,
		DataWeight: effectiveDataWeight(a.DataWeight),
		QuizWeight: 100 - effectiveDataWeight(a.DataWeight),
		Rows:       submitted,
	}, nil
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
