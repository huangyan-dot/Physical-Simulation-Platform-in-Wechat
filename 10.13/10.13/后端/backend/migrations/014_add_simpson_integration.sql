-- 014_add_simpson_integration.sql - 数值分析实验批次 L7：复合辛普森积分法（数据组）
-- 2026-09-16 新增。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 前置：013（v0.12 后 L16 拉格朗日插值为链尾 level 16）。本关挂在解锁链第 7 位（prereq=16）。
-- 链（v0.13）：L11 二分(order 1) -> L12 牛顿(order 2) -> L13 高斯(order 3)
--           -> L14 雅可比(order 4) -> L15 SOR(order 5) -> L16 插值(order 6)
--           -> L17 辛普森积分(order 7)。
--
-- 评分真值 = 复合辛普森值 S_n（后端按同一 f、区间、n 现算不落库，同 L16
-- "真值=方法输出"的设计：不是精确积分 I）。精确积分 I 仅作审计参考记入 detail。
-- 直接抄精确积分值被截断误差压住：sin_pi 默认 n=2，抄 I=2 相对误差 ~4.5% -> ~42 分不过线。
-- 题库三题：sin_pi（n=2 单次辛普森闭式 2π/3 手工可验证，I=2 已知）
-- / erf_gauss（e^(−x²) 无初等闭式必须数值积分，n=4）
-- / sqrt_x（端点导数奇异，辛普森收敛慢的方法适用性观察，n=8）。
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (17, 'simpson_integration', '复合辛普森积分法', 'canvas_2d',
    JSON_OBJECT(
      'problem_key', 'sin_pi',
      'problem_desc', '辛普森经典验证：∫₀^π sin x dx = 2（精确值已知），先用 n=2 单次辛普森算 S₂ = (π/6)·(f(0) + 4f(π/2) + f(π))',
      'func_desc', 'f(x) = sin x',
      'a', 0,
      'b', 3.141592653589793,
      'n', 2,
      'n_hint', 'n 为偶数子区间数（2~1024），加大 n 观察 S_n 逼近精确积分 2'),
    JSON_OBJECT(
      'method', 'integ_rel_err',
      'problem_key', 'sin_pi',
      'a', 0,
      'b', 3.141592653589793,
      'n', 2,
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 17：前置 L16，解锁链第 7 位 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (17, 17, '复合辛普森积分法', 7, 3, 16)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
