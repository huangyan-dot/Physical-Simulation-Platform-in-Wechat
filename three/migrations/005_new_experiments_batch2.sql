-- 005_new_experiments_batch2.sql - 第二批新增 3 个实验 + 3 个关卡
-- 幂等：可重复执行。Windows 下务必加 --default-character-set=utf8mb4。

USE physics_lab;

-- ========== 第二批新增 3 个实验 ==========
INSERT INTO experiments (id, code, name, render_mode, config, target) VALUES
  (7, 'photoelectric', '光电效应实验', 'mixed_3d_2d',
    JSON_OBJECT('wavelengths_nm', JSON_ARRAY(405,436,546,577)),
    JSON_OBJECT('method','planck_fit','planck_const',6.626e-34,'work_function',3.0e-19,'pass_score',60)),
  (8, 'frank_hertz', '弗兰克-赫兹实验', 'mixed_3d_2d',
    JSON_OBJECT('vg2_min',0,'vg2_max',30,'step',0.2),
    JSON_OBJECT('method','excitation_fit','excitation_pot_v',4.9,'pass_score',60)),
  (9, 'diffraction_grating', '光栅衍射实验', 'mixed_3d_2d',
    JSON_OBJECT('grating_const_m',3.333e-6,'wavelength_nm',589.3),
    JSON_OBJECT('method','grating_fit','grating_const_m',3.333e-6,'pass_score',60))
ON DUPLICATE KEY UPDATE
  name=VALUES(name), render_mode=VALUES(render_mode), config=VALUES(config), target=VALUES(target);

-- ========== 第二批新增 3 个关卡，续接解锁链 L6->L7->L8->L9 ==========
INSERT INTO levels (id, experiment_id, name, order_no, difficulty, prereq_level_id) VALUES
  (7, 7, '光电效应实验', 7, 3, 6),
  (8, 8, '弗兰克-赫兹实验', 8, 5, 7),
  (9, 9, '光栅衍射实验', 9, 3, 8)
ON DUPLICATE KEY UPDATE
  experiment_id=VALUES(experiment_id), name=VALUES(name), order_no=VALUES(order_no),
  difficulty=VALUES(difficulty), prereq_level_id=VALUES(prereq_level_id);
