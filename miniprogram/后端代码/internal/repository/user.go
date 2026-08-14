package repository

import (
	"physics-lab/backend/internal/model"

	"gorm.io/gorm"
)

// UserRepository 数据访问层：只跟 GORM/MySQL 打交道
type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindByID(id int64) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByOpenID 按微信 openid 查用户（登录用）
func (r *UserRepository) FindByOpenID(openid string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("openid = ?", openid).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Create 新建用户
func (r *UserRepository) Create(u *model.User) error {
	return r.db.Create(u).Error
}

// UpdateProfileFields 补全学籍信息（name / student_no）
func (r *UserRepository) UpdateProfileFields(id int64, name, studentNo string) (*model.User, error) {
	if err := r.db.Model(&model.User{}).Where("id = ?", id).
		Updates(map[string]interface{}{"name": name, "student_no": studentNo}).Error; err != nil {
		return nil, err
	}
	return r.FindByID(id)
}
