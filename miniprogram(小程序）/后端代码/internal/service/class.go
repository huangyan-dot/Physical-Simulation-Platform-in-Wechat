package service

import (
	"errors"
	"math/rand"

	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/repository"
)

type ClassService struct {
	repo *repository.ClassRepository
}

func NewClassService(repo *repository.ClassRepository) *ClassService {
	return &ClassService{repo: repo}
}

// ListForUser 按角色返回班级视图
func (s *ClassService) ListForUser(userID int64, role string) ([]model.ClassView, error) {
	return s.repo.ListByUser(userID, role)
}

// Create 教师/管理员建班，自动生成 6 位班级码
func (s *ClassService) Create(name string, teacherID int64) (*model.Class, error) {
	c := &model.Class{Name: name, TeacherID: teacherID, Code: genClassCode()}
	if err := s.repo.Create(c); err != nil {
		return nil, err
	}
	return c, nil
}

// JoinByCode 学生凭班级码自助加入
func (s *ClassService) JoinByCode(code string, userID int64) (*model.Class, error) {
	c, err := s.repo.FindByCode(code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if err := s.repo.AddMember(c.ID, userID); err != nil {
		if errors.Is(err, repository.ErrAlreadyMember) {
			return nil, ErrConflict
		}
		return nil, err
	}
	return c, nil
}

// ListMembers 教师查本班成员名单
func (s *ClassService) ListMembers(classID, requesterID int64, role string) ([]repository.MemberView, error) {
	if role != "admin" {
		c, err := s.repo.FindByID(classID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		if c.TeacherID != requesterID {
			return nil, ErrForbidden
		}
	}
	return s.repo.ListMembers(classID)
}

// genClassCode 生成 6 位大写字母+数字班级码
func genClassCode() string {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // 去掉易混 I/O/0/1
	b := make([]byte, 6)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return string(b)
}

// Delete 仅本班教师或管理员可删
func (s *ClassService) Delete(classID, requesterID int64, role string) error {
	if role != "admin" {
		c, err := s.repo.FindByID(classID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if c.TeacherID != requesterID {
			return ErrForbidden
		}
	}
	return s.repo.Delete(classID)
}

// AddMember 仅本班教师或管理员可加成员；重复加入返回 ErrConflict
func (s *ClassService) AddMember(classID, userID, requesterID int64, role string) error {
	if role != "admin" {
		c, err := s.repo.FindByID(classID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		if c.TeacherID != requesterID {
			return ErrForbidden
		}
	}
	if err := s.repo.AddMember(classID, userID); err != nil {
		if errors.Is(err, repository.ErrAlreadyMember) {
			return ErrConflict
		}
		return err
	}
	return nil
}
