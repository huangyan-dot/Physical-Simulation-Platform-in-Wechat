package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupCORSTest(t *testing.T, allowAll bool, allow []string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(allowAll, allow))
	r.GET("/ping", func(c *gin.Context) { c.String(200, "pong") })
	return r
}

func TestCORS_AllowAllReflectsOrigin(t *testing.T) {
	r := setupCORSTest(t, true, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow-all: want reflected origin, got %q", got)
	}
	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Fatal("allow-all: missing Allow-Headers")
	}
}

func TestCORS_WhitelistAllowsKnownOrigin(t *testing.T) {
	r := setupCORSTest(t, false, []string{"http://localhost:5173", "https://lab.example.com"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://lab.example.com")
	r.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://lab.example.com" {
		t.Fatalf("whitelist: known origin should be allowed, got %q", got)
	}
}

func TestCORS_WhitelistBlocksUnknownOrigin(t *testing.T) {
	r := setupCORSTest(t, false, []string{"http://localhost:5173"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	r.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("whitelist: unknown origin should not get ACAO header, got %q", got)
	}
}

func TestCORS_PreflightReturns204(t *testing.T) {
	r := setupCORSTest(t, true, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("preflight: want 204, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("preflight: missing Allow-Methods")
	}
}

func TestCORS_NoOriginNoHeaders(t *testing.T) {
	// 非浏览器请求（无 Origin，如小程序/curl）不加 CORS 头
	r := setupCORSTest(t, true, nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("no-origin: should not set ACAO, got %q", got)
	}
	if w.Code != 200 {
		t.Fatalf("want 200, got %d", w.Code)
	}
}
