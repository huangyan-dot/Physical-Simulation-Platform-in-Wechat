package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"physics-lab/backend/internal/controller"
	"physics-lab/backend/internal/middleware"
	"physics-lab/backend/internal/pkg/jwt"
	"physics-lab/backend/internal/repository"
	"physics-lab/backend/internal/service"
)

// rateLimiter 测试用：1 分钟容量 n。
func rateLimiter(n int) *middleware.RateLimiter {
	return middleware.NewRateLimiter(n, time.Minute)
}

func userRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "openid", "role", "name", "student_no", "created_at", "updated_at"})
}

func levelRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "experiment_id", "name", "order_no", "difficulty", "prereq_level_id", "created_at", "updated_at"})
}

func expRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "code", "name", "render_mode", "config", "target", "created_at", "updated_at"})
}

// newFullDeps 用 sqlmock 假造 MySQL，装配全链 repo->service->controller，返回 Deps 与 mock。
// allowDev 控制后门开关；limiters 为 nil 表示不限流（多数测试用）。
func newFullDeps(t *testing.T, allowDev bool) (Deps, sqlmock.Sqlmock, *jwt.Manager) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	classRepo := repository.NewClassRepository(db)
	levelRepo := repository.NewLevelRepository(db)
	expRepo := repository.NewExperimentRepository(db)
	progRepo := repository.NewProgressRepository(db)
	logRepo := repository.NewOperationLogRepository(db)

	jm := jwt.New("test-secret", 1)

	authSvc := service.NewAuthService(userRepo, classRepo, jm, nil, allowDev, "TEST_INVITE") // wechat=nil：测试只走 dev_ 后门
	classSvc := service.NewClassService(classRepo)
	levelSvc := service.NewLevelService(levelRepo, expRepo, progRepo)
	progSvc := service.NewProgressService(progRepo, levelRepo, expRepo, logRepo, classRepo, userRepo)
	userSvc := service.NewUserService(userRepo)
	adminSvc := service.NewAdminService(logRepo)

	return Deps{
		Mode:             gin.TestMode,
		JM:               jm,
		UserCtl:          controller.NewUserController(userSvc),
		AuthCtl:          controller.NewAuthController(authSvc),
		ClassCtl:         controller.NewClassController(classSvc),
		LevelCtl:         controller.NewLevelController(levelSvc),
		ProgressCtl:      controller.NewProgressController(progSvc),
		AdminCtl:         controller.NewAdminController(adminSvc),
		AllowDevBackdoor: allowDev,
		CORSAllowAll:     true,
	}, mock, jm
}

// TestProtectedRouteRequiresAuth 无 token 访问需鉴权接口应 401（不触库）。
func TestProtectedRouteRequiresAuth(t *testing.T) {
	deps, _, _ := newFullDeps(t, true)
	r := New(deps)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/levels", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /levels without token: want 401, got %d", w.Code)
	}
}

