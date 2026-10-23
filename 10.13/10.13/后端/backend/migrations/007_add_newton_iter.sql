-- 007_add_newton_iter.sql - 数值分析实验批次 L2：牛顿迭代法求根（数据组）
-- 2026-08-25 新增。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：006（L1 二分法，level 11）。本关挂在解锁链第 2 位（prereq=11）。
-- 与 L1 同函数 cubic_1，学生可对比两法收敛速度（牛顿 4 步 vs 二分 19 步）。

USE physics_lab;

-- ========== 实验 12：牛顿迭代法求根 ==========
-- 评分 target.method = newton_rel_err：scoring.go 按 readings/user_config 的 x0
-- 模拟牛顿迭代（|x_{n+1}-x_n|<tol 停止）得出真值根与期望迭代数；
-- 评分 = 根精度分 - 迭代效率罚分 4 x max(0, iterations - expected - 2)。
-- 真值根不落库；多根函数（sqrt_2 换题）按学生 x0 的实际收敛路径评分。
-- expected_iterations = 4：x0=1.5 下 |x_{n+1}-x_n|<1e-6 的牛顿步数。
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (12, 'newton_iter', '牛顿迭代法求根', 'canvas_2d',
    JSON_OBJECT(
      'func', 'x^3 - x - 1 = 0',
      'func_key', 'cubic_1',
      'interval', JSON_ARRAY(1.0, 1.5),
      'x0', 1.5,
      'tolerance', 0.000001,
      'max_iterations', 50,
      'expected_iterations', 4),
    JSON_OBJECT(
      'method', 'newton_rel_err',
      'func_key', 'cubic_1',
      'interval', JSON_ARRAY(1.0, 1.5),
      'x0', 1.5,
      'tolerance', 0.000001,
      'max_iterations', 50,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 12：前置 L11，解锁链第 2 位 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (12, 12, '牛顿迭代法求根', 2, 2, 11)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
