package main

import (
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"physics-lab/backend/internal/model"
)

// seedAll 启动时幂等写入开发期种子数据，让小程序开箱即用：
//   - 3 个 dev 测试账号（与 dev_ 后门对齐，登录即已补全学籍）
//   - 6 个实验（config + target，含 pass_score）
//   - 6 个关卡（串成解锁链 L1->L2->L3->L4->L5->L6）
//   - 1 个示范班级（dev 教师建班 + dev 学生入班，便于演示 /classes 与班级成绩）
//
// 线上部署应改用 migrations/*.sql 自管，并移除 dev_ 后门。
func seedAll(db *gorm.DB, logger *zap.Logger) {
	seedUsers(db, logger)
	seedExperiments(db, logger)
	seedLevels(db, logger)
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

// seedExperiments 幂等写入 6 个实验定义。
// target 必须含 pass_score：scoring.go 用 score>=pass_score 判过关，缺失则恒过关。
// oscilloscope 的 channels 键用大写 CH1/CH2，与前端 oscilloscope.js 提交的 channel 大小写一致。
func seedExperiments(db *gorm.DB, logger *zap.Logger) {
	type expSeed struct {
		code, name, renderMode string
		config, target         string
	}
	seeds := []expSeed{
		{
			code: "newton_ring", name: "牛顿环实验", renderMode: "mixed_3d_2d",
			config: `{"wavelength_nm":589.3,"lens_radius_mm":855,"k_range":[1,10],"tolerance_mm":0.02}`,
			target: `{"method":"least_squares_R","lens_radius_mm":855,"wavelength_nm":589.3,"pass_score":60}`,
		},
		{
			code: "oscilloscope", name: "示波器实验", renderMode: "mixed_3d_2d",
			config: `{"channels":{"CH1":{"A":2.0,"f":50,"phi":0},"CH2":{"A":1.5,"f":50,"phi":0.7854}}}`,
			target: `{"method":"param_match","channels":{"CH1":{"A":2.0,"f":50},"CH2":{"A":1.5,"f":50}},"pass_score":60}`,
		},
		{
			code: "pendulum", name: "单摆实验", renderMode: "mixed_3d_2d",
			config: `{"length_m":1.0,"angle_deg":15,"gravity":9.8}`,
			target: `{"method":"gravity_fit","gravity":9.8,"length_m":1.0,"pass_score":60}`,
		},
		{
			// 杨氏模量：钢丝 E≈2.0e11 Pa，砝码 0.5kg(4.905N)，丝长 1.0m，
			// 直径 0.5mm，光杠杆臂长 70mm，镜尺距离 1.5m
			code: "young_modulus", name: "杨氏模量实验", renderMode: "mixed_3d_2d",
			config: `{"force_n":4.905,"length_m":1.0,"diameter_m":0.0005,"lever_arm_m":0.070,"mirror_dist_m":1.5}`,
			target: `{"method":"young_fit","young_modulus_pa":2.0e11,"force_n":4.905,"length_m":1.0,"diameter_m":0.0005,"lever_arm_m":0.070,"mirror_dist_m":1.5,"pass_score":60}`,
		},
		{
			// 霍尔效应：B=0.3T，样品厚度 0.5mm，载流子浓度 ~1e21 m^-3
			code: "hall_effect", name: "霍尔效应实验", renderMode: "mixed_3d_2d",
			config: `{"b_field_t":0.3,"thickness_m":0.0005}`,
			target: `{"method":"hall_fit","b_field_t":0.3,"thickness_m":0.0005,"carrier_conc":1.0e21,"pass_score":60}`,
		},
		{
			// 迈克尔逊干涉仪：He-Ne 激光 632.8nm
			code: "michelson", name: "迈克尔逊干涉仪实验", renderMode: "mixed_3d_2d",
			config: `{"wavelength_nm":632.8}`,
			target: `{"method":"wavelength_fit","wavelength_nm":632.8,"pass_score":60}`,
		},
	}
	for _, s := range seeds {
		var e model.Experiment
		err := db.Where("code = ?", s.code).First(&e).Error
		if err == gorm.ErrRecordNotFound {
			e = model.Experiment{
				Code:       s.code,
				Name:       s.name,
				RenderMode: s.renderMode,
				Config:     model.JSON([]byte(s.config)),
				Target:     model.JSON([]byte(s.target)),
			}
			if err := db.Create(&e).Error; err != nil {
				logger.Warn("seed 实验失败", zap.String("code", s.code), zap.Error(err))
			}
		} else if err != nil {
			logger.Warn("seed 实验查询失败", zap.String("code", s.code), zap.Error(err))
		}
	}
}

// seedLevels 幂等写入 6 个关卡并串成解锁链。
// 按 order_no 逐个检查：已有则跳过，不存在则补建（保留 prereq 链完整性）。
func seedLevels(db *gorm.DB, logger *zap.Logger) {
	type lvSeed struct {
		expCode, name string
		orderNo       int
		difficulty    int
	}
	seeds := []lvSeed{
		{"newton_ring", "牛顿环实验", 1, 2},
		{"oscilloscope", "示波器实验", 2, 3},
		{"pendulum", "单摆实验", 3, 1},
		{"young_modulus", "杨氏模量实验", 4, 3},
		{"hall_effect", "霍尔效应实验", 5, 4},
		{"michelson", "迈克尔逊干涉仪实验", 6, 4},
	}

	for _, s := range seeds {
		// 按 order_no 检查是否已存在
		var existing model.Level
		if err := db.Where("order_no = ?", s.orderNo).First(&existing).Error; err == nil {
			continue // 已存在，跳过
		} else if err != gorm.ErrRecordNotFound {
			logger.Warn("seed 关卡查询失败", zap.String("name", s.name), zap.Error(err))
			continue
		}

		var exp model.Experiment
		if err := db.Where("code = ?", s.expCode).First(&exp).Error; err != nil {
			logger.Warn("seed 关卡：找不到实验", zap.String("code", s.expCode), zap.Error(err))
			continue
		}

		// 找前置关卡（order_no - 1）
		var prevID *int64
		if s.orderNo > 1 {
			var prev model.Level
			if err := db.Where("order_no = ?", s.orderNo-1).First(&prev).Error; err == nil {
				id := prev.ID
				prevID = &id
			}
		}

		lv := model.Level{
			ExperimentID:  exp.ID,
			Name:           s.name,
			OrderNo:        s.orderNo,
			Difficulty:     s.difficulty,
			PrereqLevelID: prevID,
		}
		if err := db.Create(&lv).Error; err != nil {
			logger.Warn("seed 关卡失败", zap.String("name", s.name), zap.Error(err))
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
