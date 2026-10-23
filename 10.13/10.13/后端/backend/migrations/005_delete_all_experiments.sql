-- 2026-08-21 清空全部实验数据（实验定义 / 关卡 / 进度 / 实验提交日志）
USE physics_lab;

-- 提交日志中带 level_id 的为实验操作记录，一并清除；登录等日志保留
DELETE FROM operation_logs WHERE level_id IS NOT NULL;
DELETE FROM user_progresses;
DELETE FROM levels;
DELETE FROM experiments;
