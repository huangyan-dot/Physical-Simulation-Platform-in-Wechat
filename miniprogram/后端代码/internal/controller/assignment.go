package controller

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"physics-lab/backend/internal/middleware"
	"physics-lab/backend/internal/service"
)

type AssignmentController struct {
	svc *service.AssignmentService
}

func NewAssignmentController(svc *service.AssignmentService) *AssignmentController {
	return &AssignmentController{svc: svc}
}

// Create POST /classes/:id/assignments（教师发布作业）
func (ctl *AssignmentController) Create(c *gin.Context) {
	classID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || classID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid class id"})
		return
	}
	var req struct {
		Title      string `json:"title" binding:"required"`
		LevelID    int64  `json:"level_id" binding:"required"`
		Deadline   string `json:"deadline" binding:"required"` // ISO 8601 字符串
		DataWeight int    `json:"data_weight"`                 // 缺省 -> service 用默认 60
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "标题、关卡和截止时间不能为空"})
		return
	}
	deadline, err := time.Parse(time.RFC3339, req.Deadline)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "截止时间格式错误，需 ISO 8601"})
		return
	}
	a, err := ctl.svc.Create(classID, req.LevelID, req.Title, deadline, req.DataWeight, middleware.CurrentUserID(c))
	if err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "发布失败"})
		}
		return
	}
	c.JSON(http.StatusOK, a)
}

// ListByClass GET /classes/:id/assignments
func (ctl *AssignmentController) ListByClass(c *gin.Context) {
	classID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || classID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid class id"})
		return
	}
	rows, err := ctl.svc.ListByClass(classID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载失败"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

// ListMine GET /assignments/mine（学生：我的作业）
func (ctl *AssignmentController) ListMine(c *gin.Context) {
	rows, err := ctl.svc.ListByUser(middleware.CurrentUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载失败"})
		return
	}
	c.JSON(http.StatusOK, rows)
}

// Submit POST /assignments/:id/submit（学生提交作业）
//
// 两段式：readings 交测量数据，quiz_score 交自测成绩，可分别提交也可一次带上。
func (ctl *AssignmentController) Submit(c *gin.Context) {
	assignmentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assignmentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid assignment id"})
		return
	}
	var req struct {
		Experiment  string          `json:"experiment"`
		Readings    json.RawMessage `json:"readings"`
		QuizScore   *int            `json:"quiz_score"`
		RegionLabel string          `json:"region_label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体格式错误"})
		return
	}
	if len(req.Readings) == 0 && req.QuizScore == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "需要提交测量数据或自测成绩"})
		return
	}
	result, err := ctl.svc.Submit(assignmentID, middleware.CurrentUserID(c), service.AssignmentSubmitInput{
		Readings:    req.Readings,
		QuizScore:   req.QuizScore,
		RegionLabel: req.RegionLabel,
	})
	if err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "提交失败"})
		}
		return
	}
	c.JSON(http.StatusOK, result)
}

// SetWeight PATCH /assignments/:id/weight（教师调节综合得分比例）
func (ctl *AssignmentController) SetWeight(c *gin.Context) {
	assignmentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assignmentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid assignment id"})
		return
	}
	var req struct {
		DataWeight *int `json:"data_weight" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.DataWeight == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "data_weight 不能为空"})
		return
	}
	if err := ctl.svc.SetDataWeight(assignmentID, middleware.CurrentUserID(c), middleware.CurrentRole(c), *req.DataWeight); err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data_weight": *req.DataWeight,
		"quiz_weight": 100 - *req.DataWeight,
	})
}

// ListSubmissions GET /assignments/:id/submissions（教师查提交明细）
func (ctl *AssignmentController) ListSubmissions(c *gin.Context) {
	assignmentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assignmentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid assignment id"})
		return
	}
	rows, err := ctl.svc.ListSubmissions(assignmentID, middleware.CurrentUserID(c), middleware.CurrentRole(c))
	if err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载失败"})
		}
		return
	}
	c.JSON(http.StatusOK, rows)
}

// Delete DELETE /assignments/:id（教师删作业）
func (ctl *AssignmentController) Delete(c *gin.Context) {
	assignmentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || assignmentID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid assignment id"})
		return
	}
	if err := ctl.svc.Delete(assignmentID, middleware.CurrentUserID(c), middleware.CurrentRole(c)); err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除失败"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
