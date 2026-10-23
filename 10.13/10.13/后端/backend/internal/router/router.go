package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"physics-lab/backend/internal/controller"
	"physics-lab/backend/internal/middleware"
	"physics-lab/backend/internal/pkg/jwt"
)

// Deps 路由依赖。DB 不可用时只填 Mode+CORS：仅注册 /ping，业务接口全部不注册。
type Deps struct {
	Mode             string
	JM               *jwt.Manager
	UserCtl          *controller.UserController
	AuthCtl          *controller.AuthController
	ClassCtl         *controller.ClassController
	LevelCtl         *controller.LevelController
	ProgressCtl      *controller.ProgressController
	AdminCtl         *controller.AdminController
	LoginLimiter     *middleware.RateLimiter // /auth/login 限流（按 IP）
	SubmitLimiter    *middleware.RateLimiter // /progress/submit 限流（按 user_id）
	AllowDevBackdoor bool                   // false 时不注册 /login 别名
	CORSAllowAll     bool
	CORSAllowOrigins []string
}

// New 装配路由。按依赖是否就绪条件注册：缺依赖的分组直接跳过，不 panic。
func New(d Deps) *gin.Engine {
	if d.Mode != "" {
		gin.SetMode(d.Mode)
	}
	r := gin.Default()

	// 全局 CORS（无 Origin 时不加头，不影响小程序/真机请求）
	r.Use(middleware.CORS(d.CORSAllowAll, d.CORSAllowOrigins))

	// 统一 404：未注册路径返回契约错误格式而非 Gin 默认文本
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "资源不存在"})
	})

	// 始终可用：健康检查
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// 公开：登录（含 dev_ 开发后门）。自带鉴权判断，无需 token。
	if d.AuthCtl != nil {
		loginGroup := r.Group("", rateLimitIfSet(d.LoginLimiter, middleware.KeyByIP))
		loginGroup.POST("/auth/login", d.AuthCtl.Login)
		// 兼容别名（契约 §0.1-1）：仅开发后门开启时注册，前端改回 /auth/login 后连同后门一起移除
		if d.AllowDevBackdoor {
			loginGroup.POST("/login", d.AuthCtl.Login)
		}
	}

	// 阶段 0 烟雾接口：保留公开，DB 不可用（UserCtl 为 nil）时不注册
	if d.UserCtl != nil {
		r.GET("/users/:id", d.UserCtl.GetUser)
	}

	// 需鉴权：JWT 校验通过后把 uid/role 注入 context
	if d.JM != nil {
		authed := r.Group("", middleware.Auth(d.JM))

		if d.AuthCtl != nil {
			authed.GET("/auth/me", d.AuthCtl.Me)
			authed.PUT("/users/:id", d.AuthCtl.CompleteProfile) // 仅本人可改（controller 内校验）
		}
		if d.ClassCtl != nil {
			authed.GET("/classes", d.ClassCtl.List)
			authed.POST("/classes", middleware.RequireRole("teacher", "admin"), d.ClassCtl.Create)
			authed.GET("/classes/:id", d.ClassCtl.Detail)          // v0.4 §14：本班教师/admin/本班学生
			authed.PUT("/classes/:id", d.ClassCtl.Update)          // v0.4 §15：本班教师/admin（service 内校验）
			authed.DELETE("/classes/:id", d.ClassCtl.Delete)
			authed.POST("/classes/:id/members", d.ClassCtl.AddMember)
			authed.DELETE("/classes/:id/members/:userId", d.ClassCtl.RemoveMember) // v0.4 §16
		}
		if d.LevelCtl != nil {
			authed.GET("/levels", d.LevelCtl.List)
			authed.GET("/experiments", d.LevelCtl.ListExperiments)  // v0.4 §17：实验元数据列表
			authed.GET("/experiments/:id", d.LevelCtl.GetExperiment) // :id 是 level_id（契约 §9）
		}
		if d.ProgressCtl != nil {
			authed.GET("/progress/mine", d.ProgressCtl.Mine)
			authed.GET("/progress/class/:classId", d.ProgressCtl.Class)
			// submit 限流：先鉴权（注入 uid）再按 user_id 限流，故放 authed 组内
			authed.POST("/progress/submit",
				rateLimitIfSet(d.SubmitLimiter, middleware.KeyByUserID),
				middleware.RequireRole("student"),
				d.ProgressCtl.Submit,
			)
		}
		if d.AdminCtl != nil {
			authed.GET("/admin/operation-logs", middleware.RequireRole("admin"), d.AdminCtl.ListLogs)
			authed.GET("/audit/mine", d.AdminCtl.MyLogs) // v0.4 §18：任意登录用户看自己日志
		}
	}

	return r
}

// rateLimitIfSet 限流器为 nil 时返回一个空操作中间件（不限流）。
func rateLimitIfSet(rl *middleware.RateLimiter, keyFn func(*gin.Context) string) gin.HandlerFunc {
	if rl == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return middleware.RateLimit(rl, keyFn)
}
