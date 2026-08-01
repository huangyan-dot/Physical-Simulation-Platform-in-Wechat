package service

import (
	"errors"

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

// Create 教师/管理员建班
func (s *ClassService) Create(name string, teacherID int64) (*model.Class, error) {
	c := &model.Class{Name: name, TeacherID: teacherID}
	if err := s.repo.Create(c); err != nil {
		return nil, err
	}
	return c, nil
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
