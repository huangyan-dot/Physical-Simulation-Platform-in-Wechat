package controller

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"physics-lab/backend/internal/middleware"
	"physics-lab/backend/internal/service"
)

type ProgressController struct {
	svc *service.ProgressService
}

func NewProgressController(svc *service.ProgressService) *ProgressController {
	return &ProgressController{svc: svc}
}

// Mine GET /progress/mine
func (ctl *ProgressController) Mine(c *gin.Context) {
	view, err := ctl.svc.Mine(middleware.CurrentUserID(c), middleware.CurrentRole(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载进度失败"})
		return
	}
	c.JSON(http.StatusOK, view)
}

// Class GET /progress/class/:classId
func (ctl *ProgressController) Class(c *gin.Context) {
	classID, err := strconv.ParseInt(c.Param("classId"), 10, 64)
	if err != nil || classID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid class id"})
		return
	}
	view, err := ctl.svc.ClassProgress(classID, middleware.CurrentUserID(c), middleware.CurrentRole(c))
	if err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载班级成绩失败"})
		}
		return
	}
	c.JSON(http.StatusOK, view)
}

// Submit POST /progress/submit（角色 student，路由层 RequireRole）
func (ctl *ProgressController) Submit(c *gin.Context) {
	var req struct {
		LevelID    int64           `json:"level_id" binding:"required"`
		Experiment string          `json:"experiment" binding:"required"`
		Readings   json.RawMessage `json:"readings"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误：level_id/experiment 必填"})
		return
	}
	res, err := ctl.svc.Submit(middleware.CurrentUserID(c), req.LevelID, req.Experiment, req.Readings)
	if err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "提交失败"})
		}
		return
	}
	c.JSON(http.StatusOK, res)
}
