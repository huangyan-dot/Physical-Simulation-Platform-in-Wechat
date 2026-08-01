package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"physics-lab/backend/internal/middleware"
	"physics-lab/backend/internal/model"
	"physics-lab/backend/internal/service"
)

type LevelController struct {
	svc *service.LevelService
}

func NewLevelController(svc *service.LevelService) *LevelController {
	return &LevelController{svc: svc}
}

// List GET /levels
func (ctl *LevelController) List(c *gin.Context) {
	levels, err := ctl.svc.ListForUser(middleware.CurrentUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载关卡失败"})
		return
	}
	if levels == nil {
		levels = []model.LevelView{}
	}
	c.JSON(http.StatusOK, levels)
}

// GetExperiment GET /experiments/:id（:id 是 level_id）
func (ctl *LevelController) GetExperiment(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "invalid level id"})
		return
	}
	exp, err := ctl.svc.GetExperiment(id)
	if err != nil {
		if !writeServiceErr(c, err) {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "加载实验失败"})
		}
		return
	}
	c.JSON(http.StatusOK, exp)
}
