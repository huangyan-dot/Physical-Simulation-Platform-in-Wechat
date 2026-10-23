package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"physics-lab/backend/internal/middleware"
	"physics-lab/backend/internal/service"
)

// AdminController 审计接口（仅 admin）。
type AdminController struct {
	svc *service.AdminService
}

func NewAdminController(svc *service.AdminService) *AdminController {
	return &AdminController{svc: svc}
}

// ListLogs GET /admin/operation-logs?user_id=&action=&level_id=&page=&size=
// 分页查询操作审计日志。仅 admin 角色可访问（路由层 RequireRole 守卫）。
func (ctl *AdminController) ListLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	userID := service.Atoi(c.Query("user_id"))
	levelID := service.Atoi(c.Query("level_id"))
	action := c.Query("action")

	view, err := ctl.svc.ListLogs(userID, levelID, action, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载审计日志失败"})
		return
	}
	c.JSON(http.StatusOK, view)
}

// MyLogs GET /audit/mine?page=&size=&action=（契约 §18）
// 任意登录用户查自己的审计日志；user_id 强制为本人，防止越权看他人。
func (ctl *AdminController) MyLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	action := c.Query("action")

	view, err := ctl.svc.ListLogs(middleware.CurrentUserID(c), 0, action, page, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载审计日志失败"})
		return
	}
	c.JSON(http.StatusOK, view)
}
