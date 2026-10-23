-- 015_add_aitken_interp.sql - 数值分析实验批次 L8：逐次线性插值法 Aitken/Neville（数据组）
-- 2026-09-17 新增（用户点名插队，不在原排队表）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：014（v0.13 后 L17 复合辛普森为链尾 level 17）。本关挂在解锁链第 8 位（prereq=17）。
-- 链（v0.14）：L11 二分(1) -> L12 牛顿(2) -> L13 高斯(3) -> L14 雅可比(4)
--           -> L15 SOR(5) -> L16 拉格朗日插值(6) -> L17 辛普森积分(7) -> L18 逐次线性插值(8)。
--
-- 评分真值 = Neville 递推终值 P_{n-1,n-1}（与 L16 拉格朗日公式评同一插值多项式，
-- 两路结果数学上相等，单测交叉验证；后端现算不落库）。题库与 L6 共用 interpProblems
-- （sin_quad / sqrt_table / runge_5），龙格题抄原函数值同样 0 分。
-- 与 L16 的区别在过程：P_{i,k} = ((x−x_{i−k})·P_{i,k−1} − (x−x_i)·P_{i−1,k−1})/(x_i−x_{i−k})，
-- 每一步都是相邻低阶结果的线性插值，逐列推进到终值。
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (18, 'aitken_interp', '逐次线性插值法', 'canvas_2d',
    JSON_OBJECT(
      'problem_key', 'sin_quad',
      'problem_desc', 'sin x 三点二次插值：与 L16 同一张节点表，改用逐次线性插值（Aitken）递推求 P(π/6)',
      'func_desc', 'f(x) = sin x',
      'nodes_x', JSON_ARRAY(0, 0.7853981633974483, 1.5707963267948966),
      'nodes_y', JSON_ARRAY(0, 0.7071067811865476, 1),
      'x_eval', 0.5235987755982988,
      'method_hint', '先算第 1 列：相邻节点的线性插值 P_{i,1}；再由第 1 列线性插值得 P_{2,2}'),
    JSON_OBJECT(
      'method', 'aitken_rel_err',
      'problem_key', 'sin_quad',
      'x_eval', 0.5235987755982988,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 18：前置 L17，解锁链第 8 位 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (18, 18, '逐次线性插值法', 8, 2, 17)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
