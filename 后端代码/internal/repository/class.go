package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
)

type ClassRepository struct {
	db *gorm.DB
}

func NewClassRepository(db *gorm.DB) *ClassRepository {
	return &ClassRepository{db: db}
}

// ListByUser 按用户视角返回班级视图行（带教师名 + 成员数）
//   - student：返回该生加入的班级
//   - teacher：返回该师带的班级
//   - admin   ：返回全部
func (r *ClassRepository) ListByUser(userID int64, role string) ([]model.ClassView, error) {
	q := r.db.Table("classes c").
		Select("c.id, c.name, c.teacher_id, u.name AS teacher_name, COUNT(cm.id) AS member_count").
		Joins("LEFT JOIN users u ON u.id = c.teacher_id").
		Joins("LEFT JOIN class_members cm ON cm.class_id = c.id").
		Group("c.id")

	switch role {
	case "teacher":
		q = q.Where("c.teacher_id = ?", userID)
	case "admin":
		// 全部
	default: // student
		q = q.Where("c.id IN (SELECT class_id FROM class_members WHERE user_id = ?)", userID)
	}

	var rows []model.ClassView
	if err := q.Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *ClassRepository) FindByID(id int64) (*model.Class, error) {
	var c model.Class
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ClassRepository) Create(c *model.Class) error {
	return r.db.Create(c).Error
}

func (r *ClassRepository) Delete(id int64) error {
	// 级联清理成员关系
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("class_id = ?", id).Delete(&model.ClassMember{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.Class{}, id).Error
	})
}

// IsMember 是否已是成员
func (r *ClassRepository) IsMember(classID, userID int64) (bool, error) {
	var cnt int64
	err := r.db.Model(&model.ClassMember{}).
		Where("class_id = ? AND user_id = ?", classID, userID).Count(&cnt).Error
	return cnt > 0, err
}

// AddMember 加入班级，重复加入返回 ErrAlreadyMember
var ErrAlreadyMember = errors.New("already a member")

func (r *ClassRepository) AddMember(classID, userID int64) error {
	ok, err := r.IsMember(classID, userID)
	if err != nil {
		return err
	}
	if ok {
		return ErrAlreadyMember
	}
	return r.db.Create(&model.ClassMember{ClassID: classID, UserID: userID, JoinedAt: time.Now()}).Error
}

// MemberUserIDs 班级全部成员的 user_id
func (r *ClassRepository) MemberUserIDs(classID int64) ([]int64, error) {
	var ids []int64
	err := r.db.Model(&model.ClassMember{}).
		Where("class_id = ?", classID).Pluck("user_id", &ids).Error
	return ids, err
}
