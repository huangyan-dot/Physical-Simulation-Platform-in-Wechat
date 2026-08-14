package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 内存令牌桶限流器。按 key（user_id 或 IP）维护一个桶：
// 每 window 补充 capacity 个令牌，请求消耗 1 个，桶空则 429。
// 单机够用；多实例部署需换 Redis。仅给少数敏感接口（登录、刷分）挂载。
type RateLimiter struct {
	mu        sync.Mutex
	buckets   map[string]*bucket
	capacity  int           // 窗口内允许请求数
	window    time.Duration // 窗口长度
}

type bucket struct {
	tokens  int
	updated time.Time
}

// NewRateLimiter capacity=窗口内最大请求数，window=统计窗口。
func NewRateLimiter(capacity int, window time.Duration) *RateLimiter {
	if capacity <= 0 {
		capacity = 60
	}
	if window <= 0 {
		window = time.Minute
	}
	return &RateLimiter{buckets: make(map[string]*bucket), capacity: capacity, window: window}
}

// allow 取一个令牌；返回是否放行。窗口滚动时按时间比例补充令牌（不超 capacity）。
func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	b := rl.buckets[key]
	if b == nil {
		b = &bucket{tokens: rl.capacity - 1, updated: now}
		rl.buckets[key] = b
		return true
	}
	// 补充：按流逝窗口数补令牌
	elapsed := now.Sub(b.updated)
	if elapsed >= rl.window {
		// 过了至少一个完整窗口，重置为满额再扣
		b.tokens = rl.capacity
	} else {
		refill := int(elapsed / rl.window * time.Duration(rl.capacity))
		if refill > 0 {
			b.tokens += refill
			if b.tokens > rl.capacity {
				b.tokens = rl.capacity
			}
			b.updated = b.updated.Add(time.Duration(refill) * rl.window / time.Duration(max1(rl.capacity)))
		}
	}
	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	b.updated = now
	return true
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// RateLimit 按 keyFn 取限流 key。无 key（keyFn 返回空）则不限流直接放行。
// 超限返回 429（契约 §0）并带 Retry-After 提示剩余窗口秒数。
func RateLimit(rl *RateLimiter, keyFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := keyFn(c)
		if key == "" || rl == nil {
			c.Next()
			return
		}
		if !rl.allow(key) {
			c.Header("Retry-After", strconv.Itoa(int(rl.window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": 429, "message": "请求过于频繁，请稍后再试",
			})
			return
		}
		c.Next()
	}
}

// KeyByUserID 鉴权后按 user_id 限流；未鉴权时回退空（不限）。
func KeyByUserID(c *gin.Context) string {
	return strconv.FormatInt(CurrentUserID(c), 10)
}

// KeyByIP 按客户端 IP 限流（用于未鉴权的登录接口）。
func KeyByIP(c *gin.Context) string {
	return c.ClientIP()
}
