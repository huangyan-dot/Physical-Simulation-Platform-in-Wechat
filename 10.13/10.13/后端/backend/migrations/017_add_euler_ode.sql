-- 017_add_euler_ode.sql - 数值分析实验批次 L10/L11：欧拉法（常微分方程初值问题）（数据组）
-- 2026-09-21 新增（用户点名，双关一次交付：L20 显式欧拉 + L21 改进欧拉）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：016（v0.15 后 L19 龙贝格积分为链尾 level 19）。本批两关接在解锁链第 10/11 位。
-- 链（v0.16）：L11 二分(1) -> ... -> L18 逐次线性插值(8) -> L19 龙贝格积分(9)
--           -> L20 显式欧拉(10) -> L21 改进欧拉(11)。
--
-- 评分真值 = 方法输出 y_N（后端按同一 f、初值、区间、步长现算不落库，"真值=方法输出"）。
-- 题库 odeProblems（与 L17/L19 的积分题库同界 [0,1]，三题均默认 n=4 h=0.25）：
--   exp_growth  y'=y      抄精确解 e 差 ~11%（显式）/ ~2.9%（改进 n=2）被截断误差拦住；
--   decay_sq    y'=-2xy   f 显含 x,y，显式 n=4 全列手算闭式（y4=0.41015625）；
--   stiff_decay y'=-10y   刚性观察题：hλ=-2.5 在绝对稳定区间外振荡发散，
--                          真值仍确定（显式 (-1.5)^4=5.0625、改进 (13/8)^4），
--                          抄精确解 e^{-10} 直接 0 分。
-- L21 改进欧拉（预测-校正）防抄：抄精确解 ~49 分、抄 L20 显式答案 ~34 分均不过线。

-- ========== 实验 20：显式欧拉法 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (20, 'euler_explicit', '显式欧拉法', 'canvas_2d',
    JSON_OBJECT(
      'problem_key', 'exp_growth',
      'problem_desc', 'y''=y, y(0)=1 的欧拉折线递推：y_{n+1}=y_n+h·f(x_n,y_n)，求 y(1)（默认 n=4，h=0.25）',
      'func_desc', 'f(x, y) = y（右端函数与精确解 e^x 对照，见斜率场）',
      'a', 0,
      'b', 1,
      'y0', 1,
      'n', 4,
      'method_hint', '折线递推 y_{n+1} = y_n + h·f(x_n, y_n)（前进欧拉，局部截断误差 O(h)）；stiff_decay 题观察 hλ 超出绝对稳定区间 |1+hλ|≤1 时的振荡发散'),
    JSON_OBJECT(
      'method', 'euler_rel_err',
      'problem_key', 'exp_growth',
      'n', 4,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 实验 21：改进欧拉法（预测-校正） ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (21, 'euler_improved', '改进欧拉法', 'canvas_2d',
    JSON_OBJECT(
      'problem_key', 'exp_growth',
      'problem_desc', 'y''=y, y(0)=1 的预测-校正递推：先显式欧拉探一步 y_p，再取梯形平均校正，求 y(1)（默认 n=2，h=0.5）',
      'func_desc', 'f(x, y) = y（预测-校正即单步梯形公式，局部截断误差 O(h²)）',
      'a', 0,
      'b', 1,
      'y0', 1,
      'n', 2,
      'method_hint', '预测 y_p = y_n + h·f(x_n,y_n)；校正 y_{n+1} = y_n + h/2·[f(x_n,y_n) + f(x_{n+1},y_p)]；y''=y 时每步恰乘 1+h+h²/2'),
    JSON_OBJECT(
      'method', 'euler_improved_rel_err',
      'problem_key', 'exp_growth',
      'n', 2,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 20/21：前置 L19/L20，解锁链第 10/11 位 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (20, 20, '显式欧拉法', 10, 2, 19),
  (21, 21, '改进欧拉法', 11, 3, 20)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
