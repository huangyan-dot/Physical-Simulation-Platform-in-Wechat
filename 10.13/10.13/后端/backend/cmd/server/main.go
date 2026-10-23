package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"physics-lab/backend/internal/config"
	"physics-lab/backend/internal/controller"
	"physics-lab/backend/internal/middleware"
	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/pkg/jwt"
	"physics-lab/backend/internal/pkg/wechat"
	"physics-lab/backend/internal/repository"
	"physics-lab/backend/internal/router"
	"physics-lab/backend/internal/service"
)

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// 1. 加载配置（可用 CONFIG_PATH 环境变量覆盖）
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		wd, _ := os.Getwd()
		cfgPath = filepath.Join(wd, "configs", "config.yaml")
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// 2. 连接 MySQL。失败不致命：仅 /ping 可用，业务接口全部不注册。
	db, err := gorm.Open(mysql.Open(cfg.MySQLDSN()), &gorm.Config{})
	if err != nil {
		logger.Warn("MySQL 连接失败，仅 /ping 可用", zap.Error(err))
		runMinimal(cfg, logger)
		return
	}
	logger.Info("MySQL 已连接")

	// 3. AutoMigrate 全部 7 张表
	if err := db.AutoMigrate(
		&model.User{}, &model.Class{}, &model.ClassMember{},
		&model.Experiment{}, &model.Level{},
		&model.UserProgress{}, &model.OperationLog{},
	); err != nil {
		logger.Warn("AutoMigrate 失败", zap.Error(err))
	}

	// 4. 种子数据
	seedAll(db, logger)

	// 5. 装配依赖：repository -> service -> controller
	jm := jwt.New(cfg.JWT.Secret, cfg.JWT.ExpireHours)
	wx := wechat.New(cfg.WeChat.AppID, cfg.WeChat.Secret)

	userRepo := repository.NewUserRepository(db)
	classRepo := repository.NewClassRepository(db)
	levelRepo := repository.NewLevelRepository(db)
	expRepo := repository.NewExperimentRepository(db)
	progRepo := repository.NewProgressRepository(db)
	logRepo := repository.NewOperationLogRepository(db)

	authSvc := service.NewAuthService(userRepo, classRepo, logRepo, jm, wx, cfg.AllowDevBackdoor)
	classSvc := service.NewClassService(classRepo, logRepo)
	levelSvc := service.NewLevelService(levelRepo, expRepo, progRepo)
	progSvc := service.NewProgressService(progRepo, levelRepo, expRepo, logRepo, classRepo, userRepo)
	userSvc := service.NewUserService(userRepo)
	adminSvc := service.NewAdminService(logRepo)

	// 限流器：mode=debug 下若未配置则给个保守默认，防误关
	loginPerMin := cfg.RateLimit.LoginPerMinute
	if loginPerMin == 0 {
		loginPerMin = 30
	}
	submitPerMin := cfg.RateLimit.SubmitPerMinute
	if submitPerMin == 0 {
		submitPerMin = 20
	}

	deps := router.Deps{
		Mode:             cfg.Server.Mode,
		JM:               jm,
		UserCtl:          controller.NewUserController(userSvc),
		AuthCtl:          controller.NewAuthController(authSvc),
		ClassCtl:         controller.NewClassController(classSvc),
		LevelCtl:         controller.NewLevelController(levelSvc),
		ProgressCtl:      controller.NewProgressController(progSvc),
		AdminCtl:         controller.NewAdminController(adminSvc),
		LoginLimiter:     middleware.NewRateLimiter(loginPerMin, time.Minute),
		SubmitLimiter:    middleware.NewRateLimiter(submitPerMin, time.Minute),
		AllowDevBackdoor: cfg.AllowDevBackdoor,
		CORSAllowAll:     cfg.Server.Mode != "release", // debug/test 放行所有 Origin，上线收紧
		CORSAllowOrigins: cfg.CORS.AllowOrigins,
	}

	// 6. 起服务
	r := router.New(deps)
	addr := ":" + strconv.Itoa(cfg.Server.Port)
	logger.Info("server listening", zap.String("addr", addr))
	if err := r.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// runMinimal DB 不可用时的降级：只注册 /ping，业务接口全部不可用。
func runMinimal(cfg *config.Config, logger *zap.Logger) {
	r := router.New(router.Deps{
		Mode:         cfg.Server.Mode,
		CORSAllowAll: cfg.Server.Mode != "release",
		CORSAllowOrigins: cfg.CORS.AllowOrigins,
	})
	addr := ":" + strconv.Itoa(cfg.Server.Port)
	logger.Info("server listening (minimal)", zap.String("addr", addr))
	if err := r.Run(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
