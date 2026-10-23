-- 016_add_romberg_integration.sql - 数值分析实验批次 L9：龙贝格积分法 Romberg（数据组）
-- 2026-09-17 新增（用户点名，不在原排队表）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：015（v0.14 后 L18 逐次线性插值为链尾 level 18）。本关挂在解锁链第 9 位（prereq=18）。
-- 链（v0.15）：L11 二分(1) -> L12 牛顿(2) -> L13 高斯(3) -> L14 雅可比(4)
--           -> L15 SOR(5) -> L16 拉格朗日插值(6) -> L17 辛普森积分(7) -> L18 逐次线性插值(8)
--           -> L19 龙贝格积分(9)。
--
-- 评分真值 = 龙贝格表终值 T_k^(k)（后端按同一 f、区间、k 现算不落库，"真值=方法输出"）。
-- 题库与 L17 共用 integProblems（sin_pi / erf_gauss / sqrt_x）：外推第 1 列恰为复合辛普森
-- S_{2^m}（单测交叉验证），sin_pi 默认 k=2 时 S_1=2π/3 正是 L17 的 S_2——
-- "梯形递推 + 辛普森 + 更高阶外推"一条链。
-- 抄精确积分 I=2 约 81 分（能过线但非满分：龙贝格收敛快）；抄 L17 答案 2π/3
-- 或抄最后一行梯形 T_2 均 ~41 分不过线；t_col（梯形列）审计暴露过程。
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (19, 'romberg_integration', '龙贝格积分法', 'canvas_2d',
    JSON_OBJECT(
      'problem_key', 'sin_pi',
      'problem_desc', 'sin x 龙贝格求积：梯形逐次减半得 T_0→T_1→T_2，再做两轮理查逊外推求终值（外推第 1 列 S_1=2π/3 正是 L17 的 S_2）',
      'func_desc', 'f(x) = sin x',
      'a', 0,
      'b', 3.141592653589793,
      'k', 2,
      'method_hint', 'T_m = T_{m−1}/2 + h_m·Σf(新节点)；逐列外推 T_i^(j) = (4^j·T_i^(j−1) − T_{i−1}^(j−1))/(4^j − 1)，终值 T_k^(k) 即提交值'),
    JSON_OBJECT(
      'method', 'romberg_rel_err',
      'problem_key', 'sin_pi',
      'k', 2,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 19：前置 L18，解锁链第 9 位 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (19, 19, '龙贝格积分法', 9, 3, 18)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
