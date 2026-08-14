package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"physics-lab/backend/internal/middleware"
	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/repository"
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

// JoinByCode POST /classes/join（学生凭班级码自助加入）
func (ctl *ClassController) JoinByCode(c *gin.Context) {
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "班级码不能为空"})
		return
	}
	cls, err := ctl.svc.JoinByCode(req.Code, middleware.CurrentUserID(c))
	if err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加入失败"})
		}
		return
	}
	c.JSON(http.StatusOK, cls)
}

// ListMembers GET /classes/:id/members（教师查名单）
func (ctl *ClassController) ListMembers(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid class id"})
		return
	}
	members, err := ctl.svc.ListMembers(id, middleware.CurrentUserID(c), middleware.CurrentRole(c))
	if err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载成员失败"})
		}
		return
	}
	if members == nil {
		members = []repository.MemberView{}
	}
	c.JSON(http.StatusOK, members)
}
