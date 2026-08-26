package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"physics-lab/backend/internal/middleware"
	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/service"
)

type ClassController struct {
	svc *service.ClassService
}

func NewClassController(svc *service.ClassService) *ClassController {
	return &ClassController{svc: svc}
}

// List GET /classes
func (ctl *ClassController) List(c *gin.Context) {
	classes, err := ctl.svc.ListForUser(middleware.CurrentUserID(c), middleware.CurrentRole(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载班级失败"})
		return
	}
	if classes == nil {
		classes = []model.ClassView{}
	}
	c.JSON(http.StatusOK, classes)
}

// Create POST /classes（角色 teacher/admin，路由层 RequireRole）
func (ctl *ClassController) Create(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "班级名称不能为空"})
		return
	}
	cls, err := ctl.svc.Create(req.Name, middleware.CurrentUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建失败"})
		return
	}
	c.JSON(http.StatusOK, cls)
}

// Delete DELETE /classes/:id
func (ctl *ClassController) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid class id"})
		return
	}
	if err := ctl.svc.Delete(id, middleware.CurrentUserID(c), middleware.CurrentRole(c)); err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除失败"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// Detail GET /classes/:id（契约 §14）
func (ctl *ClassController) Detail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid class id"})
		return
	}
	view, err := ctl.svc.Detail(id, middleware.CurrentUserID(c), middleware.CurrentRole(c))
	if err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载班级失败"})
		}
		return
	}
	c.JSON(http.StatusOK, view)
}

// Update PUT /classes/:id（契约 §15）：改名
func (ctl *ClassController) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid class id"})
		return
	}
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "班级名称不能为空"})
		return
	}
	cls, err := ctl.svc.Rename(id, req.Name, middleware.CurrentUserID(c), middleware.CurrentRole(c))
	if err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败"})
		}
		return
	}
	c.JSON(http.StatusOK, cls)
}

// RemoveMember DELETE /classes/:id/members/:userId（契约 §16）
func (ctl *ClassController) RemoveMember(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid class id"})
		return
	}
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil || userID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid user id"})
		return
	}
	if err := ctl.svc.RemoveMember(id, userID, middleware.CurrentUserID(c), middleware.CurrentRole(c)); err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "移除失败"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "removed"})
}

// AddMember POST /classes/:id/members
func (ctl *ClassController) AddMember(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid class id"})
		return
	}
	var req struct {
		UserID int64 `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "user_id 不能为空"})
		return
	}
	if err := ctl.svc.AddMember(id, req.UserID, middleware.CurrentUserID(c), middleware.CurrentRole(c)); err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "添加失败"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "added"})
}
