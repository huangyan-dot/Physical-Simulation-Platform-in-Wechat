-- 013_add_lagrange_interp.sql - 数值分析实验批次 L6：拉格朗日插值（数据组）
-- 2026-09-12 新增。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：012（v0.11 后 L15 SOR 为链尾 level 15）。本关挂在解锁链第 6 位（prereq=15）。
-- 链（v0.12）：L11 二分(order 1) -> L12 牛顿(order 2) -> L13 高斯(order 3)
--            -> L14 雅可比(order 4) -> L15 SOR(order 5) -> L16 插值(order 6)。
--
-- 评分真值 = 拉格朗日插值多项式 P(x_eval)（由节点唯一确定，后端现算不落库）。
-- 原函数 f 仅供前端画图对照（插值误差/龙格现象是观察点，不参与评分），不进本表。
-- 题库三题：sin_quad（基函数 2/9、8/9、−1/9 手工可验证）/ sqrt_table（教材开方表经典）
-- / runge_5（龙格现象，P(0.8)≈−0.379310 与 f(0.8)=0.058824 差 1.15 倍相对误差，防抄函数值）。
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (16, 'lagrange_interp', '拉格朗日插值', 'canvas_2d',
    JSON_OBJECT(
      'problem_key', 'sin_quad',
      'problem_desc', 'sin x 三点二次插值：节点 0、π/4、π/2 处的函数值构成插值表',
      'func_desc', 'f(x) = sin x',
      'nodes_x', JSON_ARRAY(0, 0.7853981633974483, 1.5707963267948966),
      'nodes_y', JSON_ARRAY(0, 0.7071067811865476, 1),
      'x_eval', 0.5235987755982988,
      'x_eval_hint', 'P(π/6)：先算三个基函数 L_i(π/6)，再加权求和'),
    JSON_OBJECT(
      'method', 'interp_rel_err',
      'problem_key', 'sin_quad',
      'x_eval', 0.5235987755982988,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 16：前置 L15，解锁链第 6 位 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (16, 16, '拉格朗日插值', 6, 2, 15)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