// TestDevLoginAndMe 全链：dev_student 登录拿 token -> 带 token 调 /auth/me 回显。
// 覆盖路由注册、JWT 签发/校验、middleware 注入 uid、controller->service->repository。
func TestDevLoginAndMe(t *testing.T) {
	deps, mock, _ := newFullDeps(t, true)
	r := New(deps)

	now := time.Now()

	// 登录：FindByOpenID("oDEV_STUDENT") 命中已种子化的 dev 学生
	mock.ExpectQuery("SELECT").WillReturnRows(
		userRows().AddRow(1, "oDEV_STUDENT", "student", "测试同学", "2023001", now, now),
	)

	body, _ := json.Marshal(map[string]string{"code": "dev_student"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login: want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var login struct {
		Token string `json:"token"`
		User  struct {
			ID           int64  `json:"id"`
			Role         string `json:"role"`
			Name         string `json:"name"`
			NeedComplete bool   `json:"need_complete"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &login); err != nil {
		t.Fatalf("login: bad json: %v, body=%s", err, w.Body.String())
	}
	if login.Token == "" {
		t.Fatalf("login: no token, body=%s", w.Body.String())
	}
	if login.User.Role != "student" || login.User.ID != 1 {
		t.Fatalf("login: unexpected user %+v", login.User)
	}
	if login.User.NeedComplete {
		t.Fatalf("login: dev student should be already complete")
	}

	// /auth/me：FindByID(1)
	mock.ExpectQuery("SELECT").WillReturnRows(
		userRows().AddRow(1, "oDEV_STUDENT", "student", "测试同学", "2023001", now, now),
	)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/auth/me", nil)
	req2.Header.Set("Authorization", "Bearer "+login.Token)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("/auth/me: want 200, got %d, body=%s", w2.Code, w2.Body.String())
	}
	if !bytes.Contains(w2.Body.Bytes(), []byte(`"openid":"oDEV_STUDENT"`)) {
		t.Fatalf("/auth/me: missing openid, body=%s", w2.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sql expectations: %v", err)
	}
}

// TestCompleteProfileForbiddenForOthers PUT /users/:id 仅本人可改：用 user1 的 token 改 user2 应 403（不触库）。
func TestCompleteProfileForbiddenForOthers(t *testing.T) {
	deps, mock, _ := newFullDeps(t, true)
	r := New(deps)

	// 登录拿 user1 的 token
	mock.ExpectQuery("SELECT").WillReturnRows(
		userRows().AddRow(1, "oDEV_STUDENT", "student", "测试同学", "2023001", time.Now(), time.Now()),
	)
	body, _ := json.Marshal(map[string]string{"code": "dev_student"})
	wl := httptest.NewRecorder()
	rl, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rl.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wl, rl)
	var login struct {
		Token string `json:"token"`
	}
	json.Unmarshal(wl.Body.Bytes(), &login)

	// 用 user1 的 token 改 user2 -> 403（controller 在触库前就拒绝）
	w := httptest.NewRecorder()
	pbody, _ := json.Marshal(map[string]string{"name": "x", "student_no": "y"})
	req, _ := http.NewRequest(http.MethodPut, "/users/2", bytes.NewReader(pbody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+login.Token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("PUT /users/2 as user1: want 403, got %d, body=%s", w.Code, w.Body.String())
	}
	// 不应有额外 SQL：鉴权+归属校验在 service 之前
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("should not touch DB after 403: %v", err)
	}
}

// TestDevBackdoorDisabled 关闭后门开关时：dev_ code 被拒(400)，/login 别名不注册(404)。
func TestDevBackdoorDisabled(t *testing.T) {
	deps, _, _ := newFullDeps(t, false) // allowDev=false
	r := New(deps)

	// dev_student code 应被拒绝（service 返错 -> 400），且不触库（不开 wx、不查 openid）
	body, _ := json.Marshal(map[string]string{"code": "dev_student"})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("dev code with backdoor off: want 400, got %d, body=%s", w.Code, w.Body.String())
	}

	// /login 别名不应注册 -> 404
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("/login alias with backdoor off: want 404, got %d", w2.Code)
	}
}

// TestSubmitRateLimit /progress/submit 限流：超配额后第 N+1 次返回 429。
func TestSubmitRateLimit(t *testing.T) {
	deps, mock, _ := newFullDeps(t, true)
	// 容量 2 的限流器：前 2 次放行，第 3 次 429
	deps.SubmitLimiter = rateLimiter(2)
	r := New(deps)

	// 登录拿 token
	mock.ExpectQuery("SELECT").WillReturnRows(
		userRows().AddRow(1, "oDEV_STUDENT", "student", "测试同学", "2023001", time.Now(), time.Now()),
	)
	body, _ := json.Marshal(map[string]string{"code": "dev_student"})
	wl := httptest.NewRecorder()
	rl, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	rl.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(wl, rl)
	var login struct {
		Token string `json:"token"`
	}
	json.Unmarshal(wl.Body.Bytes(), &login)

	// 两次合法 submit：每次都查 level+experiment+ensure progress+upsert
	for i := 0; i < 2; i++ {
		mock.ExpectQuery("SELECT").WillReturnRows(levelRows().AddRow(1, 1, "牛顿环实验", 1, 2, nil, time.Now(), time.Now()))
		mock.ExpectQuery("SELECT").WillReturnRows(expRows().AddRow(1, "newton_ring", "牛顿环实验", "mixed_3d_2d", []byte(`{}`), []byte(`{"method":"least_squares_R","lens_radius_mm":855,"wavelength_nm":589.3,"pass_score":60}`), time.Now(), time.Now()))
		mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "level_id", "best_score", "last_score", "attempts", "passed", "created_at", "updated_at"}).AddRow(1, 1, 1, 0, 0, 0, false, time.Now(), time.Now()))
		mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1)) // operation_logs
	}

	submit := func() *httptest.ResponseRecorder {
		payload := `{"level_id":1,"experiment":"newton_ring","readings":[{"k":1,"r":0.7},{"k":2,"r":1.0}]}`
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/progress/submit", bytes.NewReader([]byte(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+login.Token)
		r.ServeHTTP(w, req)
		return w
	}
	// 前 2 次不 429
	for i := 0; i < 2; i++ {
		if w := submit(); w.Code == http.StatusTooManyRequests {
			t.Fatalf("submit %d: should not be rate limited yet, got 429", i+1)
		}
	}
	// 第 3 次 -> 429（限流在 RequireRole 之前，不触库）
	w3 := submit()
	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("submit 3rd: want 429, got %d, body=%s", w3.Code, w3.Body.String())
	}
}
