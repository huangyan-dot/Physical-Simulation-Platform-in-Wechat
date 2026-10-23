package response

import "github.com/gin-gonic/gin"

// OK 成功：直接返回数据本体（契约 0：不包 {code:0,...}）
func OK(c *gin.Context, data interface{}) {
	c.JSON(200, data)
}

// Error 统一错误：{code, message}，HTTP 状态码与 code 一致
func Error(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"code": status, "message": msg})
}
