package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func setupRateTest(t *testing.T, rl *RateLimiter, keyFn func(*gin.Context) string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", RateLimit(rl, keyFn), func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func TestRateLimit_AllowsWithinCapacity(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)
	r := setupRateTest(t, rl, KeyByIP)
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("req %d: want 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimit_BlocksOverLimit429(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	r := setupRateTest(t, rl, KeyByIP)

	// 前 2 个放行
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("req %d: want 200, got %d", i+1, w.Code)
		}
	}
	// 第 3 个超限 -> 429 + Retry-After
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("over-limit: want 429, got %d", w.Code)
	}
	if w.Result().Header.Get("Retry-After") == "" {
		t.Fatal("over-limit: missing Retry-After header")
	}
}

func TestRateLimit_ResetsAfterWindow(t *testing.T) {
	rl := NewRateLimiter(1, 30*time.Millisecond)
	r := setupRateTest(t, rl, KeyByIP)

	// 用光配额
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("first req: want 200, got %d", w.Code)
	}
	// 立刻再请求 -> 429
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("second req: want 429, got %d", w.Code)
	}
	// 等窗口过去 -> 恢复
	time.Sleep(40 * time.Millisecond)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("after window: want 200, got %d", w.Code)
	}
}

func TestRateLimit_DistinctKeysIndependent(t *testing.T) {
	// user1 用光不影响 user2
	rl := NewRateLimiter(1, time.Minute)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", RateLimit(rl, func(c *gin.Context) string { return c.Query("u") }), func(c *gin.Context) { c.String(200, "ok") })

	req := func(u string) *http.Request {
		r, _ := http.NewRequest(http.MethodGet, "/?u="+u, nil)
		return r
	}
	// user1 放行 1 次，第 2 次 429
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req("u1"))
	if w.Code != 200 {
		t.Fatalf("u1 first: want 200, got %d", w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req("u1"))
	if w.Code != 429 {
		t.Fatalf("u1 second: want 429, got %d", w.Code)
	}
	// user2 仍可放行
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req("u2"))
	if w.Code != 200 {
		t.Fatalf("u2: want 200, got %d", w.Code)
	}
}

func TestRateLimit_NilOrEmptyKeyBypasses(t *testing.T) {
	// keyFn 返回空 -> 不限流，恒放行
	rl := NewRateLimiter(1, time.Minute)
	r := setupRateTest(t, rl, func(*gin.Context) string { return "" })
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("empty-key req %d: want 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimit_ConcurrentSafe(t *testing.T) {
	// 并发打同一个 key 不应 panic / race
	rl := NewRateLimiter(50, time.Minute)
	r := setupRateTest(t, rl, KeyByIP)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		}()
	}
	wg.Wait()
}
