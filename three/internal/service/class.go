package service

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/repository"
)

const maxClassNameLen = 64 // DB VARCHAR(64)

type ClassService struct {
	repo    *repository.ClassRepository
	logRepo *repository.OperationLogRepository
}

func NewClassService(repo *repository.ClassRepository, logRepo *repository.OperationLogRepository) *ClassService {
	return &ClassService{repo: repo, logRepo: logRepo}
}

// validateClassName 校验班级名：去空格后非空、不超长
func validateClassName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrBadRequest
	}
	if len(name) > maxClassNameLen {
		return ErrBadRequest
	}
	return nil
}

// ListForUser 按角色返回班级视图
func (s *ClassService) ListForUser(userID int64, role string) ([]model.ClassView, error) {
	return s.repo.ListByUser(userID, role)
}

// Create 教师/管理员建班
func (s *ClassService) Create(name string, teacherID int64) (*model.Class, error) {
	if err := validateClassName(name); err != nil {
		return nil, err
	}
	c := &model.Class{Name: strings.TrimSpace(name), TeacherID: teacherID}
	if err := s.repo.Create(c); err != nil {
		return nil, err
	}
	s.audit(teacherID, "class.create", fmt.Sprintf("class_id=%d name=%s", c.ID, c.Name))
	return c, nil
}

// guardTeacherOrAdmin 本班教师或管理员才可操作；班级不存在返回 ErrNotFound
func (s *ClassService) guardTeacherOrAdmin(classID, requesterID int64, role string) (*model.Class, error) {
	c, err := s.repo.FindByID(classID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if role != "admin" && c.TeacherID != requesterID {
		return nil, ErrForbidden
	}
	return c, nil
}

// Delete 仅本班教师或管理员可删
func (s *ClassService) Delete(classID, requesterID int64, role string) error {
	c, err := s.guardTeacherOrAdmin(classID, requesterID, role)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(classID); err != nil {
		return err
	}
	s.audit(requesterID, "class.delete", fmt.Sprintf("class_id=%d name=%s", classID, c.Name))
	return nil
}

// AddMember 仅本班教师或管理员可加成员；重复加入返回 ErrConflict
func (s *ClassService) AddMember(classID, userID, requesterID int64, role string) error {
	if _, err := s.guardTeacherOrAdmin(classID, requesterID, role); err != nil {
		return err
	}
	if err := s.repo.AddMember(classID, userID); err != nil {
		if errors.Is(err, repository.ErrAlreadyMember) {
			return ErrConflict
		}
		return err
	}
	s.audit(requesterID, "class.member.add", fmt.Sprintf("class_id=%d user_id=%d", classID, userID))
	return nil
}

// Detail GET /classes/:id（契约 §14）：本班教师 / admin / 本班学生成员可看
func (s *ClassService) Detail(classID, requesterID int64, role string) (*model.ClassDetailView, error) {
	c, err := s.repo.FindByID(classID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	// 权限：admin / 本班教师直接放行；学生须是本班成员
	if role != "admin" && c.TeacherID != requesterID {
		ok, err := s.repo.IsMember(classID, requesterID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrForbidden
		}
	}
	members, err := s.repo.MembersWithUser(classID)
	if err != nil {
		return nil, err
	}
	if members == nil {
		members = []model.ClassMemberView{}
	}
	return &model.ClassDetailView{
		ID: c.ID, Name: c.Name, TeacherID: c.TeacherID,
		TeacherName: s.repo.TeacherName(c.TeacherID),
		Members:     members,
	}, nil
}

// Rename PUT /classes/:id（契约 §15）：仅本班教师或管理员
func (s *ClassService) Rename(classID int64, name string, requesterID int64, role string) (*model.Class, error) {
	if err := validateClassName(name); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	c, err := s.guardTeacherOrAdmin(classID, requesterID, role)
	if err != nil {
		return nil, err
	}
	if err := s.repo.UpdateName(classID, name); err != nil {
		return nil, err
	}
	c.Name = name
	s.audit(requesterID, "class.rename", fmt.Sprintf("class_id=%d name=%s", classID, name))
	return c, nil
}

// RemoveMember DELETE /classes/:id/members/:userId（契约 §16）：仅本班教师或管理员
func (s *ClassService) RemoveMember(classID, userID, requesterID int64, role string) error {
	if _, err := s.guardTeacherOrAdmin(classID, requesterID, role); err != nil {
		return err
	}
	if err := s.repo.RemoveMember(classID, userID); err != nil {
		if errors.Is(err, repository.ErrNotMember) {
			return ErrNotFound
		}
		return err
	}
	s.audit(requesterID, "class.member.remove", fmt.Sprintf("class_id=%d user_id=%d", classID, userID))
	return nil
}

// audit 写审计日志，失败不阻断业务（M6 验收要求"班级变更有日志"）。
func (s *ClassService) audit(userID int64, action, detail string) {
	if s.logRepo == nil {
		return
	}
	_ = s.logRepo.Create(&model.OperationLog{
		UserID: userID,
		Action: action,
		Detail: detail,
	})
}
