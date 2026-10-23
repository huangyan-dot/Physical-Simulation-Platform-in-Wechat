-- 008_restore_physics_experiments.sql - 恢复 PDF 交付实验：牛顿环 + 示波器（数据组）
-- 2026-08-25。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 依据《开发任务讲解》交付物 #4"至少 2 个完整可玩实验（牛顿环、示波器）"。
-- config/target 与前端页面实际提交逐一对齐（newton-ring.js / oscilloscope.js），
-- id 沿用 002 旧编号（1/2）——前端 newton-ring.js 默认 level_id=1、oscilloscope.js 默认 level_id=2。
--
-- 打分曲线 target.bands（v0.7 新机制，scoring.go parseRelErrBands 解析）：
-- 物理实验按读数容差量级标定（容差 0.02mm ≈ 0.5%~2% 相对误差），与数值分析收敛精度不同：
--   <=0.5% 100 / 2% 90 / 5% 80 / 10% 65 / 30% 30 / >=60% 0（拐点间对数插值）
--
-- 解锁链重排（物理优先，PDF 阶段2 牛顿环 -> 阶段3 示波器 -> 数值分析批次）：
--   L1 牛顿环(order 1, 无前置) -> L2 示波器(order 2, prereq 1)
--   -> L11 二分法(order 3, prereq 2) -> L12 牛顿迭代(order 4, prereq 11)

USE physics_lab;

-- ========== 实验 1：牛顿环（least_squares_R：r²=kλR 过原点最小二乘拟合 R）==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (1, 'newton_ring', '牛顿环实验', 'mixed_3d_2d',
    JSON_OBJECT('wavelength_nm', 589.3, 'lens_radius_mm', 855, 'k_range', JSON_ARRAY(1,10), 'tolerance_mm', 0.02),
    JSON_OBJECT('method','least_squares_R','lens_radius_mm',855,'wavelength_nm',589.3,'pass_score',60,
      'bands', CAST('[[0.005,100],[0.02,90],[0.05,80],[0.1,65],[0.3,30],[0.6,0]]' AS JSON)))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 实验 2：示波器（param_match：CH1/CH2 的 A/f 四参数平均）==========
-- config.channels 键大写 CH1/CH2（与前端提交一致）；phi 参与波形渲染但不评分。
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (2, 'oscilloscope', '示波器实验', 'mixed_3d_2d',
    JSON_OBJECT('channels', JSON_OBJECT(
      'CH1', JSON_OBJECT('A',2.0,'f',50,'phi',0),
      'CH2', JSON_OBJECT('A',1.5,'f',50,'phi',0.7854))),
    JSON_OBJECT('method','param_match','channels', JSON_OBJECT(
      'CH1', JSON_OBJECT('A',2.0,'f',50),
      'CH2', JSON_OBJECT('A',1.5,'f',50)),'pass_score',60,
      'bands', CAST('[[0.005,100],[0.02,90],[0.05,80],[0.1,65],[0.3,30],[0.6,0]]' AS JSON)))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 1/2：物理实验居链首 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (1, 1, '牛顿环实验', 1, 2, NULL),
  (2, 2, '示波器实验', 2, 3, 1)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);

-- ========== 数值分析关卡顺延至链尾（原 order 1/2 -> 3/4）==========
UPDATE levels SET order_no = 3, prereq_level_id = 2  WHERE id = 11;
UPDATE levels SET order_no = 4, prereq_level_id = 11 WHERE id = 12;
