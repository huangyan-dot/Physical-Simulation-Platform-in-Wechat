-- 006_new_experiments_batch3.sql - 第三批新增 3 个实验 + 3 个关卡
-- 幂等：可重复执行。Windows 下务必加 --default-character-set=utf8mb4。

USE physics_lab;

-- ========== 第三批新增 3 个实验 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (10, 'oil_drop', '密立根油滴实验', 'mixed_3d_2d',
    JSON_OBJECT('plate_dist_m',0.006,'oil_density',850,'air_viscosity',1.83e-5),
    JSON_OBJECT('method','oil_drop_fit','elementary',1.602e-19,'pass_score',60)),
  (11, 'polarization', '偏振光实验', 'mixed_3d_2d',
    JSON_OBJECT('initial_intensity',1.0),
    JSON_OBJECT('method','malus_fit','pass_score',60)),
  (12, 'sound_speed', '声速测量实验', 'mixed_3d_2d',
    JSON_OBJECT('freq_hz',37000),
    JSON_OBJECT('method','sound_speed_fit','speed_ms',343.0,'freq_hz',37000,'pass_score',60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 第三批新增 3 个关卡，续接解锁链 L9->L10->L11->L12 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (10, 10, '密立根油滴实验', 10, 4, 9),
  (11, 11, '偏振光实验', 11, 2, 10),
  (12, 12, '声速测量实验', 12, 3, 11)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
