package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// v0.4 新增接口的集成测试（契约 §14-§18）。

// loginAs 登录 dev 账号并返回 token。dev_student -> uid 2，dev_teacher -> uid 3。
func loginAs(t *testing.T, r http.Handler, mock sqlmock.Sqlmock, code string, uid int64, role, name string) string {
	t.Helper()
	mock.ExpectQuery("SELECT").WillReturnRows(
		userRows().AddRow(uid, "oDEV_"+code[len("dev_"):], role, name, "2023001", time.Now(), time.Now()),
	)
	// 登录审计日志 INSERT（GORM Create 包装在事务中）
	mock.ExpectBegin()
	mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	body, _ := json.Marshal(map[string]string{"code": code})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login %s: want 200, got %d, body=%s", code, w.Code, w.Body.String())
	}
	var login struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &login); err != nil || login.Token == "" {
		t.Fatalf("login %s: no token, body=%s", code, w.Body.String())
	}
	return login.Token
}

func authedReq(method, path, token string, body []byte) *http.Request {
	req, _ := http.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}

func classRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "name", "teacher_id", "created_at", "updated_at"})
}

// TestExperimentsList §17：GET /experiments 返回元数据列表，不含 config/target。
func TestExperimentsList(t *testing.T) {
	deps, mock, _ := newFullDeps(t, true)
	r := New(deps)
	token := loginAs(t, r, mock, "dev_student", 2, "student", "测试同学")

	mock.ExpectQuery("SELECT").WillReturnRows(
		sqlmock.NewRows([]string{"id", "code", "name", "render_mode"}).
			AddRow(1, "newton_ring", "牛顿环实验", "mixed_3d_2d").
			AddRow(2, "oscilloscope", "示波器实验", "mixed_3d_2d").
			AddRow(3, "pendulum", "单摆实验", "mixed_3d_2d"),
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodGet, "/experiments", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("GET /experiments: want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("want 3 experiments, got %d, body=%s", len(list), w.Body.String())
	}
	if _, ok := list[0]["config"]; ok {
		t.Fatalf("list should not contain config, body=%s", w.Body.String())
	}
	if _, ok := list[0]["target"]; ok {
		t.Fatalf("list should not contain target, body=%s", w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

// TestAuditMine §18：学生只能看自己的日志（user_id 强制为本人）。
func TestAuditMine(t *testing.T) {
	deps, mock, _ := newFullDeps(t, true)
	r := New(deps)
	token := loginAs(t, r, mock, "dev_student", 2, "student", "测试同学")

	// List：COUNT 带 user_id=2 过滤（证明强制本人），再 SELECT 当前页
	mock.ExpectQuery("SELECT count").WithArgs(int64(2)).WillReturnRows(
		sqlmock.NewRows([]string{"count"}).AddRow(1),
	)
	mock.ExpectQuery("SELECT").WillReturnRows(
		sqlmock.NewRows([]string{"id", "user_id", "action", "level_id", "score", "detail", "created_at"}).
			AddRow(9, 2, "submit", 1, 100, "ok", time.Now()),
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodGet, "/audit/mine", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("GET /audit/mine: want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"total":1`)) {
		t.Fatalf("want total=1, body=%s", w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

// TestClassDetailAsMember §14：本班学生可看班级详情（含成员列表）。
func TestClassDetailAsMember(t *testing.T) {
	deps, mock, _ := newFullDeps(t, true)
	r := New(deps)
	token := loginAs(t, r, mock, "dev_student", 2, "student", "测试同学")

	now := time.Now()
	mock.ExpectQuery("SELECT").WillReturnRows(classRows().AddRow(1, "物理实验1班", 3, now, now)) // FindByID
	mock.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))   // IsMember: 是成员
	mock.ExpectQuery("SELECT").WillReturnRows( // MembersWithUser
		sqlmock.NewRows([]string{"user_id", "name", "student_no", "joined_at"}).
			AddRow(2, "测试同学", "2023001", now),
	)
	mock.ExpectQuery("SELECT").WillReturnRows( // TeacherName
		sqlmock.NewRows([]string{"name"}).AddRow("张老师"),
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodGet, "/classes/1", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("GET /classes/1: want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	for _, want := range []string{`"teacher_name":"张老师"`, `"members"`, `"student_no":"2023001"`} {
		if !bytes.Contains(w.Body.Bytes(), []byte(want)) {
			t.Fatalf("body missing %s: %s", want, w.Body.String())
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

// TestClassDetailForbiddenForOutsider §14：非本班学生看详情 -> 403。
func TestClassDetailForbiddenForOutsider(t *testing.T) {
	deps, mock, _ := newFullDeps(t, true)
	r := New(deps)
	token := loginAs(t, r, mock, "dev_student", 2, "student", "测试同学")

	mock.ExpectQuery("SELECT").WillReturnRows(classRows().AddRow(1, "物理实验1班", 3, time.Now(), time.Now()))
	mock.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0)) // 非成员
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodGet, "/classes/1", token, nil))

	if w.Code != http.StatusForbidden {
		t.Fatalf("outsider GET /classes/1: want 403, got %d, body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

// TestClassRenameByOwnerTeacher §15：本班教师改名成功。
func TestClassRenameByOwnerTeacher(t *testing.T) {
	deps, mock, _ := newFullDeps(t, true)
	r := New(deps)
	token := loginAs(t, r, mock, "dev_teacher", 3, "teacher", "张老师")

	mock.ExpectQuery("SELECT").WillReturnRows(classRows().AddRow(1, "物理实验1班", 3, time.Now(), time.Now()))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	// 班级改名审计日志 INSERT（GORM Create 包装在事务中）
	mock.ExpectBegin()
	mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	body, _ := json.Marshal(map[string]string{"name": "物理实验1班（秋）"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodPut, "/classes/1", token, body))

	if w.Code != http.StatusOK {
		t.Fatalf("PUT /classes/1: want 200, got %d, body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("物理实验1班（秋）")) {
		t.Fatalf("name not updated in response: %s", w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

// TestClassRenameForbiddenForOtherTeacher §15：别班教师改名 -> 403，且不写库。
func TestClassRenameForbiddenForOtherTeacher(t *testing.T) {
	deps, mock, _ := newFullDeps(t, true)
	r := New(deps)
	token := loginAs(t, r, mock, "dev_teacher", 3, "teacher", "张老师")

	mock.ExpectQuery("SELECT").WillReturnRows(classRows().AddRow(1, "物理实验1班", 99, time.Now(), time.Now())) // 别人的班
	body, _ := json.Marshal(map[string]string{"name": "抢班"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodPut, "/classes/1", token, body))

	if w.Code != http.StatusForbidden {
		t.Fatalf("PUT other teacher's class: want 403, got %d, body=%s", w.Code, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("should not write DB after 403: %v", err)
	}
}

// TestClassRemoveMember §16：本班教师移成员成功；关系不存在 -> 404。
func TestClassRemoveMember(t *testing.T) {
	deps, mock, _ := newFullDeps(t, true)
	r := New(deps)
	token := loginAs(t, r, mock, "dev_teacher", 3, "teacher", "张老师")

	// 成功移除
	mock.ExpectQuery("SELECT").WillReturnRows(classRows().AddRow(1, "物理实验1班", 3, time.Now(), time.Now()))
	mock.ExpectBegin()
	mock.ExpectExec("DELETE").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	// 移除成员审计日志 INSERT（GORM Create 包装在事务中）
	mock.ExpectBegin()
	mock.ExpectExec("INSERT").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, authedReq(http.MethodDelete, "/classes/1/members/2", token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("DELETE member: want 200, got %d, body=%s", w.Code, w.Body.String())
	}

	// 成员关系不存在 -> 404
	mock.ExpectQuery("SELECT").WillReturnRows(classRows().AddRow(1, "物理实验1班", 3, time.Now(), time.Now()))
	mock.ExpectBegin()
	mock.ExpectExec("DELETE").WillReturnResult(sqlmock.NewResult(0, 0)) // RowsAffected=0
	mock.ExpectCommit()
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, authedReq(http.MethodDelete, "/classes/1/members/999", token, nil))
	if w2.Code != http.StatusNotFound {
		t.Fatalf("DELETE non-member: want 404, got %d, body=%s", w2.Code, w2.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}
