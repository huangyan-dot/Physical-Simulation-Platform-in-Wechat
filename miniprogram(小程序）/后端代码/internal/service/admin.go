package service

import (
	"strconv"

	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/repository"
)

// AdminService 管理员审计查询。仅 admin 角色可调用（路由层 RequireRole 守卫）。
type AdminService struct {
	logRepo *repository.OperationLogRepository
}

func NewAdminService(logRepo *repository.OperationLogRepository) *AdminService {
	return &AdminService{logRepo: logRepo}
}

// LogListView GET /admin/operation-logs 响应
type LogListView struct {
	Page    int                   `json:"page"`
	Size    int                   `json:"size"`
	Total   int64                 `json:"total"`
	Records []model.OperationLog  `json:"records"`
}

// ListLogs 分页查询审计日志。
func (s *AdminService) ListLogs(userID, levelID int64, action string, page, size int) (*LogListView, error) {
	q := repository.LogQuery{
		UserID:  userID,
		LevelID: levelID,
		Action:  action,
		Page:    page,
		Size:    size,
	}
	logs, total, err := s.logRepo.List(q)
	if err != nil {
		return nil, err
	}
	if logs == nil {
		logs = []model.OperationLog{}
	}
	return &LogListView{
		Page: q.Page, Size: q.Size, Total: total, Records: logs,
	}, nil
}

// Atoi 默认 0（不过滤）。用于把 query string 解析为 int64。
func Atoi(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}
