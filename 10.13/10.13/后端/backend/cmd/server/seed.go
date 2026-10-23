package main

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
)

// seedAll 启动时幂等写入开发期种子数据，让小程序开箱即用：
//   - 3 个 dev 测试账号（与 dev_ 后门对齐，登录即已补全学籍）
//   - 1 个示范班级（dev 教师建班 + dev 学生入班，便于演示 /classes 与班级成绩）
//
// 实验与关卡数据已按需求全部清空（2026-08-21）：
// 线上部署应改用 migrations/*.sql 自管，并移除 dev_ 后门。
func seedAll(db *gorm.DB, logger *zap.Logger) {
	seedUsers(db, logger)
	seedDemoClass(db, logger)
}

// seedUsers 幂等写入 dev 账号；已存在者只补缺字段，不覆盖用户自填的学籍。
func seedUsers(db *gorm.DB, logger *zap.Logger) {
	type seedUser struct {
		openid, role, name, studentNo string
	}
	seeds := []seedUser{
		{"oDEV_STUDENT", "student", "测试同学", "2023001"},
		{"oDEV_TEACHER", "teacher", "张老师", ""},
		{"oDEV_ADMIN", "admin", "管理员", ""},
	}
	for _, s := range seeds {
		var u model.User
		err := db.Where("openid = ?", s.openid).First(&u).Error
		if err == gorm.ErrRecordNotFound {
			name := s.name
			u = model.User{OpenID: s.openid, Role: s.role, Name: &name}
			if s.studentNo != "" {
				u.StudentNo = &s.studentNo
			}
			if err := db.Create(&u).Error; err != nil {
				logger.Warn("seed 用户失败", zap.String("openid", s.openid), zap.Error(err))
			}
			continue
		}
		if err != nil {
			logger.Warn("seed 用户查询失败", zap.String("openid", s.openid), zap.Error(err))
			continue
		}
		// 已存在：仅补缺，不覆盖
		updates := map[string]interface{}{}
		if u.Role == "" {
			updates["role"] = s.role
		}
		if u.Name == nil || *u.Name == "" {
			updates["name"] = s.name
		}
		if (u.StudentNo == nil || *u.StudentNo == "") && s.studentNo != "" {
			updates["student_no"] = s.studentNo
		}
		if len(updates) > 0 {
			if err := db.Model(&u).Updates(updates).Error; err != nil {
				logger.Warn("seed 用户补全失败", zap.String("openid", s.openid), zap.Error(err))
			}
		}
	}
}

// seedDemoClass 幂等创建示范班级并让 dev 学生加入，便于演示班级视角接口。
func seedDemoClass(db *gorm.DB, logger *zap.Logger) {
	var teacher, student model.User
	if err := db.Where("openid = ?", "oDEV_TEACHER").First(&teacher).Error; err != nil {
		logger.Warn("seed 班级：找不到 dev 教师", zap.Error(err))
		return
	}
	if err := db.Where("openid = ?", "oDEV_STUDENT").First(&student).Error; err != nil {
		logger.Warn("seed 班级：找不到 dev 学生", zap.Error(err))
		return
	}

	const className = "物理实验1班"
	var cls model.Class
	err := db.Where("teacher_id = ? AND name = ?", teacher.ID, className).First(&cls).Error
	if err == gorm.ErrRecordNotFound {
		cls = model.Class{Name: className, TeacherID: teacher.ID}
		if err := db.Create(&cls).Error; err != nil {
			logger.Warn("seed 班级创建失败", zap.Error(err))
			return
		}
	} else if err != nil {
		logger.Warn("seed 班级查询失败", zap.Error(err))
		return
	}

	// 学生入班（幂等）
	var cm model.ClassMember
	if err := db.Where("class_id = ? AND user_id = ?", cls.ID, student.ID).First(&cm).Error; err == gorm.ErrRecordNotFound {
		if err := db.Create(&model.ClassMember{ClassID: cls.ID, UserID: student.ID, JoinedAt: time.Now()}).Error; err != nil {
			logger.Warn("seed 班级成员失败", zap.Error(err))
		}
	}
}
