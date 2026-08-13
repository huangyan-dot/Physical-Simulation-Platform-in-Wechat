-- 004_new_experiments.sql - 新增 3 个实验 + 3 个关卡（数据组规范脚本）
-- 用 root 执行一次即可。幂等：可重复执行。
-- Windows 下 mysql 客户端务必加 --default-character-set=utf8mb4，否则中文报错 1366。

USE physics_lab;

-- ========== 新增 3 个实验 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (4, 'young_modulus', '杨氏模量实验', 'mixed_3d_2d',
    JSON_OBJECT('force_n',4.905,'length_m',1.0,'diameter_m',0.0005,'lever_arm_m',0.070,'mirror_dist_m',1.5),
    JSON_OBJECT('method','young_fit','young_modulus_pa',2.0e11,'force_n',4.905,'length_m',1.0,'diameter_m',0.0005,'lever_arm_m',0.070,'mirror_dist_m',1.5,'pass_score',60)),
  (5, 'hall_effect', '霍尔效应实验', 'mixed_3d_2d',
    JSON_OBJECT('b_field_t',0.3,'thickness_m',0.0005),
    JSON_OBJECT('method','hall_fit','b_field_t',0.3,'thickness_m',0.0005,'carrier_conc',1.0e21,'pass_score',60)),
  (6, 'michelson', '迈克尔逊干涉仪实验', 'mixed_3d_2d',
    JSON_OBJECT('wavelength_nm',632.8),
    JSON_OBJECT('method','wavelength_fit','wavelength_nm',632.8,'pass_score',60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 新增 3 个关卡，续接解锁链 L3->L4->L5->L6 ==========
-- L4 前置 L3，L5 前置 L4，L6 前置 L5
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (4, 4, '杨氏模量实验', 4, 3, 3),
  (5, 5, '霍尔效应实验', 5, 4, 4),
  (6, 6, '迈克尔逊干涉仪实验', 6, 4, 5)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
