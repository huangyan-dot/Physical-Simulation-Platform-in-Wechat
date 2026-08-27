-- 006_add_numerical_analysis.sql - 数值分析实验批次 L1：二分法求根（数据组）
-- 2026-08-25 新增。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。
--
-- 编号约定：物理实验保留 id 1-10（002 占用 1-3），数值分析批次从 11 起，
-- 避免将来恢复物理实验时主键冲突。order_no 从 1 起（当前库无其他关卡）；
-- 若物理实验回归，需全组重排解锁链（order_no / prereq_level_id）。

USE physics_lab;

-- ========== 实验 11：二分法求根 ==========
-- 评分 target.method = root_rel_err：scoring.go 按 func_key + interval 现算真值根
-- （真值不落库），与 readings[0].root 求相对误差打分，曲线见契约附录。
-- user_config 可覆盖 func_key / interval（前端换题或换区间时后端重算真值）。
-- expected_iterations = ceil(log2(0.5/1e-6)) = 19，供前端/教学参考。
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (11, 'bisection', '二分法求根', 'canvas_2d',
    JSON_OBJECT(
      'func', 'x^3 - x - 1 = 0',
      'func_key', 'cubic_1',
      'interval', JSON_ARRAY(1.0, 1.5),
      'tolerance', 0.000001,
      'max_iterations', 50,
      'expected_iterations', 19),
    JSON_OBJECT(
      'method', 'root_rel_err',
      'func_key', 'cubic_1',
      'interval', JSON_ARRAY(1.0, 1.5),
      'pass_score', 60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 关卡 11：数值分析解锁链首关（无前置）==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (11, 11, '二分法求根', 1, 1, NULL)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
