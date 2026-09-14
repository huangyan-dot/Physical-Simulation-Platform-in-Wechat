-- 010_add_jacobi_iter.sql - 数值分析实验批次 L4：雅可比迭代法解线性方程组（数据组）
-- 2026-08-27 新增（插队：插值/积分等批次仍在排队）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：009（L3 高斯消去，level 13）。本关挂在解锁链第 6 位（prereq=13）。
-- 链（v0.9）：L1 牛顿环 -> L2 示波器 -> L11 二分(order 3) -> L12 牛顿(order 4)
--           -> L13 高斯(order 5) -> L14 雅可比(order 6)。
--
-- 命名说明：雅可比方法是迭代法（x^{(k+1)} = D^{-1}(b-(L+U)x^{(k)})），不是消去法；
-- 实验名用规范术语"雅可比迭代法"，与 L13 高斯消去构成"直接法 vs 迭代法"对照。

USE physics_lab;

-- ========== 实验 14：雅可比迭代法解线性方程组 ==========
-- 评分 target.method = jacobi_rel_err：
--   真值解 = gaussSolve 列主元高斯精确解（不落库）；
--   期望迭代数 = 后端从学生 x0 模拟雅可比（max_i|Δx| < tol 停止）。
-- 评分 = 解精度分 - 迭代效率罚分 4 x max(0, expected/2 - 2 - iterations)
-- （防直接消去法求精确解后冒充少量雅可比迭代；高斯-赛德尔约快一倍属合法不罚；
--  模拟不收敛（非对角占优）则无期望、不罚分）。
-- sys3_diagdom 严格对角占优（雅可比必收敛），解恰为整数 (1, 2, 3)；
-- expected_iterations = 14：x0=[0,0,0] 下 max_i|Δx|<1e-6 的雅可比步数。
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (14, 'jacobi_iter', '雅可比迭代法解线性方程组', 'canvas_2d',
    JSON_OBJECT(
      'system_key', 'sys3_diagdom',
      'system_desc', '10x + 2y + z = 17; x + 10y + 2z = 27; 2x + y + 10z = 34',
      'A', JSON_ARRAY(JSON_ARRAY(10,2,1), JSON_ARRAY(1,10,2), JSON_ARRAY(2,1,10)),
      'b', JSON_ARRAY(17,27,34),
      'x0', JSON_ARRAY(0,0,0),
      'tolerance', 0.000001,
      'max_iterations', 100,
      'expected_iterations', 14),
    JSON_OBJECT(
      'method', 'jacobi_rel_err',
      'system_key', 'sys3_diagdom',
      'x0', JSON_ARRAY(0,0,0),
      'tolerance', 0.000001,
      'max_iterations', 100,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 14：前置 L13，解锁链第 6 位 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (14, 14, '雅可比迭代法解线性方程组', 6, 2, 13)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
