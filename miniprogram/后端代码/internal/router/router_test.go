package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPing(t *testing.T) {
	// 仅填 Mode：模拟 MySQL 不可用时的最小服务模式
	r := New(Deps{Mode: gin.TestMode})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /ping: want status 200, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("GET /ping: response is not JSON: %v", err)
	}
	if body["message"] != "pong" {
		t.Fatalf("GET /ping: want message=pong, got %q", body["message"])
	}
}

func TestBusinessRoutesNotRegisteredWhenDBDown(t *testing.T) {
	// DB 不可用 -> Deps 无任何 controller -> 业务接口不注册，全部 404
	r := New(Deps{Mode: gin.TestMode})

	for _, path := range []string{"/users/1", "/levels", "/classes", "/progress/mine"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("GET %s without DB: want 404, got %d", path, w.Code)
		}
	}
}
