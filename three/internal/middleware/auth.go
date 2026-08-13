package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"physics-lab/backend/internal/pkg/jwt"
)

// context key
const (
	CtxUserID = "userID"
	CtxRole   = "role"
)

// Auth 校验 Bearer token，把 uid/role 注入 context
func Auth(jm *jwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "missing token"})
			return
		}
		claims, err := jm.Parse(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "token expired"})
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxRole, claims.Role)
		c.Next()
	}
}

// RequireRole 仅允许指定角色通过
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		r := CurrentRole(c)
		for _, want := range roles {
			if r == want {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "没有权限执行此操作"})
	}
}

// CurrentUserID 从 context 取当前用户 ID
func CurrentUserID(c *gin.Context) int64 {
	v, _ := c.Get(CtxUserID)
	id, _ := v.(int64)
	return id
}

// CurrentRole 从 context 取当前角色
func CurrentRole(c *gin.Context) string {
	v, _ := c.Get(CtxRole)
	r, _ := v.(string)
	return r
}
