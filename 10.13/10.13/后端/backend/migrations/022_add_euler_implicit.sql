-- 022_add_euler_implicit.sql - 数值分析实验批次 L16：隐式（后退）欧拉法（数据组）
-- 2026-10-09 新增（用户点名"微分方程数值解"再添一关：与 L20 显式欧拉同题
-- 显隐对照，绝对稳定性教学主场）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：021（v0.20 后 L25 三次样条插值为链尾 level 25）。本关接在解锁链第 16 位。
-- 链（v0.21）：L11 二分(1) -> ... -> L24 最小二乘拟合(14) -> L25 三次样条插值(15)
--   -> L26 隐式欧拉法(16)。
--
-- 评分真值 = 方法输出 y_N（后端按同一 f、初值、区间、步长现算 eulerImplicit
-- 不落库，"真值=方法输出"同 L20-L23 设计）。
-- 隐式（后退）欧拉：y_{n+1} = y_n + h·f(x_{n+1}, y_{n+1})，未知量在等式两侧；
-- 题库 odeProblems 均为线性齐次 y'=a(x)·y，闭式解 y_{n+1} = y_n/(1−h·a(x_{n+1}))。
-- 稳定性函数 R(hλ)=1/(1−hλ)：绝对稳定域含整个左半平面（A 稳定），与显式欧拉
-- 的圆盘 |1+z|≤1 互补——stiff_decay（hλ=−2.5）上 L20 显式振荡发散 (−1.5)^4=5.0625，
-- 隐式单调收敛 (2/7)^4=16/2401≈0.006664，正面对照。
-- 题库复用 odeProblems（与 L17 积分/L20-L23 ODE 系列同界 [0,1] 三题）：
--   exp_growth  y'=y     每步除以 (1−h)：n=4 真值 (4/3)^4=256/81≈3.160494，
--                        抄精确解 e ~24 分、抄 L20 显式 ~15 分、抄 L21 改进 ~21 分、
--                        抄 L22 RK4 ~23 分、抄 L23 阿当姆斯 ~24 分均不过线；
--   decay_sq    y'=-2xy  每步除以 1+2h·x_{n+1}：n=4 真值 ≈0.344781，
--                        抄精确解 e^{-1} ~36 分不过线；
--   stiff_decay y'=-10y  A 稳定收敛真值 16/2401，抄精确解 e^{-10} 与抄 L20
--                        显式发散值 5.0625 均 0 分。
-- 注意：h=1 时 1−h·f(x,1)=0（exp_growth n=1），隐式方程无有限解 -> 400。

-- ========== 实验 26：隐式欧拉法 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (26, 'euler_implicit', '隐式欧拉法', 'canvas_2d',
    JSON_OBJECT(
      'problem_key', 'exp_growth',
      'problem_desc', 'y''=y, y(0)=1 的后退欧拉递推：y_{n+1}=y_n+h·f(x_{n+1},y_{n+1})，解隐式方程求 y(1)（默认 n=4，h=0.25，每步除以 1−h 恰乘 4/3）；stiff_decay 题观察 A 稳定：显式发散处隐式单调收敛',
      'func_desc', 'f(x, y) = y（隐式方程 y_{n+1}=y_n+h·y_{n+1} 解出 y_{n+1}=y_n/(1−h)，稳定性函数 R=1/(1−hλ)，绝对稳定域含整个左半平面）',
      'a', 0,
      'b', 1,
      'y0', 1,
      'n', 4,
      'method_hint', '后退欧拉 y_{n+1} = y_n + h·f(x_{n+1}, y_{n+1})（局部截断误差 O(h)，与 L20 显式同阶而方向相反：从上方压向精确解）；y''=y 时每步除以 (1−h)；绝对稳定域为 |1−z|≥1（A 稳定），刚性题不发散'),
    JSON_OBJECT(
      'method', 'euler_implicit_rel_err',
      'problem_key', 'exp_growth',
      'n', 4,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 26：前置 L25，解锁链第 16 位（难度 3：隐式方程求解为新概念） ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (26, 26, '隐式欧拉法', 16, 3, 25)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
