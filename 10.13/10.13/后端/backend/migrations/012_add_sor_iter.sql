-- 012_add_sor_iter.sql - 数值分析实验批次 L5：SOR 逐次超松弛迭代法解线性方程组（数据组）
-- 2026-09-10 新增（插队：插值/积分等批次仍在排队）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：011（v0.10 后 L14 雅可比为链尾 level 14）。本关挂在解锁链第 5 位（prereq=14）。
-- 链（v0.11）：L11 二分(order 1) -> L12 牛顿(order 2) -> L13 高斯(order 3) -> L14 雅可比(order 4) -> L15 SOR(order 5)。
--
-- 与 L14 雅可比的对照：SOR = 高斯-赛德尔 + 松弛因子 ω；sys6_poisson 是 SOR 主场
-- （雅可比 114 步超 100 上限不收敛、ω=1 高斯-赛德尔 61 步、最优 ω≈1.4 仅 19 步），
-- 学生调 ω 寻优即本关教学目标。
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (15, 'sor_iter', 'SOR 逐次超松弛迭代法解线性方程组', 'canvas_2d',
    JSON_OBJECT(
      'system_key', 'sys6_poisson',
      'system_desc', '2x1 - x2 = 1; -x1 + 2x2 - x3 = 0; -x2 + 2x3 - x4 = 0; -x3 + 2x4 - x5 = 0; -x4 + 2x5 - x6 = 0; -x5 + 2x6 = 1',
      'A', JSON_ARRAY(
        JSON_ARRAY(2, -1, 0, 0, 0, 0),
        JSON_ARRAY(-1, 2, -1, 0, 0, 0),
        JSON_ARRAY(0, -1, 2, -1, 0, 0),
        JSON_ARRAY(0, 0, -1, 2, -1, 0),
        JSON_ARRAY(0, 0, 0, -1, 2, -1),
        JSON_ARRAY(0, 0, 0, 0, -1, 2)),
      'b', JSON_ARRAY(1, 0, 0, 0, 0, 1),
      'x0', JSON_ARRAY(0, 0, 0, 0, 0, 0),
      'omega', 1.0,
      'omega_min', 0.5,
      'omega_max', 1.95,
      'omega_hint', 'omega=1 即高斯-赛德尔；试试 1.3~1.5，观察 61 步被加速到约 19 步',
      'tolerance', 0.000001,
      'max_iterations', 100,
      'expected_iterations', 61),
    JSON_OBJECT(
      'method', 'sor_rel_err',
      'system_key', 'sys6_poisson',
      'x0', JSON_ARRAY(0, 0, 0, 0, 0, 0),
      'omega', 1.0,
      'tolerance', 0.000001,
      'max_iterations', 100,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 15：前置 L14，解锁链第 5 位 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (15, 15, 'SOR 逐次超松弛迭代法解线性方程组', 5, 2, 14)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
