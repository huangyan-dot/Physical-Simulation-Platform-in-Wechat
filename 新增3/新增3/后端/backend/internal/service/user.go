package service

import (
	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/repository"
)

// UserService 业务逻辑层
type UserService struct {
	repo *repository.UserRepository
}

func NewUserService(repo *repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) GetUser(id int64) (*model.User, error) {
	return s.repo.FindByID(id)
}
