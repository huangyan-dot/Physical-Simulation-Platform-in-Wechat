package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"physics-lab/backend/internal/repository"
	"physics-lab/backend/internal/service"
)

// newTestController 用 sqlmock 假造一个 MySQL，走通 repository → service → controller 全链
func newTestController(t *testing.T) (*UserController, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	gormDB, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	repo := repository.NewUserRepository(gormDB)
	svc := service.NewUserService(repo)
	return NewUserController(svc), mock
}

func userColumns() []string {
	return []string{"id", "openid", "role", "name", "student_no", "created_at", "updated_at"}
}

func serve(ctl *UserController, path string) *httptest.ResponseRecorder {
	r := gin.New()
	r.GET("/users/:id", ctl.GetUser)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	r.ServeHTTP(w, req)
	return w
}

func TestGetUser_OK(t *testing.T) {
	ctl, mock := newTestController(t)

	now := time.Now()
	rows := sqlmock.NewRows(userColumns()).
		AddRow(1, "oTEST_001", "student", "测试同学", "2023001", now, now)
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	w := serve(ctl, "/users/1")

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"openid":"oTEST_001"`) {
		t.Fatalf("response missing openid, body=%s", w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	ctl, mock := newTestController(t)

	// 空结果集 → GORM 返回 ErrRecordNotFound → controller 应返回 404
	mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(userColumns()))

	w := serve(ctl, "/users/999")

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d, body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":404`) {
		t.Fatalf("error body should follow unified format, body=%s", w.Body.String())
	}
}

func TestGetUser_BadID(t *testing.T) {
	ctl, mock := newTestController(t) // 不应发生任何 SQL 调用

	w := serve(ctl, "/users/abc")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d, body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("bad id should not touch DB: %v", err)
	}
}
