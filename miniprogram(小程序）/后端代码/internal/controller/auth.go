package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"physics-lab/backend/internal/middleware"
	"physics-lab/backend/internal/service"
)

// AuthController 登录 / 当前用户 / 补全学籍
type AuthController struct {
	svc *service.AuthService
}

func NewAuthController(svc *service.AuthService) *AuthController {
	return &AuthController{svc: svc}
}

// Login POST /auth/login（别名 POST /login）
func (ctl *AuthController) Login(c *gin.Context) {
	var req struct {
		Code       string `json:"code"`
		Role       string `json:"role"`        // "student" / "teacher"
		InviteCode string `json:"invite_code"` // 教师邀请码
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "code 不能为空"})
		return
	}
	token, user, err := ctl.svc.Login(service.LoginInput{
		Code:       req.Code,
		Role:       req.Role,
		InviteCode: req.InviteCode,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

// Me GET /auth/me
func (ctl *AuthController) Me(c *gin.Context) {
	user, err := ctl.svc.Me(middleware.CurrentUserID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// CompleteProfile PUT /users/:id —— 仅本人可改
func (ctl *AuthController) CompleteProfile(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid user id"})
		return
	}
	if middleware.CurrentUserID(c) != id {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "不能修改他人信息"})
		return
	}

	var req struct {
		Name      string `json:"name" binding:"required"`
		StudentNo string `json:"student_no" binding:"required"`
		ClassID   *int64 `json:"class_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "姓名和学号不能为空"})
		return
	}

	user, err := ctl.svc.CompleteProfile(id, req.Name, req.StudentNo, req.ClassID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "保存失败"})
		return
	}
	// 契约 §0/§3：成功响应直接返回数据本体（不包 {code:0,...} 也不用 {data:...}）。
	// 前端用 {...user, ...res.data} 合并；返回裸 user 即让 res.data||res 命中。
	c.JSON(http.StatusOK, user)
}
