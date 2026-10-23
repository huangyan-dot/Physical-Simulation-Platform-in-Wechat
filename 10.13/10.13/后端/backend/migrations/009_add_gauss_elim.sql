-- 009_add_gauss_elim.sql - 数值分析实验批次 L3：高斯消去法解线性方程组（数据组）
-- 2026-08-26 新增（插队：插值/积分等批次仍在排队）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：007（L2 牛顿迭代，level 12）。本关挂在解锁链第 5 位（prereq=12）。
-- 链（v0.8）：L1 牛顿环 -> L2 示波器 -> L11 二分(order 3) -> L12 牛顿(order 4)
--           -> L13 高斯(order 5)。

USE physics_lab;

-- ========== 实验 13：高斯消去法解线性方程组 ==========
-- 评分 target.method = linear_rel_err：scoring.go 用列主元高斯消去法按
-- system_key（或 user_config 自定义 A/b 方程组）现算真值解（真值不落库），
-- 与 readings[0].solution 按无穷范数相对误差打分。
-- sys3_basic 为经典教材例题，解恰为整数 (2, 3, -1)，学生手工消去即可满分；
-- 前端换题可用 user_config 换库内方程组（sys4_tridiag / hilbert4）或自定义 A/b。
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (13, 'gauss_elim', '高斯消去法解线性方程组', 'canvas_2d',
    JSON_OBJECT(
      'system_key', 'sys3_basic',
      'system_desc', '2x + y - z = 8; -3x - y + 2z = -11; -2x + y + 2z = -3',
      'A', JSON_ARRAY(JSON_ARRAY(2,1,-1), JSON_ARRAY(-3,-1,2), JSON_ARRAY(-2,1,2)),
      'b', JSON_ARRAY(8,-11,-3),
      'tolerance', 0.000001),
    JSON_OBJECT(
      'method', 'linear_rel_err',
      'system_key', 'sys3_basic',
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 13：前置 L12，解锁链第 5 位 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (13, 13, '高斯消去法解线性方程组', 5, 2, 12)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
