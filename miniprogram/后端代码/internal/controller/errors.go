package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"physics-lab/backend/internal/service"
)

// writeServiceErr 把 service 层 sentinel 错误映射成统一错误响应。
// 命中返回 true。
func writeServiceErr(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, service.ErrBadRequest):
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
	case errors.Is(err, service.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "资源不存在"})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "没有权限执行此操作"})
	case errors.Is(err, service.ErrConflict):
		c.JSON(http.StatusConflict, gin.H{"code": 409, "message": "already a member"})
	default:
		return false
	}
	return true
}
