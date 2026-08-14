package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// corsConfig CORS 配置：由 main.go 按 config 填充。
type corsConfig struct {
	allowAll   bool     // debug 模式放行所有 Origin
	allowList  []string // 上线模式显式白名单（可含 http://localhost:port 等）
	allowHeads []string
	allowMethods []string
}

// CORS 跨域中间件。
// 微信小程序本身不受浏览器 CORS 约束，但 Apifox 调试、本地 H5 联调、
// 以及将来若转 Web 端都需要。开发期 allowAll=true 放行所有 Origin；上线收紧到白名单。
func CORS(allowAll bool, allowOrigins []string) gin.HandlerFunc {
	cfg := corsConfig{
		allowAll:     allowAll,
		allowList:    allowOrigins,
		allowHeads:   []string{"Origin", "Content-Type", "Authorization"},
		allowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			switch {
			case cfg.allowAll:
				c.Header("Access-Control-Allow-Origin", origin)
			case contains(cfg.allowList, origin):
				c.Header("Access-Control-Allow-Origin", origin)
			}
			c.Header("Access-Control-Allow-Headers", join(cfg.allowHeads))
			c.Header("Access-Control-Allow-Methods", join(cfg.allowMethods))
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Vary", "Origin")
		}
		// 预检请求直接 204，不进后续 handler
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func join(list []string) string {
	if len(list) == 0 {
		return ""
	}
	out := list[0]
	for _, v := range list[1:] {
		out += ", " + v
	}
	return out
}
