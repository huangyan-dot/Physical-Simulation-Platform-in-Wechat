-- 018_add_rk4_ode.sql - 数值分析实验批次 L12：四阶龙格-库塔法（数据组）
-- 2026-09-22 新增（原排队表末位，按 L13-L17 同款四件套节奏交付）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：017（v0.16 后 L21 改进欧拉为链尾 level 21）。本关接在解锁链第 12 位。
-- 链（v0.17）：L11 二分(1) -> ... -> L20 显式欧拉(10) -> L21 改进欧拉(11) -> L22 龙格-库塔(12)。
--
-- 评分真值 = 方法输出 y_N（后端按同一 f、初值、区间、步长现算 rk4Column 不落库，
-- "真值=方法输出"同 L7/L9/L20/L21 设计；精确解闭式记审计 ref_Y 不打分）。
-- 题库复用 L20/L21 的 odeProblems（exp_growth / decay_sq / stiff_decay，均 [0,1]），
-- 与 L20/L21 构成"同题库、精度阶对照"（O(h) / O(h²) / O(h⁴)）。
-- target 默认 exp_growth n=1（单步四阶：K1..K4 = 1、1.5、1.75、2.75 全手算，
-- 终值恰 65/24 ≈ 2.7083333）。
-- 防作弊语义：抄精确解 e 约 72 分（RK4 收敛快，能过线但非满分——过程真伪由
-- y_col 审计暴露，L19 龙贝格同款语义）；抄 L21 改进欧拉答案 2.640625 约 51 分、
-- 抄 L20 显式答案 2.44140625 约 30 分均不过线；stiff_decay 单步即发散
-- （稳定性函数 |R(-10)|=291>1），抄精确解 e^{-10} 与抄 L20/L21 发散值均直接 0 分。

-- ========== 实验 22：四阶龙格-库塔法 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (22, 'runge_kutta_ode', '龙格-库塔法', 'canvas_2d',
    JSON_OBJECT(
      'problem_key', 'exp_growth',
      'problem_desc', 'y''=y, y(0)=1 的经典四阶 Runge-Kutta 单步递推：K1..K4 四次斜率探测加权平均，求 y(1)（默认 n=1，h=1，单步）',
      'func_desc', 'f(x, y) = y（RK4 全局截断误差 O(h⁴)，与 L20/L21 构成精度阶对照 O(h)/O(h²)/O(h⁴)）',
      'a', 0,
      'b', 1,
      'y0', 1,
      'n', 1,
      'method_hint', 'K1=f(x_n,y_n)；K2=f(x_n+h/2, y_n+h/2·K1)；K3=f(x_n+h/2, y_n+h/2·K2)；K4=f(x_n+h, y_n+h·K3)；y_{n+1}=y_n+h/6·(K1+2K2+2K3+K4)'
    ),
    JSON_OBJECT(
      'method', 'rk4_rel_err',
      'problem_key', 'exp_growth',
      'n', 1,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 22：前置 L21，解锁链第 12 位（难度 4，链上最高） ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (22, 22, '龙格-库塔法', 12, 4, 21)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
