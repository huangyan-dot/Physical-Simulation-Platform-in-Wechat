package service

import (
	"testing"

	"physics-lab/backend/internal/model"
)

// 学生受前置关限制；教师/管理员一律开放（备课、演示、验题需要随时进任意实验）。
func TestComputeStatus_LockRulesByRole(t *testing.T) {
	prereq := int64(1)
	firstLevel := model.Level{ID: 1}                          // 首关：无前置
	secondLevel := model.Level{ID: 2, PrereqLevelID: &prereq} // 第二关：需通过第 1 关

	noProgress := map[int64]*model.UserProgress{}
	prereqPassed := map[int64]*model.UserProgress{
		1: {LevelID: 1, Passed: true, Attempts: 1},
	}

	cases := []struct {
		name  string
		level model.Level
		prog  map[int64]*model.UserProgress
		role  string
		want  string
	}{
		{"学生-首关无前置-开放", firstLevel, noProgress, "student", "unlocked"},
		{"学生-前置未过-锁定", secondLevel, noProgress, "student", "locked"},
		{"学生-前置已过-开放", secondLevel, prereqPassed, "student", "unlocked"},

		{"教师-前置未过-仍开放", secondLevel, noProgress, "teacher", "unlocked"},
		{"管理员-前置未过-仍开放", secondLevel, noProgress, "admin", "unlocked"},

		// 角色缺失（token 无 role）时按最严格的学生规则处理，避免绕过解锁链
		{"空角色-前置未过-锁定", secondLevel, noProgress, "", "locked"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeStatus(tc.level, tc.prog, tc.role); got != tc.want {
				t.Errorf("computeStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}

// 教师已有作答记录时，仍如实返回 passed / in_progress，而不是一律 unlocked。
func TestComputeStatus_TeacherKeepsRealProgress(t *testing.T) {
	prereq := int64(1)
	lv := model.Level{ID: 2, PrereqLevelID: &prereq}

	passed := map[int64]*model.UserProgress{2: {LevelID: 2, Passed: true, Attempts: 3}}
	if got := computeStatus(lv, passed, "teacher"); got != "passed" {
		t.Errorf("已通过的关卡 = %q, want \"passed\"", got)
	}

	inProgress := map[int64]*model.UserProgress{2: {LevelID: 2, Attempts: 1}}
	if got := computeStatus(lv, inProgress, "teacher"); got != "in_progress" {
		t.Errorf("进行中的关卡 = %q, want \"in_progress\"", got)
	}
}
