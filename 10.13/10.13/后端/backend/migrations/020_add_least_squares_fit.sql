-- 020_add_least_squares_fit.sql - 数值分析实验批次 L14：最小二乘拟合（数据组）
-- 2026-10-01 新增（排队表末位按序上线，数值分析链收官）。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：019（v0.18 后 L23 阿当姆斯预报-校正为链尾 level 23）。本关接在解锁链第 14 位。
-- 链（v0.19）：L11 二分(1) -> ... -> L22 龙格-库塔(12) -> L23 阿当姆斯预报-校正(13) -> L24 最小二乘拟合(14)。
--
-- 评分真值 = 法方程拟合多项式 P(x_eval)（后端按同一数据点、同一 m 现算 lsqFit
-- 不落库，"真值=方法输出"同 L7/L9/L16/L20-L23 设计）。
-- 幂基法方程 sum_k x_k^{i+j} * c_i = sum_k x_k^j * y_k（j=0..m），列主元高斯求解；
-- m 须 0~min(5, 点数-2)——上界防拟合退化成插值（m=点数-1 时法方程解=插值多项式，
-- 防抄 L16/L18 插值答案的语义随之失效），5 封顶防法方程病态。
-- 题库 fitProblems（前后端逐字同步）：
--   lin_line 直线拟合 4 点 (1,2),(2,3.1),(3,4),(4,4.9)，法方程手算 a=1.1、b=0.96，
--     x_eval=3.5 真值恰 4.46；
--   quad_parabola 二次拟合 5 点，系数恰 27/350、1/28、137/140，P(2)=711/175；
--   exp_lnw 指数模型 y=a*e^{bx} ln 线性化拟合 5 点，真值=e^{A+2.5B}=6.9103931。
-- 防作弊语义（默认曲线 pass_score=60）：拟合不过点，抄观测值（29/30/44 分）、
-- 抄均值（16 分）、指数题不线性化直接拟 y（12 分）均不过线；quad 题 x_eval 落在
-- 数据点上，抄真函数 x^2=4 恰 58 分差线 2 分；抄三次插值 P3(3.5)=4.4375 约 69 分
-- 过线但非满分（插值≠拟合，与 RK4 抄 e 同款"同为合法路径但不该得满分"语义，
-- 过程真伪由 coef 审计暴露）。

-- ========== 实验 24：最小二乘拟合 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (24, 'least_squares_fit', '最小二乘拟合', 'canvas_2d',
    JSON_OBJECT(
      'problem_key', 'lin_line',
      'problem_desc', '散数据直线拟合 y=a+bx：求和表列 x_k、y_k、x_k*y_k、x_k^2 组装法方程（4a+10b=14、10a+30b=39.8 手算可得 a=1.1、b=0.96），解出系数后给出拟合值 P(3.5)；可换题二次拟合/指数模型 ln 线性化',
      'func_desc', '最小二乘：使残差平方和 sum (y_k - P(x_k))^2 最小的 m 次多项式，法方程为 sum_k x_k^{i+j} c_i = sum_k x_k^j y_k（j=0..m）；指数模型 y=a*e^{bx} 先取 ln y 线性化再拟合 A+Bx',
      'm', 1,
      'x_eval', 3.5,
      'method_hint', '法方程（幂基 m=1）：n*c0 + (sum x)*c1 = sum y；(sum x)*c0 + (sum x^2)*c1 = sum x*y，高斯消去解 c0..c_m 后求 P(x_eval)；m 须 0~min(5, 点数-2)'
    ),
    JSON_OBJECT(
      'method', 'lsq_rel_err',
      'problem_key', 'lin_line',
      'm', 1,
      'x_eval', 3.5,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 24：前置 L23，解锁链第 14 位（难度 3） ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (24, 24, '最小二乘拟合', 14, 3, 23)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
