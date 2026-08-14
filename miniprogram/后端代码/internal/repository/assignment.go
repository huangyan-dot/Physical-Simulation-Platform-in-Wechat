package repository

import (
	"time"

	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
)

type AssignmentRepository struct {
	db *gorm.DB
}

func NewAssignmentRepository(db *gorm.DB) *AssignmentRepository {
	return &AssignmentRepository{db: db}
}

// Create 发布作业
func (r *AssignmentRepository) Create(a *model.Assignment) error {
	return r.db.Create(a).Error
}

// FindByID 查单个作业
func (r *AssignmentRepository) FindByID(id int64) (*model.Assignment, error) {
	var a model.Assignment
	if err := r.db.First(&a, id).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

// ListByClass 班级作业列表（带关卡名 + 实验码）
func (r *AssignmentRepository) ListByClass(classID int64) ([]model.AssignmentView, error) {
	var rows []model.AssignmentView
	err := r.db.Table("assignments a").
		Select("a.id, a.class_id, a.level_id, a.title, a.data_weight, a.deadline, a.created_at, "+
			"l.name AS level_name, e.code AS experiment_code").
		Joins("LEFT JOIN levels l ON l.id = a.level_id").
		Joins("LEFT JOIN experiments e ON e.id = l.experiment_id").
		Where("a.class_id = ?", classID).
		Order("a.deadline ASC").
		Scan(&rows).Error
	return rows, err
}

// ListByUser 学生视角：自己所有班级的作业（带提交状态）
func (r *AssignmentRepository) ListByUser(userID int64) ([]model.AssignmentView, error) {
	var rows []model.AssignmentView
	err := r.db.Table("assignments a").
		Select("a.id, a.class_id, a.level_id, a.title, a.data_weight, a.deadline, a.created_at, "+
			"l.name AS level_name, e.code AS experiment_code, "+
			"COALESCE(s.best_score, 0) AS best_score, COALESCE(s.attempts, 0) AS attempts, "+
			"COALESCE(s.best_data_score, 0) AS best_data_score, "+
			"COALESCE(s.best_quiz_score, 0) AS best_quiz_score, "+
			"COALESCE(s.quiz_done, FALSE) AS quiz_done, "+
			"CASE WHEN s.id IS NOT NULL THEN TRUE ELSE FALSE END AS submitted, "+
			"CASE WHEN a.deadline < NOW() THEN TRUE ELSE FALSE END AS overdue").
		Joins("LEFT JOIN levels l ON l.id = a.level_id").
		Joins("LEFT JOIN experiments e ON e.id = l.experiment_id").
		Joins("LEFT JOIN class_members cm ON cm.class_id = a.class_id AND cm.user_id = ?", userID).
		Joins("LEFT JOIN assignment_submissions s ON s.assignment_id = a.id AND s.user_id = ?", userID).
		Where("cm.user_id = ?", userID).
		Order("a.deadline ASC").
		Scan(&rows).Error
	return rows, err
}

// UpdateDataWeight 教师调节综合得分中测量数据所占比例
func (r *AssignmentRepository) UpdateDataWeight(id int64, weight int) error {
	return r.db.Model(&model.Assignment{}).Where("id = ?", id).
		Update("data_weight", weight).Error
}

// FindSubmission 查某学生某作业的提交记录
func (r *AssignmentRepository) FindSubmission(assignmentID, userID int64) (*model.AssignmentSubmission, error) {
	var s model.AssignmentSubmission
	err := r.db.Where("assignment_id = ? AND user_id = ?", assignmentID, userID).First(&s).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// UpsertSubmission 插入或更新提交记录（允许多次提交，保留最高分）
func (r *AssignmentRepository) UpsertSubmission(s *model.AssignmentSubmission) error {
	return r.db.Save(s).Error // Save：有主键更新，无主键插入
}

// ListSubmissions 教师查看某作业的全班提交明细
func (r *AssignmentRepository) ListSubmissions(assignmentID int64) ([]model.AssignmentSubmissionView, error) {
	var rows []model.AssignmentSubmissionView
	err := r.db.Table("assignment_submissions s").
		Select("s.user_id, u.name, u.student_no, "+
			"s.best_data_score AS data_score, s.best_quiz_score AS quiz_score, "+
			"s.quiz_done, s.region_label, s.best_score, "+
			"s.attempts, s.passed, s.submitted_at").
		Joins("LEFT JOIN users u ON u.id = s.user_id").
		Where("s.assignment_id = ?", assignmentID).
		Order("s.best_score DESC").
		Scan(&rows).Error
	return rows, err
}

// ClassMemberIDsForAssignment 取作业所属班级的全部成员 ID（教师查提交明细时补未交者）
func (r *AssignmentRepository) ClassMemberIDsForAssignment(assignmentID int64) ([]int64, error) {
	var ids []int64
	err := r.db.Table("class_members cm").
		Joins("JOIN assignments a ON a.class_id = cm.class_id").
		Where("a.id = ?", assignmentID).
		Pluck("cm.user_id", &ids).Error
	return ids, err
}

// Delete 删除作业
func (r *AssignmentRepository) Delete(id int64) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("assignment_id = ?", id).Delete(&model.AssignmentSubmission{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Assignment{}, id).Error
	})
}

// Now 返回当前时间（方便测试 mock）
var now = func() time.Time { return time.Now() }
